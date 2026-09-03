package fleet

import (
	"context"
	"fmt"
	"strings"

	"github.com/spinloop-ai/spinloop/internal/daemon"
	"github.com/spinloop-ai/spinloop/internal/metrics"
	"github.com/spinloop-ai/spinloop/internal/remote"
)

// remoteNode is one member of the fleet as seen through a remote, scale-to-zero
// environment: a `remote.Config` reached over its cloud control plane rather than a
// machine's daemon. It answers the same operations a local node answers, so the same
// fan-out and rendering pass over both kinds — which is the whole point of routing a
// remote environment through the node contract.
//
// The control-plane state (EC2) and a daemon's engine state are different
// vocabularies. We do not translate one into the other: the status carries the
// control-plane state as the control plane reports it. A remote environment's
// "running" is the instance state, not a claim about the engine.
type remoteNode struct {
	name string
	cfg  remote.Config
	// logs holds the position a follow of this node's engine log has reached.
	// It is the same cursor `spinloop remote logs -f` uses, and for the same
	// reason: CloudWatch has no resumable read position of its own, so a poll
	// re-asks a little behind the newest event already seen and this
	// suppresses what the overlap re-reads, by event id.
	logs *remote.FollowCursor
}

// NewRemoteNode builds the live node for a named remote environment. The config
// must be complete enough to send a signed control call — start, stop and region —
// so a config that cannot be resolved is a configuration error against this node
// rather than a call that fails part-way through.
func NewRemoteNode(name string, cfg remote.Config) (Node, error) {
	if cfg.StartURL == "" || cfg.StopURL == "" || cfg.Region == "" {
		return nil, fmt.Errorf(
			"remote environment %q is not fully configured: start_url, stop_url and region are all required",
			name)
	}
	return &remoteNode{name: name, cfg: cfg, logs: remote.NewFollowCursor(remote.FollowOverlap)}, nil
}

func (n *remoteNode) Name() string { return n.name }

func (n *remoteNode) Status(ctx context.Context) (daemon.StatusResponse, error) {
	resp, err := remote.Status(ctx, n.cfg)
	if err != nil {
		return daemon.StatusResponse{}, err
	}
	return statusFromRemote(*resp), nil
}

func (n *remoteNode) Metrics(ctx context.Context) (metrics.Stats, error) {
	resp, err := remote.Stats(ctx, n.cfg)
	if err != nil {
		return metrics.Stats{}, err
	}
	return statsFromRemote(*resp), nil
}

func (n *remoteNode) Start(ctx context.Context) (daemon.StatusResponse, error) {
	return n.StartWithProgress(ctx, func(string) {})
}

// StartWithProgress is Start with the control plane's status lines passed to
// progress as the boot proceeds: the environment's state and the wait until the
// next poll, one line per retry, plus one line each time a new attempt is
// issued. Callers may render or discard them.
func (n *remoteNode) StartWithProgress(ctx context.Context, progress func(string)) (daemon.StatusResponse, error) {
	if progress == nil {
		// remote.Start invokes progress on every retry path, so nil is
		// substituted with a no-op here rather than left to whichever paths a
		// given start takes.
		progress = func(string) {}
	}
	onState := func(state string) {
		if line := startStateLine(state); line != "" {
			progress(line)
		}
	}
	resp, err := remote.Start(ctx, n.cfg, progress, onState, nil)
	if err != nil {
		return daemon.StatusResponse{}, err
	}
	return statusFromRemote(*resp), nil
}

// startStateLine returns the progress line for a poll's state, or "" for states
// that remote.Start already writes a line for.
//
// Only StateInFlight returns a line, and it exists to overwrite a wait notice
// that is no longer current. remote.Start writes its progress lines immediately
// before a wait ("instance no-capacity; retrying in 120s"); each is accurate
// only until the next attempt is issued. The attempt that obtains capacity then
// holds a single request open for the duration of the boot — minutes — and
// writes no further line. A caller that appends lines to a scrolling log is
// unaffected either way. A caller that renders the most recent line as the
// node's current state, such as the dashboard tile, would otherwise display a
// capacity wait for the remainder of the start, after the instance is up and
// serving. StateInFlight was added to the remote client for this case.
func startStateLine(state string) string {
	if state == remote.StateInFlight {
		return "waking the instance…"
	}
	return ""
}

