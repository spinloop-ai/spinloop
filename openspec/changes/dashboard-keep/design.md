# Design: dashboard keep

## Context

See proposal.md for the motivation (issues #158 and #157). The state of the
code the change lands in:

- The dashboard (`cmd/spinloop`, roughly 1,500 lines of model and render) is
  Bubble Tea with every widget hand-rolled — the spinner, the key-help footer,
  the grid scrolling — and no Bubbles component. Start and stop run through a
  per-node action value (`dashAction`): the call runs in its own goroutine,
  reports progress onto the node's tile, and on completion the footer shows
  the one-shot outcome and the node is re-read immediately.
- Keep already works end to end for the one-shot CLI: `remote.Keep` sends
  `set-keep` to the update Lambda, which sets the `Retain-Until` EC2 tag, and
  the idle sweep honours it. The fleet's `remoteNode` wraps the same
  `remote.Config` and already answers status, metrics, start, stop and logs
  through it.
- The dashboard refreshes remote environments through the stats Lambda
  (the metrics call), whose reply does not today carry the deadline. The
  Lambda already parses the tag into its instance info; it does not send it.
- Nodes advertise partial capabilities through optional interfaces that the
  dashboard type-asserts (a start that reports progress), rather than through
  methods every node must answer.

## Goals / Non-Goals

**Goals:**

- Keep settable from the grid and the detail screen for remote environments,
  with a duration the operator types.
- The deadline visible on the tile and the detail screen, from the normal
  refresh, worded the same as every one-shot surface that reads the same
  stats reply.
- No new dependencies; the dashboard's hand-rolled, terminal-free test style
  unchanged.
- A control plane that predates the reply field reads back cleanly and shows
  no line.

**Non-Goals:**

- Clearing or shortening a keep from the dashboard: setting a new deadline
  overwrites, and the sweep takes over once a deadline passes.
- Keep for local daemon nodes: there is no idle sweep on a daemon, so no
  deadline to set.
- Changing how `remote status` reports the tag: it keeps reporting the tag's
  value as it does today, including a passed one.
- Any interactive account of the sweep itself (when it next runs, why it
  skipped an instance).

## Decisions

### 1. A hand-rolled duration field, no Bubbles

The prompt is one state flag and one buffer string on the dashboard model,
driven from the model's existing key switch: printable runes append, backspace
deletes, enter confirms, escape cancels. A duration is a handful of characters
from a fixed set (digits and `h`, `m`, `s`), typed left to right, so a cursor
model, selection, and paste handling would be machinery for input that never
arrives. This matches how the dashboard already owns its spinner, key help and
scrolling, and the state tests without a terminal exactly like the rest of the
model.

Alternative considered: `bubbles/textinput`. It is the first Bubbles use in a
deliberately hand-rolled TUI, it is a general text editor where a duration
needs four keys, and styling it into the one-accent-chrome footer would cost
most of writing the field.

### 2. A capability interface, not a Node method

Keep rides a new optional capability interface in the style of the existing
progress-reporting start, implemented only by the remote node. The dashboard
type-asserts for it, as it already does for the progress start, and the key
help and the key handler gate on the same assertion.

Alternatives considered: adding `Keep` to the node interface would force the
local daemon node to answer a method it can never honestly answer; gating on
the fleet file's node kind in the dashboard would move a node-kind decision
into the screen layer and around the node abstraction.

### 3. Keep takes a duration and returns the deadline

The capability method takes a duration and returns the deadline the control
plane set. Computing now-plus-duration inside the node keeps "now" in one
place for every keeper, and the reply's value — not the caller's arithmetic —
is what the footer line and the panel show.

### 4. The deadline rides the stats reply, only while active

The stats Lambda includes the parsed tag in its reply only while it is a time
in the future, and the client maps it across the existing path: the stats
reply type, the node's stats mapping, and the shared stats shape, as an
omitted-when-absent field. The "is it still active" judgement sits in the
control plane, once, instead of in every client that reads the reply; a passed
tag is no active retention, so nothing downstream can draw a deadline the
sweep has already moved past.

Alternatives considered: the dashboard issuing a status read alongside its
metrics read for remote environments doubles the signed control-plane calls
per environment per minute for one line; filtering passed deadlines on the
client side leaves a stale deadline on the wire for every other reader and
puts a clock comparison in each of them.

The field is additive: an environment whose stats Lambda has not been
redeployed simply omits it, and the dashboard shows no line there.

### 5. One shared line beside the last-active figure

The deadline renders as a line in the shared bar-format body, next to the
last-active line that body already prints: the dashboard tile, the one-shot
fleet metrics, and the one-shot remote metrics all call that body, so they
cannot word the same read differently. The table format prints the line when
the read carries it, and the json format carries the field on its own. The
line is omitted wherever the read has no deadline, the same rule the
last-active figure follows for a figure it lacks.

### 6. No second confirmation; not abortable

The prompt is the confirmation: the operator sees the duration it will set,
and choosing to send it is the commitment. A stop asks because it ends
something; a keep overwrites a deadline and ends nothing. The abort key exists
to end a wait that has no deadline of its own — a cold cloud wake; a keep is
one fast signed call, so abort drives nothing on it, as it does on a stop.

### 7. The prompt state mirrors the stop confirmation

The prompt is handled in the model's update before the grid/detail dispatch,
the same position the stop confirmation occupies, so it works from both
screens and navigation, selection and refreshes all stand still while it is
open. Confirming an entry that does not parse as a positive duration leaves
the prompt open and shows the parse reason in the footer's hint slot — the
operator keeps their entry and corrects it in place. Escape cancels; quit and
interrupt cancel the prompt and leave the dashboard, exactly as the stop
confirmation does.

The prompt opens pre-filled with `4h`, the duration the one-shot command's
help already advertises as its example. The common overnight keep is then the
key and a confirm, and a mistaken confirm still does what the footer showed.

### 8. The key help names keep only where it does something

The footer's key help drops the keep entry for a node that does not support
retention or has an action in flight — the same mechanism that already drops
the abort entry where nothing is abortable.

### 9. The action reuses the existing in-flight machinery

Keep runs through the per-node action value with its own verb: the spinner and
elapsed time on the tile, one action per node, the node re-read immediately on
completion — which is what brings the new deadline onto the panel at the
node's next round rather than waiting out its full cadence. The action's
completion message carries the deadline so the footer line can report the
value the control plane set; start and stop leave it empty.

## Risks / Trade-offs

- [A pre-filled `4h` means a stray confirm sets four hours of retention] →
  the buffer is visible in the footer before the confirm; a keep overwrites
  and the sweep resumes when the deadline passes, so a mistaken one is
  correctable and bounded; the default is the CLI's own example value, not a
  larger one.
- [The Lambda change reaches environments only as each deployment updates] →
  the field is additive and the keep action itself reports its deadline on
  the status line from its own reply, so the feature is complete before the
  panel line appears; the line then lands per environment as deployments
  update.
- [A stopped instance keeps its tag across the stop] → intended: the sweep
  must not terminate a retained stopped instance, and the line on a stopped
  tile is exactly the protection the operator set.
- [The hand-rolled field has no cursor motion] → a duration is typed left to
  right and is at most a handful of characters; backspace and retype is
  cheaper than the cursor keys an editor would bring.

## Migration Plan

- The Go side (client, capability, dashboard, shared line) and the stats
  Lambda change ship together; nothing on the Go side requires the Lambda
  change to function — it only makes the panel line appear.
- The Lambda change is a routine deployment of the remote project; no data
  migration, no client gating: older clients ignore the extra field and newer
  clients omit the line where the field is absent.
- Rolling back the CLI/dashboard removes the key; the extra reply field is
  inert to older clients.
