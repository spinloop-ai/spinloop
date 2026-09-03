package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/spinloop-ai/spinloop/internal/daemon"
	"github.com/spinloop-ai/spinloop/internal/fleet"
	"github.com/spinloop-ai/spinloop/internal/metrics"
	"github.com/spinloop-ai/spinloop/internal/remote"
)

// stubNode serves a daemon control API for one fleet node.
func stubNode(t *testing.T, state string) *httptest.Server {
	return stubNodeWithVersion(t, state, "")
}

// stubNodeWithVersion is like stubNode but also returns a version field.
func stubNodeWithVersion(t *testing.T, state string, ver string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	status := map[string]any{
		"state": state, "runner": "llamacpp", "model": "org/qwen", "uptimeSeconds": 65,
	}
	if ver != "" {
		status["version"] = ver
	}
	mux.HandleFunc("GET /v1/status", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(status)
	})
	mux.HandleFunc("GET /v1/metrics", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"state": state, "runner": "llamacpp", "modelId": "org/qwen",
			"cpu":    map[string]any{"utilization": 30.0},
			"memory": map[string]any{"total": 1000, "used": 400},
			"tokens": map[string]any{"running": 2, "promptTokens": 4096, "generationTokens": 1024, "requests": 17},
		})
	})
	mux.HandleFunc("POST /v1/start", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"state": "running"})
	})
	mux.HandleFunc("POST /v1/stop", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"state": "stopped"})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// hostPort splits an httptest server URL into host and port.
func hostPort(t *testing.T, srv *httptest.Server) (string, int) {
	t.Helper()
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatal(err)
	}
	return u.Hostname(), port
}

// writeFleetFile writes a fleet.yaml in a temp dir, chdirs there, and returns
// the directory — so the commands resolve ./fleet.yaml the way a user would.
func writeFleetFile(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fleet.yaml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	return dir
}