// StartWith is how a router wakes a node to serve something. A remote environment
// is not woken: what it serves is set by `spinloop remote deploy`, a heavier flow
// (provisioning, weight seeding, ingress) that a node start must not conflate.
func (n *remoteNode) StartWith(ctx context.Context, dc *remote.DeployConfig, engineKey string) (daemon.StatusResponse, error) {
	_ = dc
	_ = engineKey
	return daemon.StatusResponse{}, fmt.Errorf(
		"%s is a remote environment, not a node to be woken: tell it what to serve with `spinloop remote deploy`",
		n.name)
}

func (n *remoteNode) Stop(ctx context.Context) (daemon.StatusResponse, error) {
	resp, err := remote.Stop(ctx, n.cfg)
	if err != nil {
		return daemon.StatusResponse{}, err
	}
	return statusFromRemote(*resp), nil
}

// remoteEngineTail caps how many engine log events a node read pulls. Remote logs
// are a bounded tail pulled from the log store, not a byte cursor, so a follow of
// a chatty engine must not page through an unbounded window.
const remoteEngineTail = 1000

func (n *remoteNode) Logs(ctx context.Context, offset int64, limit int) (daemon.LogsResponse, error) {
	// daemon.TailLog means a fresh open of the view: start the cursor over,
	// so this open shows its own tail rather than having it suppressed as
	// already seen by whatever this node last followed.
	if offset == daemon.TailLog {
		n.logs.Reset()
	}
	start := n.logs.Start()
	res, err := remote.FetchLogs(ctx, n.cfg, remote.LogQuery{
		Environment: n.cfg.Environment,
		Source:      remote.LogSourceEngine,
		Limit:       remoteEngineTail,
		Start:       start,
	})
	if err != nil {
		return daemon.LogsResponse{}, err
	}
	fresh := n.logs.Advance(res.Events)
	// Missing only on the read that had no lower bound yet finding nothing:
	// that is genuinely no log, ever. A later poll with nothing new is a
	// quiet log, not a missing one.
	missing := start.IsZero() && len(res.Events) == 0
	return logsFromRemote(fresh, missing), nil
}

// statusFromRemote maps the control plane's status reply onto the node's status.
// It carries only what a control-plane reply can honestly be mapped across: the
// state and its last-active record. The version is not in this reply — the stats
// reply carries it — so it is empty here, and runner/model are likewise absent.
func statusFromRemote(resp remote.Response) daemon.StatusResponse {
	return daemon.StatusResponse{
		State:        resp.State,
		LastActiveAt: resp.LastActiveAt,
		IdleSeconds:  resp.IdleSeconds,
	}
}

// statsFromRemote maps the stats Lambda's reply onto the shared stats shape. The
// per-stat fields already alias the metrics types the collector produces, so the
// mapping is a field-for-field copy; the reply's version and instance facts have
// no home on the node's stats and are left to the remote view.
func statsFromRemote(resp remote.StatsResponse) metrics.Stats {
	return metrics.Stats{
		State:         resp.State,
		Runner:        resp.Runner,
		ModelID:       resp.ModelID,
		UptimeSeconds: resp.UptimeSeconds,
		Tokens:        resp.Tokens,
		GPUs:          resp.GPUs,
		CPU:           resp.CPU,
		Memory:        resp.Memory,
		Errors:        resp.Errors,
		LastActiveAt:  resp.LastActiveAt,
		IdleSeconds:   resp.IdleSeconds,
	}
}

// logsFromRemote maps one poll's fresh events — the ones remoteNode.Logs's
// FollowCursor has not already returned — onto the node's log reply. Events
// arrive oldest first, so the content reads top to bottom. missing reports
// that this was a from-the-beginning read that found nothing: the state a
// reader can act on (the engine has not run here, or has sent nothing),
// distinct from a later poll simply having nothing new to add. NextOffset
// only needs to say "not a fresh open" to the next call — the real position
// lives in the node's own cursor — so it carries the newest event shown for
// whoever finds that useful to see, and 0 has the same effect when there is
// none.
func logsFromRemote(fresh []remote.LogEvent, missing bool) daemon.LogsResponse {
	if len(fresh) == 0 {
		return daemon.LogsResponse{Missing: missing}
	}
	msgs := make([]string, len(fresh))
	for i, e := range fresh {
		msgs[i] = e.Message
	}
	// Each event is already one complete, discrete line — unlike a byte-stream
	// tail, there is never a trailing partial line to leave unterminated — so
	// the join ends in a newline the same way a local engine log's lines do.
	content := strings.Join(msgs, "\n") + "\n"
	return daemon.LogsResponse{
		Content:    content,
		NextOffset: fresh[len(fresh)-1].Timestamp.UnixMilli(),
		Size:       int64(len(content)),
	}
}
