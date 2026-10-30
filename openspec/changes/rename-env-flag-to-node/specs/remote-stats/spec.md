# Spec Delta

## MODIFIED Requirements

### Requirement: An environment's metrics are reported

The system SHALL provide a `metrics` command (`spinloop metrics --node <name>`) that reports the current state of a cloud inference instance. It SHALL select which environment it reports on using the same `--node <name>` flag every other read verb uses. A Spinloop given as an argument is read only for its `ENV` instructions and adjacent `.env`, never to select the environment. When the instance is running, the report SHALL include the spinloop version from the daemon, carried by the stats Lambda reply.

#### Scenario: Stats with a running instance

- **WHEN** the user runs `spinloop metrics --node <name>` with a running instance
- **THEN** the command reports the instance state, runner, model, spinloop version, GPU info, CPU/RAM usage, token counts, and request counts

#### Scenario: Stats with a stopped instance

- **WHEN** the user runs `spinloop metrics --node <name>` and the instance is stopped
- **THEN** the command reports `state: stopped` and no metrics

#### Scenario: Stats names the environment with the flag

- **WHEN** the user runs `spinloop metrics --node dev-2`
- **THEN** the command reports on the `dev-2` environment's instance

#### Scenario: Version is shown in stats output

- **WHEN** the user runs `spinloop metrics --node <name>` with a running instance
- **THEN** the output includes the spinloop version

#### Scenario: The cloud spelling names its replacement

- **WHEN** the operator runs `spinloop cloud metrics`
- **THEN** it fails naming `spinloop metrics --node <name>` as the command that
  replaced it

### Requirement: An environment's cost is reported on request

When the user passes `--cost`, the stats report SHALL include an estimated on-demand cost for the current running session. The cost SHALL be computed from the instance type's on-demand price (fetched from the AWS Price List API for the deployed region) multiplied by the elapsed time since launch. Without `--cost`, no price lookup is performed and no cost is shown.

#### Scenario: Cost is shown with flag

- **WHEN** the user runs `spinloop metrics --node <name> --cost` with a running instance
- **THEN** the report includes the estimated cost for the current session

#### Scenario: Cost is not shown by default

- **WHEN** the user runs `spinloop metrics --node <name>` without `--cost`
- **THEN** the report does not include a cost line

### Requirement: The metrics report's tabular display

The stats output SHALL support four formats via the `--format` flag: `gauge` (default), `bar`, `table`, and `json`. The `gauge` format SHALL produce a compact display with horizontal progress gauges for the current reading, colour-coded by utilization level. The `bar` format SHALL produce a compact display drawing each resource series as a sparkline of the daemon's retained history, with each point colour-coded by its own utilization level. The `table` format SHALL produce a tab-separated key-value table, one line per metric, with the key column left-aligned and values right of it. The `json` format SHALL output the response as a JSON object to standard output. Progress and error messages SHALL go to standard error regardless of format.

#### Scenario: Clean output

- **WHEN** the command succeeds
- **THEN** standard output contains only the stats data with no progress or debug lines

#### Scenario: Default format is gauge

- **WHEN** the user runs `spinloop metrics --node <name>` without `--format`
- **THEN** the output is in gauge format

#### Scenario: Table format is explicit

- **WHEN** the user runs `spinloop metrics --node <name> --format=table`
- **THEN** the output is in table format

#### Scenario: Bar format is explicit

- **WHEN** the user runs `spinloop metrics --node <name> --format=bar`
- **THEN** the output is in bar format, drawing each resource series as a sparkline of the daemon's retained history

#### Scenario: Gauge format is explicit

- **WHEN** the user runs `spinloop metrics --node <name> --format=gauge`
- **THEN** the output is in gauge format with progress gauges for the current reading

#### Scenario: JSON format

- **WHEN** the user runs `spinloop metrics --node <name> --format=json`
- **THEN** the output is valid JSON containing the instance state, runner, model, GPU info, CPU/RAM usage, and token counts

#### Scenario: JSON format with cost

- **WHEN** the user runs `spinloop metrics --node <name> --format=json --cost`
- **THEN** the JSON output includes a cost estimate field

#### Scenario: Invalid format errors

- **WHEN** the user runs `spinloop metrics --node <name> --format=csv`
- **THEN** the command exits with an error and usage message