// twoNodeFleet writes a fleet of one reachable node and one that is down.
// "up" declares a file field naming a minimal Spinloop, so `fleet start`
// resolves a source for it — required for any kind: daemon node since
// resolution became mandatory (see resolveNodeSpinloop).
func twoNodeFleet(t *testing.T, state string) *httptest.Server {
	t.Helper()
	t.Setenv("SPINLOOP_CONFIG_DIR", t.TempDir())
	up := stubNode(t, state)
	host, port := hostPort(t, up)
	dir := writeFleetFile(t, fmt.Sprintf(
		"nodes:\n  - name: up\n    host: %s\n    port: %d\n    file: ./up.Spinloop\n  - name: down\n    host: 127.0.0.1\n    port: 1\n",
		host, port))
	if err := os.WriteFile(filepath.Join(dir, "up.Spinloop"), []byte("PROVIDER llamacpp\nMODEL org/m:Q4\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return up
}

func TestCmdFleetStatusRendersEveryNode(t *testing.T) {
	twoNodeFleet(t, "running")
	out := captureStdout(t, func() {
		if err := cmdFleet([]string{"status"}); err != nil {
			// One unreachable node must not fail the command.
			t.Errorf("fleet status returned %v", err)
		}
	})
	if !strings.Contains(out, "NODE") || !strings.Contains(out, "STATE") {
		t.Errorf("no header:\n%s", out)
	}
	// The reachable node shows its state and what it serves.
	if !strings.Contains(out, "up") || !strings.Contains(out, "running") || !strings.Contains(out, "org/qwen") {
		t.Errorf("reachable node row missing:\n%s", out)
	}
	// The unreachable node is a row with its reason, not an omission.
	if !strings.Contains(out, "down") || !strings.Contains(out, "unreachable") {
		t.Errorf("unreachable node not rendered:\n%s", out)
	}
}

// Fleet status includes the daemon version so the operator can spot a
// partially-upgraded fleet without SSH access.
func TestCmdFleetStatusShowsVersion(t *testing.T) {
	srv := stubNodeWithVersion(t, "running", "1.18.0")
	host, port := hostPort(t, srv)
	writeFleetFile(t, fmt.Sprintf(
		"nodes:\n  - name: node\n    host: %s\n    port: %d\n",
		host, port))
	out := captureStdout(t, func() {
		if err := cmdFleet([]string{"status"}); err != nil {
			t.Error(err)
		}
	})
	if !strings.Contains(out, "1.18.0") {
		t.Errorf("version not rendered:\n%s", out)
	}
}

func TestCmdFleetMetricsFormats(t *testing.T) {
	twoNodeFleet(t, "running")

	t.Run("bar", func(t *testing.T) {
		out := captureStdout(t, func() {
			if err := cmdFleet([]string{"metrics"}); err != nil {
				t.Error(err)
			}
		})
		if !strings.Contains(out, "CPU") || !strings.Contains(out, "RAM") {
			t.Errorf("no resource bars:\n%s", out)
		}
		if !strings.Contains(out, "prompt tokens") {
			t.Errorf("no token block:\n%s", out)
		}
		if !strings.Contains(out, "unreachable") {
			t.Errorf("unreachable node omitted:\n%s", out)
		}
	})

	t.Run("table", func(t *testing.T) {
		out := captureStdout(t, func() {
			if err := cmdFleet([]string{"metrics", "--format=table"}); err != nil {
				t.Error(err)
			}
		})
		if !strings.Contains(out, "CPU:") || !strings.Contains(out, "RAM:") {
			t.Errorf("no table lines:\n%s", out)
		}
	})

	t.Run("json covers the whole fleet", func(t *testing.T) {
		out := captureStdout(t, func() {
			if err := cmdFleet([]string{"metrics", "--format=json"}); err != nil {
				t.Error(err)
			}
		})
		var decoded []struct {
			Node    string          `json:"node"`
			Outcome string          `json:"outcome"`
			Error   string          `json:"error"`
			Metrics json.RawMessage `json:"metrics"`
		}
		if err := json.Unmarshal([]byte(out), &decoded); err != nil {
			t.Fatalf("output is not valid JSON: %v\n%s", err, out)
		}
		if len(decoded) != 2 {
			t.Fatalf("got %d entries, want one per node", len(decoded))
		}
		// File order is preserved and every node is accounted for — the
		// unreachable one included, with its reason.
		if decoded[0].Node != "up" || decoded[0].Outcome != "ok" || len(decoded[0].Metrics) == 0 {
			t.Errorf("entry 0 = %+v", decoded[0])
		}
		if decoded[1].Node != "down" || decoded[1].Outcome != "unreachable" || decoded[1].Error == "" {
			t.Errorf("entry 1 = %+v", decoded[1])
		}
	})
}

func TestCmdFleetMetricsRejectsBadFormat(t *testing.T) {
	twoNodeFleet(t, "running")
	if err := cmdFleet([]string{"metrics", "--format=csv"}); err == nil {
		t.Fatal("--format=csv accepted")
	}
}

func TestCmdFleetStartStopDriveOneNode(t *testing.T) {
	twoNodeFleet(t, "idle")

	out := captureStdout(t, func() {
		if err := cmdFleet([]string{"start", "up"}); err != nil {
			t.Error(err)
		}
	})
	if !strings.Contains(out, "up") || !strings.Contains(out, "running") {
		t.Errorf("start output = %q", out)
	}

	out = captureStdout(t, func() {
		if err := cmdFleet([]string{"stop", "up"}); err != nil {
			t.Error(err)
		}
	})
	if !strings.Contains(out, "stopped") {
		t.Errorf("stop output = %q", out)
	}
}

// Mutating verbs demand an explicit target: with no node and no --all they
// list the fleet and touch nothing, rather than acting on the whole fleet
// by accident.
func TestCmdFleetStartStopRequireANode(t *testing.T) {
	twoNodeFleet(t, "idle")
	for _, verb := range []string{"start", "stop"} {
		err := cmdFleet([]string{verb})
		if err == nil {
			t.Fatalf("fleet %s with no node was accepted", verb)
		}
		// The message lists the fleet so the user can see what to type.
		for _, want := range []string{"up", "down"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("fleet %s error %q does not list node %q", verb, err, want)
			}
		}
	}
}

// threeStubNodesFleet writes a fleet of three reachable daemon nodes, each
// declaring a file field naming a minimal Spinloop, so `fleet start` (which
// now requires a resolvable source for every kind: daemon node) can start
// any of them.
func threeStubNodesFleet(t *testing.T, state string) {
	t.Helper()
	t.Setenv("SPINLOOP_CONFIG_DIR", t.TempDir())
	var lines strings.Builder
	for _, name := range []string{"a", "b", "c"} {
		srv := stubNode(t, state)
		host, port := hostPort(t, srv)
		fmt.Fprintf(&lines, "  - name: %s\n    host: %s\n    port: %d\n    file: ./%s.Spinloop\n", name, host, port, name)
	}
	dir := writeFleetFile(t, "nodes:\n"+lines.String())
	for _, name := range []string{"a", "b", "c"} {
		path := filepath.Join(dir, name+".Spinloop")
		if err := os.WriteFile(path, []byte("PROVIDER llamacpp\nMODEL org/m:Q4\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCmdFleetStartSeveralNamedNodes(t *testing.T) {
	threeStubNodesFleet(t, "idle")
	out := captureStdout(t, func() {
		if err := cmdFleet([]string{"start", "a", "b"}); err != nil {
			t.Error(err)
		}
	})
	for _, want := range []string{"a  running", "b  running"} {
		if !strings.Contains(out, want) {
			t.Errorf("start a b output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "c") {
		t.Errorf("start a b should not touch c:\n%s", out)
	}
}

func TestCmdFleetStartAll(t *testing.T) {
	threeStubNodesFleet(t, "idle")
	out := captureStdout(t, func() {
		if err := cmdFleet([]string{"start", "--all"}); err != nil {
			t.Error(err)
		}
	})
	for _, want := range []string{"a  running", "b  running", "c  running"} {
		if !strings.Contains(out, want) {
			t.Errorf("start --all output missing %q:\n%s", want, out)
		}
	}
}

func TestCmdFleetStopSeveralNamedNodesAndAll(t *testing.T) {
	threeStubNodesFleet(t, "running")
	out := captureStdout(t, func() {
		if err := cmdFleet([]string{"stop", "a", "b"}); err != nil {
			t.Error(err)
		}
	})
	for _, want := range []string{"a  stopped", "b  stopped"} {
		if !strings.Contains(out, want) {
			t.Errorf("stop a b output missing %q:\n%s", want, out)
		}
	}

	out = captureStdout(t, func() {
		if err := cmdFleet([]string{"stop", "--all"}); err != nil {
			t.Error(err)
		}
	})
	for _, want := range []string{"a  stopped", "b  stopped", "c  stopped"} {
		if !strings.Contains(out, want) {
			t.Errorf("stop --all output missing %q:\n%s", want, out)
		}
	}
}

func TestCmdFleetStartStopAllPlusNamesIsAmbiguous(t *testing.T) {
	threeStubNodesFleet(t, "idle")
	for _, verb := range []string{"start", "stop"} {
		err := cmdFleet([]string{verb, "--all", "a"})
		if err == nil || !strings.Contains(err.Error(), "ambiguous") {
			t.Errorf("fleet %s --all a: want an ambiguous-target error, got %v", verb, err)
		}
	}
}

func TestCmdFleetStartUnknownNameAmongSeveralFailsBeforeStartingAny(t *testing.T) {
	threeStubNodesFleet(t, "idle")
	out := captureStdout(t, func() {
		err := cmdFleet([]string{"start", "a", "nope"})
		if err == nil || !strings.Contains(err.Error(), "nope") {
			t.Errorf("want an unknown-node error naming it, got %v", err)
		}
	})
	if out != "" {
		t.Errorf("nothing should have started, got output:\n%s", out)
	}
}

// A kind: daemon node with no file field, no matching alias, and no
// matching subdirectory cannot resolve a Spinloop source, so fleet start
// fails for it — no fallback to a plain, config-less start.
func TestCmdFleetStartDaemonNodeWithNoResolvableSourceFails(t *testing.T) {
	twoNodeFleet(t, "idle") // "down" declares no file field
	var err error
	out := captureStdout(t, func() {
		err = cmdFleet([]string{"start", "down"})
	})
	if err == nil {
		t.Fatal("start on a node with no resolvable Spinloop source was accepted")
	}
	if !strings.Contains(err.Error(), "down") {
		t.Errorf("error %q should name the failed node", err)
	}
	for _, want := range []string{"file", "alias", "subdirectory"} {
		if !strings.Contains(out, want) {
			t.Errorf("output %q should mention %q", out, want)
		}
	}
}

// fakeFleetNode is a minimal fleet.Node for exercising fleetStartCall's
// dispatch directly, without a real daemon or control plane — it just
// counts which of Start/StartWith was called.
type fakeFleetNode struct {
	name                       string
	startCalls, startWithCalls int
}

func (f *fakeFleetNode) Name() string { return f.name }
func (f *fakeFleetNode) Status(context.Context) (daemon.StatusResponse, error) {
	return daemon.StatusResponse{}, nil
}
func (f *fakeFleetNode) Metrics(context.Context) (metrics.Stats, error) { return metrics.Stats{}, nil }
func (f *fakeFleetNode) Start(context.Context) (daemon.StatusResponse, error) {
	f.startCalls++
	return daemon.StatusResponse{State: "running"}, nil
}
func (f *fakeFleetNode) StartWith(context.Context, *remote.DeployConfig, string) (daemon.StatusResponse, error) {
	f.startWithCalls++
	return daemon.StatusResponse{State: "running"}, nil
}
func (f *fakeFleetNode) Stop(context.Context) (daemon.StatusResponse, error) {
	return daemon.StatusResponse{}, nil
}
func (f *fakeFleetNode) Logs(context.Context, int64, int) (daemon.LogsResponse, error) {
	return daemon.LogsResponse{}, nil
}

// A kind: remote node's start always uses a plain start, never StartWith —
// StartWith refuses a config for that kind unconditionally (see
// remoteNode.StartWith), so fleetStartCall must not even attempt it.
func TestFleetStartCallRemoteNodeUsesPlainStart(t *testing.T) {
	t.Setenv("SPINLOOP_CONFIG_DIR", t.TempDir())
	dir := writeFleetFile(t, "nodes:\n  - name: gpu-env\n    kind: remote\n    file: ./gpu-env.Spinloop\n")
	if err := os.WriteFile(filepath.Join(dir, "gpu-env.Spinloop"), []byte("PROVIDER llamacpp\nMODEL org/m:Q4\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := fleet.Resolve(filepath.Join(dir, "fleet.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	node := &fakeFleetNode{name: "gpu-env"}
	r := fleetStartCall(cfg)(context.Background(), node)
	if !r.OK() {
		t.Fatalf("fleetStartCall on a remote node = %+v", r)
	}
	if node.startWithCalls != 0 {
		t.Errorf("StartWith was called %d times for a remote node, want 0", node.startWithCalls)
	}
	if node.startCalls != 1 {
		t.Errorf("Start was called %d times, want 1", node.startCalls)
	}
}

// A kind: daemon node with a resolvable source is started via StartWith,
// never a plain Start — the whole point of the resolved config.
func TestFleetStartCallDaemonNodeUsesStartWith(t *testing.T) {
	t.Setenv("SPINLOOP_CONFIG_DIR", t.TempDir())
	dir := writeFleetFile(t, "nodes:\n  - name: dev-1\n    host: dev1.local\n    file: ./dev-1.Spinloop\n")
	if err := os.WriteFile(filepath.Join(dir, "dev-1.Spinloop"), []byte("PROVIDER llamacpp\nMODEL org/m:Q4\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := fleet.Resolve(filepath.Join(dir, "fleet.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	node := &fakeFleetNode{name: "dev-1"}
	r := fleetStartCall(cfg)(context.Background(), node)
	if !r.OK() {
		t.Fatalf("fleetStartCall on a daemon node = %+v", r)
	}
	if node.startCalls != 0 {
		t.Errorf("Start was called %d times for a daemon node with a resolved source, want 0", node.startCalls)
	}
	if node.startWithCalls != 1 {
		t.Errorf("StartWith was called %d times, want 1", node.startWithCalls)
	}
}

func TestCmdFleetUnknownNodeNamesTheKnownOnes(t *testing.T) {
	twoNodeFleet(t, "idle")
	err := cmdFleet([]string{"stop", "nope"})
	if err == nil {
		t.Fatal("unknown node accepted")
	}
	if !strings.Contains(err.Error(), "nope") || !strings.Contains(err.Error(), "up") {
		t.Errorf("error %q should name the unknown node and the known ones", err)
	}
}

func TestCmdFleetMissingFileNamesThePath(t *testing.T) {
	t.Chdir(t.TempDir())
	err := cmdFleet([]string{"status"})
	if err == nil {
		t.Fatal("missing fleet file accepted")
	}
	if !strings.Contains(err.Error(), "fleet.yaml") {
		t.Errorf("error %q does not name the expected path", err)
	}
}

func TestCmdFleetExplicitPath(t *testing.T) {
	up := stubNode(t, "running")
	host, port := hostPort(t, up)
	dir := t.TempDir()
	path := filepath.Join(dir, "cluster.yaml")
	body := fmt.Sprintf("nodes:\n  - name: solo\n    host: %s\n    port: %d\n", host, port)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	// Somewhere else entirely, so only --fleet can find it.
	t.Chdir(t.TempDir())
	out := captureStdout(t, func() {
		if err := cmdFleet([]string{"status", "--fleet", path}); err != nil {
			t.Error(err)
		}
	})
	if !strings.Contains(out, "solo") {
		t.Errorf("explicit --fleet not used:\n%s", out)
	}
}

func TestCmdFleetUnknownSubcommand(t *testing.T) {
	if err := cmdFleet([]string{"wat"}); err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("unknown subcommand should error, got %v", err)
	}
	// Bare: no error — cobra shows the group's own help, with the
	// subcommand list generated from the tree.
	out := captureStdout(t, func() {
		if err := cmdFleet([]string{}); err != nil {
			t.Fatalf("bare fleet should show its help, got %v", err)
		}
	})
	if !strings.Contains(out, "Available Commands") {
		t.Errorf("bare fleet should list its subcommands:\n%s", out)
	}
}

// Watch mode redraws until interrupted, then exits cleanly.
func TestCmdFleetMetricsWatchExitsOnInterrupt(t *testing.T) {
	twoNodeFleet(t, "running")

	// Keep the loop tight so the test does not wait a minute for a refresh.
	orig := metricsWatchInterval
	metricsWatchInterval = 50 * time.Millisecond
	t.Cleanup(func() { metricsWatchInterval = orig })

	done := make(chan error, 1)
	go func() { done <- cmdFleet([]string{"metrics", "--watch"}) }()

	// Let it draw at least twice, so the clear-and-redraw path runs.
	time.Sleep(200 * time.Millisecond)
	if err := syscall.Kill(os.Getpid(), syscall.SIGINT); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("watch exited with %v, want a clean exit", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("watch did not exit on SIGINT")
	}
}

// The daemon tracks when its engine last did work; the fleet view is where
// "which node is doing nothing?" gets asked, so the row shows it.
func TestCmdFleetStatusShowsIdleTime(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"state": "running", "runner": "llamacpp", "model": "org/qwen",
			"uptimeSeconds": 600,
			"lastActiveAt":  "2026-08-10T10:00:00Z",
			"idleSeconds":   125,
		})
	}))
	defer srv.Close()
	host, port := hostPort(t, srv)
	writeFleetFile(t, fmt.Sprintf("nodes:\n  - name: busy\n    host: %s\n    port: %d\n", host, port))

	out := captureStdout(t, func() {
		if err := cmdFleet([]string{"status"}); err != nil {
			t.Error(err)
		}
	})
	// 125s formats the same way the uptime column does, so the two read alike.
	if !strings.Contains(out, "last active 2m 5s ago") {
		t.Errorf("last-active time missing or misformatted:\n%s", out)
	}
}

// A daemon that has recorded no activity yet must not be reported as idle
// since boot — there is no last-active time to measure from.
func TestCmdFleetStatusOmitsIdleWithoutActivity(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"state": "running", "runner": "llamacpp", "model": "org/qwen",
			"uptimeSeconds": 600,
		})
	}))
	defer srv.Close()
	host, port := hostPort(t, srv)
	writeFleetFile(t, fmt.Sprintf("nodes:\n  - name: fresh\n    host: %s\n    port: %d\n", host, port))

	out := captureStdout(t, func() {
		if err := cmdFleet([]string{"status"}); err != nil {
			t.Error(err)
		}
	})
	if strings.Contains(out, "last active") {
		t.Errorf("activity claimed when the daemon recorded none:\n%s", out)
	}
	// The rest of the row is unaffected.
	if !strings.Contains(out, "up 10m 0s") {
		t.Errorf("uptime missing:\n%s", out)
	}
}
