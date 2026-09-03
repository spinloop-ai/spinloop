package fleet

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spinloop-ai/spinloop/internal/daemon"
	"github.com/spinloop-ai/spinloop/internal/metrics"
	"github.com/spinloop-ai/spinloop/internal/remote"
)

// stubAWSCreds pins the AWS credential chain to static environment credentials
// and disables the EC2 metadata lookup, so a signed control call can run against
// an httptest server with no network or real account.
func stubAWSCreds(t *testing.T) {
	t.Helper()
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIATESTTESTTESTTEST")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret")
	t.Setenv("AWS_SESSION_TOKEN", "")
	t.Setenv("AWS_PROFILE", "")
	t.Setenv("AWS_CONFIG_FILE", filepath.Join(t.TempDir(), "no-such-file"))
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(t.TempDir(), "no-such-file"))
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
}

func boolPtr(b bool) *bool { return &b }

// remoteControlServer serves the shape a remote control endpoint answers.
func remoteControlServer(t *testing.T, body string, statusCode int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if statusCode != http.StatusOK {
			w.WriteHeader(statusCode)
		}
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestNewRemoteNodeRequiresACompleteConfig(t *testing.T) {
	if _, err := NewRemoteNode("env", remote.Config{StopURL: "http://x", Region: "r"}); err == nil {
		t.Error("missing start_url should be a configuration error")
	}
	if _, err := NewRemoteNode("env", remote.Config{StartURL: "http://x", StopURL: "http://x"}); err == nil {
		t.Error("missing region should be a configuration error")
	}
	if _, err := NewRemoteNode("env", remote.Config{StartURL: "http://x", StopURL: "http://x", Region: "r"}); err != nil {
		t.Errorf("a complete config should build a node: %v", err)
	}
}

func TestStatusFromRemote(t *testing.T) {
	got := statusFromRemote(remote.Response{
		State: "running", Healthy: boolPtr(true), BaseURL: "http://1.2.3.4:8000/v1",
		LastActiveAt: "2026-01-02T00:00:00Z", IdleSeconds: 30,
	})
	if got.State != "running" || got.IdleSeconds != 30 || got.LastActiveAt != "2026-01-02T00:00:00Z" {
		t.Errorf("statusFromRemote = %+v", got)
	}
	// A stopped endpoint reports no activity: nothing to measure from.
	if got := statusFromRemote(remote.Response{State: "stopped"}); got.State != "stopped" || got.LastActiveAt != "" {
		t.Errorf("stopped status = %+v", got)
	}
}

func TestStatsFromRemote(t *testing.T) {
	tokens := &metrics.TokenStats{Running: 2, PromptTokens: 5, GenerationTokens: 7, Requests: 3}
	got := statsFromRemote(remote.StatsResponse{
		State: "running", Runner: "llamacpp", ModelID: "org/m", UptimeSeconds: 10,
		Tokens: tokens, LastActiveAt: "2026-01-02T00:00:00Z", IdleSeconds: 5, Version: "1.2.3",
	})
	if got.State != "running" || got.Runner != "llamacpp" || got.ModelID != "org/m" || got.UptimeSeconds != 10 {
		t.Errorf("statsFromRemote = %+v", got)
	}
	if got.Tokens == nil || got.Tokens.Running != 2 || got.Tokens.Requests != 3 {
		t.Errorf("token stats not carried over: %+v", got.Tokens)
	}
	if got.IdleSeconds != 5 || got.LastActiveAt == "" {
		t.Errorf("activity not carried over: %+v", got)
	}
}

func TestLogsFromRemote(t *testing.T) {
	if got := logsFromRemote(nil, true); !got.Missing {
		t.Errorf("no fresh events on a from-the-beginning read should be reported missing, got %+v", got)
	}
	// No fresh events on a later poll — the common steady state once a follow
	// is caught up — must not be reported missing: the log is not gone, it is
	// just quiet right now.
	if got := logsFromRemote(nil, false); got.Missing {
		t.Errorf("no fresh events on a later poll should not be reported missing, got %+v", got)
	}

	t1 := time.UnixMilli(1000)
	t2 := time.UnixMilli(2000)
	got := logsFromRemote([]remote.LogEvent{
		{Message: "loading model", Timestamp: t1}, {Message: "server ready", Timestamp: t2},
	}, false)
	want := "loading model\nserver ready\n"
	if got.Content != want {
		t.Errorf("content = %q, want %q", got.Content, want)
	}
	if got.NextOffset != t2.UnixMilli() {
		t.Errorf("nextOffset = %d, want the newest event's millisecond %d", got.NextOffset, t2.UnixMilli())
	}
	if got.Size != int64(len(want)) {
		t.Errorf("size = %d, want %d", got.Size, len(want))
	}
}

// A remote node's log follow uses the exact same cursor as `spinloop remote
// logs -f` (remote.FollowCursor, seeded with remote.FollowOverlap) — not a
// lookalike reimplementation — so a second poll bounds its query behind the
// newest event already shown by the same overlap window, and does not show
// that event again. The bug this guards is the tail being replayed in full
// on every poll regardless of what was already shown.
func TestRemoteNodeLogsSharesTheFollowCursorWithRemoteLogsCommand(t *testing.T) {
	stubAWSCreds(t)
	eventMs := time.Date(2026, 8, 9, 11, 30, 0, 0, time.UTC).UnixMilli()

	var mu sync.Mutex
	served := map[string]bool{}
	var starts []*int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var in struct {
			LogGroupName string `json:"logGroupName"`
			StartTime    *int64 `json:"startTime"`
		}
		json.Unmarshal(body, &in)

		mu.Lock()
		starts = append(starts, in.StartTime)
		first := !served[in.LogGroupName]
		served[in.LogGroupName] = true
		mu.Unlock()

		w.Header().Set("Content-Type", "application/x-amz-json-1.1")
		if first {
			// Each group ships the same one event, the first time it is asked —
			// the fixture's stand-in for "the engine's log group has this".
			fmt.Fprintf(w, `{"events":[{"eventId":"e1","timestamp":%d,"logStreamName":"env-1/i-1","message":"hello"}]}`, eventMs)
			return
		}
		w.Write([]byte(`{"events":[]}`))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("AWS_ENDPOINT_URL_CLOUDWATCH_LOGS", srv.URL)
	cfg := remote.Config{StartURL: "http://x", StopURL: "http://x", Environment: "env-1", Region: "us-east-1"}
	node, err := NewRemoteNode("env", cfg)
	if err != nil {
		t.Fatal(err)
	}

	resp1, err := node.Logs(context.Background(), daemon.TailLog, 0)
	if err != nil {
		t.Fatalf("tail poll: %v", err)
	}
	if !strings.Contains(resp1.Content, "hello") {
		t.Fatalf("the tail poll should show the event, got %+v", resp1)
	}
	for _, s := range starts {
		if s != nil {
			t.Errorf("the tail poll (offset TailLog) sent a start bound: %v", *s)
		}
	}

	starts = nil
	resp2, err := node.Logs(context.Background(), 0, 0)
	if err != nil {
		t.Fatalf("follow-up poll: %v", err)
	}
	if resp2.Content != "" || resp2.Missing {
		t.Errorf("the follow-up poll should show nothing new and not report missing, got %+v", resp2)
	}
	wantStart := eventMs - remote.FollowOverlap.Milliseconds()
	for _, s := range starts {
		if s == nil {
			t.Fatal("the follow-up poll sent no start bound")
		}
		if *s != wantStart {
			t.Errorf("start bound = %d, want the shared overlap behind the last event (%d)", *s, wantStart)
		}
	}
}

func TestRemoteNodeStartWithIsRefused(t *testing.T) {
	node, _ := NewRemoteNode("env", remote.Config{StartURL: "http://x", StopURL: "http://x", Region: "r"})
	_, err := node.StartWith(context.Background(), &remote.DeployConfig{Runner: "llamacpp"}, "")
	if err == nil || !strings.Contains(err.Error(), "spinloop remote deploy") {
		t.Errorf("StartWith should refuse, naming the deploy path; got %v", err)
	}
}

func TestRemoteNodeStatusOverTheControlPlane(t *testing.T) {
	stubAWSCreds(t)
	srv := remoteControlServer(t,
		`{"state":"running","healthy":true,"lastActiveAt":"2026-01-02T00:00:00Z","idleSeconds":30}`, http.StatusOK)
	node, err := NewRemoteNode("env", remote.Config{StartURL: srv.URL, StopURL: srv.URL, Region: "us-east-1"})
	if err != nil {
		t.Fatal(err)
	}
	r := StatusCall(context.Background(), node)
	if !r.OK() || r.Status.State != "running" || r.Status.IdleSeconds != 30 {
		t.Errorf("status result = %+v", r)
	}
	if r.Name != "env" {
		t.Errorf("name = %q", r.Name)
	}
}

// A remote node drives start, stop and metrics over its control plane exactly
// like a node would, mapping each reply onto the node's types.
func TestRemoteNodeStartStopMetricsOverTheControlPlane(t *testing.T) {
	stubAWSCreds(t)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /start", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"state":"ready","healthy":true}`))
	})
	mux.HandleFunc("POST /stop", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"state":"stopped","healthy":false}`))
	})
	mux.HandleFunc("GET /stats", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"state":"running","runner":"llamacpp","modelId":"org/m","uptimeSeconds":9,"tokens":{"running":1,"requests":2},"version":"1.0.0"}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	cfg := remote.Config{StartURL: srv.URL + "/start", StopURL: srv.URL + "/stop", StatsURL: srv.URL + "/stats", Region: "us-east-1"}
	node, err := NewRemoteNode("env", cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	started, err := node.Start(ctx)
	if err != nil || started.State != "ready" {
		t.Errorf("Start = %+v, %v", started, err)
	}
	stats, err := node.Metrics(ctx)
	if err != nil || stats.State != "running" || stats.ModelID != "org/m" {
		t.Errorf("Metrics = %+v, %v", stats, err)
	}
	if stats.Tokens == nil || stats.Tokens.Running != 1 || stats.Tokens.Requests != 2 {
		t.Errorf("metrics tokens not mapped: %+v", stats.Tokens)
	}
	stopped, err := node.Stop(ctx)
	if err != nil || stopped.State != "stopped" {
		t.Errorf("Stop = %+v, %v", stopped, err)
	}
}

// A node whose config has no stats endpoint reports that, rather than a silent
// empty result: a control plane predating stats is a readable state.
func TestRemoteNodeMetricsWithoutStatsURL(t *testing.T) {
	node, err := NewRemoteNode("env", remote.Config{StartURL: "http://x", StopURL: "http://x", Region: "r"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = node.Metrics(context.Background())
	if err == nil || !strings.Contains(err.Error(), "stats_url") {
		t.Errorf("metrics for a config with no stats_url should name the gap, got %v", err)
	}
}

// A node for an environment with no name cannot be pointed at a log stream, so a
// log read fails before it touches CloudWatch.
func TestRemoteNodeLogsWithoutAnEnvironmentFails(t *testing.T) {
	node, _ := NewRemoteNode("env", remote.Config{StartURL: "http://x", StopURL: "http://x", Region: "r"})
	_, err := node.Logs(context.Background(), daemon.TailLog, 100)
	if err == nil || !strings.Contains(err.Error(), "environment") {
		t.Errorf("a log read for an environment with no name should fail, got %v", err)
	}
}

func TestRemoteNodeRejectedCallIsAOutcomeNotAFailure(t *testing.T) {
	stubAWSCreds(t)
	srv := remoteControlServer(t, `{"error":"boom"}`, http.StatusInternalServerError)
	node, _ := NewRemoteNode("env", remote.Config{StartURL: srv.URL, StopURL: srv.URL, Region: "us-east-1"})
	r := StatusCall(context.Background(), node)
	// A rejected call is a typed outcome carrying the reason, not an error that
	// would blank the rest of the node set.
	if r.OK() {
		t.Fatal("a 500 control reply should not be OK")
	}
	if !strings.Contains(r.Detail(), "boom") {
		t.Errorf("detail should carry the reason, got %q", r.Detail())
	}
}

// A mixed set — a local daemon node and a remote environment — is observed
// through the one fan-out, in order, and one failing member does not stop the
// rest.
func TestFanOutNodesOverAMixedSet(t *testing.T) {
	stubAWSCreds(t)

	up := stubDaemon(t, "", "running")
	cfg := fleetFor(t, up, "")
	entry, _ := cfg.Node("box")
	dnode, err := cfg.NewNode(entry)
	if err != nil {
		t.Fatal(err)
	}

	remoteUp := remoteControlServer(t, `{"state":"running","healthy":true}`, http.StatusOK)
	rnode, _ := NewRemoteNode("env", remote.Config{StartURL: remoteUp.URL, StopURL: remoteUp.URL, Region: "us-east-1"})

	results := FanOutNodes(context.Background(), StatusCall, []Node{dnode, rnode})
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	// Order is the order given, so the two kinds sit side by side.
	if results[0].Name != "box" || results[1].Name != "env" {
		t.Fatalf("order = %q, %q; want box then env", results[0].Name, results[1].Name)
	}
	if !results[0].OK() || results[0].Status.State != "running" {
		t.Errorf("daemon node result = %+v", results[0])
	}
	if !results[1].OK() || results[1].Status.State != "running" {
		t.Errorf("remote node result = %+v", results[1])
	}
}

func TestFanOutNodesKeepsAGoodNodeWhenAnotherFails(t *testing.T) {
	stubAWSCreds(t)
	good := remoteControlServer(t, `{"state":"running"}`, http.StatusOK)
	rnode, _ := NewRemoteNode("env", remote.Config{StartURL: good.URL, StopURL: good.URL, Region: "us-east-1"})

	results := FanOutNodes(context.Background(), StatusCall, []Node{&failingNode{name: "bad"}, rnode})
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	if results[0].OK() {
		t.Error("the failing node should not be OK")
	}
	if !results[1].OK() {
		t.Errorf("a good node in the set should still be observed: %+v", results[1])
	}
}

// failingNode is a node whose status call always errors, for fan-out tests.
type failingNode struct{ name string }

func (n *failingNode) Name() string { return n.name }
func (n *failingNode) Status(context.Context) (daemon.StatusResponse, error) {
	return daemon.StatusResponse{}, fmt.Errorf("no such host: %s", n.name)
}
func (n *failingNode) Metrics(context.Context) (metrics.Stats, error) {
	return metrics.Stats{}, nil
}
func (n *failingNode) Start(context.Context) (daemon.StatusResponse, error) {
	return daemon.StatusResponse{}, nil
}
func (n *failingNode) StartWith(context.Context, *remote.DeployConfig, string) (daemon.StatusResponse, error) {
	return daemon.StatusResponse{}, nil
}
func (n *failingNode) Stop(context.Context) (daemon.StatusResponse, error) {
	return daemon.StatusResponse{}, nil
}
func (n *failingNode) Logs(context.Context, int64, int) (daemon.LogsResponse, error) {
	return daemon.LogsResponse{}, nil
}

// StartWithProgress carries the control plane's own lines up to the caller:
// the retry during a boot, then the final verdict.
func TestRemoteNodeStartWithProgressReportsTheBoot(t *testing.T) {
	stubAWSCreds(t)
	var (
		mu       sync.Mutex
		lines    []string
		attempts int
	)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /start", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		attempts++
		first := attempts == 1
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if first { // the boot needs a moment; the endpoint says so
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"state":"starting","retryAfterSeconds":1}`))
			return
		}
		w.Write([]byte(`{"state":"ready","healthy":true}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	cfg := remote.Config{StartURL: srv.URL + "/start", StopURL: srv.URL + "/start", Region: "us-east-1"}
	node, err := NewRemoteNode("env", cfg)
	if err != nil {
		t.Fatal(err)
	}
	starter, ok := node.(ProgressStarter)
	if !ok {
		t.Fatal("the remote node does not carry progress")
	}
	resp, err := starter.StartWithProgress(context.Background(), func(line string) {
		mu.Lock()
		lines = append(lines, line)
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("StartWithProgress: %v", err)
	}
	if resp.State != "ready" {
		t.Errorf("state = %q", resp.State)
	}
	mu.Lock()
	defer mu.Unlock()
	if attempts != 2 {
		t.Errorf("start attempts = %d (want 2)", attempts)
	}
	if len(lines) < 1 || !strings.Contains(lines[0], "instance starting; retrying in 1s") {
		t.Errorf("progress lines = %q (want the boot's retry line)", lines)
	}
}

// A start the control plane refuses, and a stop it refuses, both surface as
// errors the node's caller can show on its row.
func TestRemoteNodeRejectedOperationsSurfaceAsErrors(t *testing.T) {
	stubAWSCreds(t)
	srv := remoteControlServer(t, `{"error":"boom"}`, http.StatusInternalServerError)
	cfg := remote.Config{StartURL: srv.URL, StopURL: srv.URL, Region: "us-east-1"}
	node, err := NewRemoteNode("env", cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	starter, ok := node.(ProgressStarter)
	if !ok {
		t.Fatal("the remote node does not carry progress")
	}
	if _, err := starter.StartWithProgress(ctx, nil); err == nil {
		t.Error("a rejected start did not fail")
	}
	if _, err := node.Stop(ctx); err == nil {
		t.Error("a rejected stop did not fail")
	}
}

// A failing log read surfaces as an error, not as an empty tail: the caller
// renders the detail rather than reporting "no logs" when the read itself
// went wrong.
func TestRemoteNodeLogsErrorsSurface(t *testing.T) {
	stubAWSCreds(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest) // a client error: the SDK will not retry it
		w.Write([]byte(`{"message":"boom"}`))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("AWS_ENDPOINT_URL_CLOUDWATCH_LOGS", srv.URL)
	cfg := remote.Config{StartURL: "http://x", StopURL: "http://x", Environment: "env-1", Region: "us-east-1"}
	node, err := NewRemoteNode("env", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := node.Logs(context.Background(), 0, 100); err == nil {
		t.Error("a failing log read surfaced as success")
	}
}

// A log store that answers with an empty tail is a readable state — the
// engine has not logged here — so the node reports missing, not an error.
func TestRemoteNodeLogsEmptyTailIsMissingNotFailure(t *testing.T) {
	stubAWSCreds(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-amz-json-1.1")
		w.Write([]byte(`{"events":[]}`))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("AWS_ENDPOINT_URL_CLOUDWATCH_LOGS", srv.URL)
	cfg := remote.Config{StartURL: "http://x", StopURL: "http://x", Environment: "env-1", Region: "us-east-1"}
	node, err := NewRemoteNode("env", cfg)
	if err != nil {
		t.Fatal(err)
	}
	got, err := node.Logs(context.Background(), 0, 100)
	if err != nil {
		t.Fatalf("an empty tail is not a failure: %v", err)
	}
	if !got.Missing {
		t.Errorf("an empty tail should be reported missing: %+v", got)
	}
}