### Requirement: The metrics report refreshes on request

The system SHALL support a `--watch`/`-w` flag that repeatedly queries metrics every 60 seconds. When enabled, the command SHALL clear the screen and redraw the output in place for each refresh, producing no scrollback accumulation. Each refresh SHALL pre-render the metrics output into a buffer before clearing the screen, so the redisplay is instantaneous after the network round-trip. The command SHALL continue until the user sends `SIGINT` (Ctrl+C) or `SIGTERM`, at which point it SHALL exit cleanly.

#### Scenario: Watch mode repeats output

- **WHEN** the user runs `spinloop metrics --node <name> --watch`
- **THEN** the command prints metrics, waits 60 seconds, clears the screen, and prints updated metrics

#### Scenario: Watch redraws in place

- **WHEN** the user runs `spinloop metrics --node <name> -w`
- **THEN** each refresh after the first clears the screen before displaying new output, with no separator lines

#### Scenario: Watch with JSON format

- **WHEN** the user runs `spinloop metrics --node <name> --watch --format=json`
- **THEN** each refresh clears the screen and outputs a JSON object

#### Scenario: Watch with cost

- **WHEN** the user runs `spinloop metrics --node <name> --watch --cost`
- **THEN** each refresh includes the cost estimate

#### Scenario: Watch stops on interrupt

- **WHEN** the user runs `spinloop metrics --node <name> -w` and presses Ctrl+C
- **THEN** the command exits cleanly without error

### Requirement: An environment's metrics report when it last did work

The metrics report SHALL include how long it has been since the endpoint's
engine last did any work, taken from the activity the on-instance daemon
tracks, in every format the command supports. The figure exists to answer "is
this endpoint doing anything?" at a glance, without the reader having to infer
it from the running-request count.

The figure SHALL be labelled "last active" rather than "idle": `idle` is
already an engine *state* meaning nothing has been started, and one report
SHALL NOT carry two meanings of the word. The elapsed time SHALL be rendered
the same way the command's other durations are, so an uptime and a last-active
figure read alike.

An endpoint whose daemon reports no activity — because no engine has run yet,
or because the daemon could not be reached — SHALL omit the figure rather than
show one implying the endpoint has been quiet since it started.

#### Scenario: A working endpoint reports its last activity

- **WHEN** the user runs `spinloop metrics --node <name>` against a running endpoint
  whose engine has served work
- **THEN** the report shows how long ago that work happened, labelled "last
  active"

#### Scenario: Every format carries the figure

- **WHEN** the user runs `spinloop metrics --node <name>` with `--format=bar`,
  `--format=table`, or `--format=json`
- **THEN** each output carries the last-active figure in its own idiom

#### Scenario: An endpoint that has done nothing omits the figure

- **WHEN** the user runs `spinloop metrics --node <name>` against an endpoint whose
  engine has not yet done any work
- **THEN** the report shows no last-active figure

#### Scenario: An unreachable daemon omits the figure

- **WHEN** the control plane cannot reach the instance's daemon to collect
  metrics
- **THEN** the report shows no last-active figure, and the rest of the report
  renders as it does today

### Requirement: History in the metrics report

When the on-instance daemon's metrics reply carries a history of system readings, the report SHALL carry it through to the command's output: the `json` format SHALL include the readings, and the `bar` format SHALL draw them. Where the daemon's reply carries no history, the report SHALL omit the field and the `bar` format SHALL fall back per the bar format specification. The control plane's relay of the daemon's reply SHALL NOT alter the readings it carries.

#### Scenario: JSON carries the daemon's history

- **WHEN** the instance's daemon reports a retained history and the user runs `spinloop metrics --node <name> --format=json`
- **THEN** the JSON output includes the history's readings

#### Scenario: Bar draws the relayed history

- **WHEN** the instance's daemon reports a retained history and the user runs `spinloop metrics --node <name> --format=bar`
- **THEN** each resource series is drawn as a sparkline from the readings the control plane relayed

#### Scenario: A daemon without history degrades

- **WHEN** the instance runs a daemon whose reply carries no history and the user runs `spinloop metrics --node <name> --format=bar`
- **THEN**    the report omits the history field and bar format draws the current reading in the gauge's filled style
