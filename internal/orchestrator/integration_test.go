package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spinloop-ai/spinloop/internal/daemon"
	"github.com/spinloop-ai/spinloop/internal/fleet"
	"github.com/spinloop-ai/spinloop/internal/gateway"
	"github.com/spinloop-ai/spinloop/internal/remote"
)

// The integration suite stands the real gateway up on loopback against fake
// daemons and runs the real loop against it, launching real stub agents: the
// units in between — the topology reading, the admission, the dispatch, the
// state — are the production ones.

const integrationToken = "the-gateway-token"

// fakeNode is one daemon for the fleet file to name: an httptest server
// answering the daemon's status call, the reading it reports being mutable
// for a test that changes the fleet mid-run, and a down switch that makes it
// stop answering at all.
type fakeNode struct {
	mu     sync.Mutex
	down   bool
	state  string
	model  string
	served string
	srv    *httptest.Server
}

func newFakeNode(t *testing.T, state, model string) *fakeNode {
	t.Helper()
	n := &fakeNode{state: state, model: model}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/status", func(w http.ResponseWriter, r *http.Request) {
		n.mu.Lock()
		defer n.mu.Unlock()
		if n.down {
			http.Error(w, "connection refused", http.StatusInternalServerError)
			return
		}
		resp := map[string]any{"state": n.state, "model": n.model}
		if n.served != "" {
			resp["servedName"] = n.served
		}
		if n.state == string(daemon.StateRunning) {
			resp["ready"] = daemon.ReadyYes
			resp["lastActiveAt"] = "2026-01-01T00:00:00Z"
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	n.srv = httptest.NewServer(mux)
	t.Cleanup(n.srv.Close)
	return n
}

func (n *fakeNode) set(state, model string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.state, n.model = state, model
}

func (n *fakeNode) setDown(down bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.down = down
}

// addrOf is a server's host:port, the way the fleet file names it.
func addrOf(srv *httptest.Server) string {
	return srv.URL[strings.LastIndex(srv.URL, "//")+2:]
}

// nodeEntry is one node's lines for a fleet file.
func nodeEntry(name, addr string, tags map[string]string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "  - name: %s\n    host: %s\n    port: %s\n", name, hostOf(addr), portOf(addr))
	if len(tags) > 0 {
		b.WriteString("    tags:\n")
		for k, v := range tags {
			fmt.Fprintf(&b, "      %s: %s\n", k, v)
		}
	}
	return b.String()
}

// startLiveGateway writes the fleet file, loads it, and serves the real
// gateway against it.
func startLiveGateway(t *testing.T, fleetFile string, cfgFor fleet.ConfigFor) *httptest.Server {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fleet.yaml")
	if err := os.WriteFile(path, []byte(fleetFile), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := fleet.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(gateway.New(cfg, integrationToken, gateway.Options{ConfigFor: cfgFor}))
	t.Cleanup(srv.Close)
	return srv
}

// argsAgent is a stub agent that records what it was launched with in its
// working directory, then sleeps for the time the test sets.
func argsAgent(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "agent.sh")
	src := "#!/bin/sh\necho \"$@\" > args.txt\nsleep \"${AGENT_SLEEP:-0.05}\"\n"
	if err := os.WriteFile(p, []byte(src), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

// liveConfig is a Run config wired to a real gateway and a real stub agent.
func liveConfig(t *testing.T, gw *httptest.Server, itemsPath, agent string) Config {
	t.Helper()
	return Config{
		Gateway:    gw.URL,
		ItemsPath:  itemsPath,
		Topologist: NewGatewayTopologist(gw.URL, integrationToken),
		Dispatcher: NewDispatcher(&fakeHarness{name: "opencode", bin: agent}, gw.URL, integrationToken),
		Tick:       10 * time.Millisecond,
	}
}

// waitFor polls the state until the check passes or the test's patience runs
// out.
func waitFor(t *testing.T, itemsPath string, check func(stateFile) bool) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if check(readState(t, itemsPath)) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("the state never reached the wanted shape:\n%v", readState(t, itemsPath))
}

// TestIntegration_TheFleetsLimitsHoldEndToEnd is 5.1: the loop, the gateway,
// the daemons and the agents are all real, and the fleet's declared
// concurrency is what bounds the in-flight.
func TestIntegration_TheFleetsLimitsHoldEndToEnd(t *testing.T) {
	agent := argsAgent(t)
	t.Setenv("AGENT_SLEEP", "0.3")

	for _, tc := range []struct {
		name        string
		concurrency string
		tags        []string
		limit       int
	}{
		{name: "the total holds", concurrency: "concurrency:\n  total: 2\n", limit: 2},
		{name: "a tag holds its own items", concurrency: "concurrency:\n  total: 5\n  tags:\n    \"gpu=a100\": 1\n", tags: []string{"gpu=a100"}, limit: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			n := newFakeNode(t, string(daemon.StateRunning), "org/m")
			fleetFile := "nodes:\n" + nodeEntry("n", addrOf(n.srv), map[string]string{"gpu": "a100"}) + tc.concurrency
			gw := startLiveGateway(t, fleetFile, nil)

			var extra []string
			for _, tag := range tc.tags {
				extra = append(extra, "    - "+tag)
			}
			if len(extra) > 0 {
				extra = append([]string{"  tags:"}, extra...)
			}
			items := itemsFile(
				itemSpec{id: "a", dir: t.TempDir(), extra: extra},
				itemSpec{id: "b", dir: t.TempDir(), extra: extra},
				itemSpec{id: "c", dir: t.TempDir(), extra: extra},
			)
			itemsPath := writeItems(t, items)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- Run(ctx, liveConfig(t, gw, itemsPath, agent)) }()

			waitFor(t, itemsPath, func(sf stateFile) bool {
				count := 0
				for _, st := range sf.Items {
					if st.State == StateDone {
						count++
					}
				}
				return count == 3
			})
			cancel()
			if err := <-done; err != nil {
				t.Fatalf("the clean interrupt should end the run without an error, got %v", err)
			}
			sf := readState(t, itemsPath)
			for id, st := range sf.Items {
				if st.State != StateDone {
					t.Errorf("item %s = %s, want done", id, st.State)
				}
			}
		})
	}
}

// TestIntegration_ItemsTakeTheNodeTheFleetShapes is 5.2: the same item,
// different fleet shapes, the node the item lands on changing the way the
// fleet's own routing says it should.
func TestIntegration_ItemsTakeTheNodeTheFleetShapes(t *testing.T) {
	t.Setenv("AGENT_SLEEP", "0.05")

	t.Run("an item only takes a node that carries all of its tags", func(t *testing.T) {
		t.Parallel()
		agent := argsAgent(t)
		gpu := newFakeNode(t, string(daemon.StateRunning), "org/gpu-model")
		ram := newFakeNode(t, string(daemon.StateRunning), "org/ram-model")
		both := newFakeNode(t, string(daemon.StateRunning), "org/both-model")
		fleetFile := "nodes:\n" +
			nodeEntry("gpu", addrOf(gpu.srv), map[string]string{"gpu": "a100"}) +
			nodeEntry("ram", addrOf(ram.srv), map[string]string{"ram": "64"}) +
			nodeEntry("both", addrOf(both.srv), map[string]string{"gpu": "a100", "ram": "64"})
		gw := startLiveGateway(t, fleetFile, nil)

		workdir := t.TempDir()
		itemsPath := writeItems(t, itemsFile(itemSpec{
			id: "a", dir: workdir,
			extra: []string{"  tags:", "    - gpu=a100", "    - ram=64"},
		}))

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan error, 1)
		go func() { done <- Run(ctx, liveConfig(t, gw, itemsPath, agent)) }()
		waitFor(t, itemsPath, func(sf stateFile) bool { return sf.Items["a"].State == StateDone })
		cancel()
		if err := <-done; err != nil {
			t.Fatalf("the clean interrupt should end the run without an error, got %v", err)
		}

		args, err := os.ReadFile(filepath.Join(workdir, "args.txt"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(args), "spinloop-orchestrator-both") || !strings.Contains(string(args), "org/both-model") {
			t.Errorf("the item should have run against the node carrying both tags, got: %s", args)
		}
	})

	t.Run("a running node is preferred over a stopped one", func(t *testing.T) {
		t.Parallel()
		agent := argsAgent(t)
		run := newFakeNode(t, string(daemon.StateRunning), "org/run-model")
		stop := newFakeNode(t, string(daemon.StateStopped), "")
		fleetFile := "nodes:\n" +
			nodeEntry("run", addrOf(run.srv), map[string]string{"gpu": "a100"}) +
			nodeEntry("stop", addrOf(stop.srv), map[string]string{"gpu": "a100"})
		gw := startLiveGateway(t, fleetFile, nil)

		workdir := t.TempDir()
		itemsPath := writeItems(t, itemsFile(itemSpec{
			id: "a", dir: workdir, extra: []string{"  tags:", "    - gpu=a100"},
		}))

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan error, 1)
		go func() { done <- Run(ctx, liveConfig(t, gw, itemsPath, agent)) }()
		waitFor(t, itemsPath, func(sf stateFile) bool { return sf.Items["a"].State == StateDone })
		cancel()
		if err := <-done; err != nil {
			t.Fatalf("the clean interrupt should end the run without an error, got %v", err)
		}

		args, err := os.ReadFile(filepath.Join(workdir, "args.txt"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(args), "spinloop-orchestrator-run") || !strings.Contains(string(args), "org/run-model") {
			t.Errorf("the item should have run against the running node, got: %s", args)
		}
	})

	t.Run("a stopped node is used only when the fleet wakes", func(t *testing.T) {
		t.Parallel()
		stop := newFakeNode(t, string(daemon.StateStopped), "")
		fleetBase := "nodes:\n" + nodeEntry("stop", addrOf(stop.srv), map[string]string{"gpu": "a100"})
		cfgFor := fleet.ConstantConfig(remote.DeployConfig{ModelID: "org/wake-model"}, nil)

		t.Run("wake on: the item runs against the wakeable model", func(t *testing.T) {
			t.Parallel()
			agent := argsAgent(t)
			gw := startLiveGateway(t, fleetBase+"\nwake: on\n", cfgFor)

			workdir := t.TempDir()
			itemsPath := writeItems(t, itemsFile(itemSpec{
				id: "a", dir: workdir, extra: []string{"  tags:", "    - gpu=a100"},
			}))

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- Run(ctx, liveConfig(t, gw, itemsPath, agent)) }()
			waitFor(t, itemsPath, func(sf stateFile) bool { return sf.Items["a"].State == StateDone })
			cancel()
			if err := <-done; err != nil {
				t.Fatalf("the clean interrupt should end the run without an error, got %v", err)
			}

			args, err := os.ReadFile(filepath.Join(workdir, "args.txt"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(args), "org/wake-model") {
				t.Errorf("the item should have run against the wakeable model, got: %s", args)
			}
		})

		t.Run("wake off: the item waits", func(t *testing.T) {
			t.Parallel()
			agent := argsAgent(t)
			gw := startLiveGateway(t, fleetBase+"\nwake: off\n", cfgFor)

			workdir := t.TempDir()
			itemsPath := writeItems(t, itemsFile(itemSpec{
				id: "a", dir: workdir, extra: []string{"  tags:", "    - gpu=a100"},
			}))

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- Run(ctx, liveConfig(t, gw, itemsPath, agent)) }()

			time.Sleep(300 * time.Millisecond)
			if st := readState(t, itemsPath).Items["a"]; st.State != "" {
				t.Fatalf("with the fleet's wake off the item should wait, got %s", st.State)
			}
			if _, err := os.ReadFile(filepath.Join(workdir, "args.txt")); !os.IsNotExist(err) {
				t.Errorf("with the fleet's wake off the item should not be launched")
			}
			cancel()
			if err := <-done; err != nil {
				t.Fatalf("the clean interrupt should end the run without an error, got %v", err)
			}
		})
	})

	t.Run("an item waits for a node that appears in the topology", func(t *testing.T) {
		t.Parallel()
		agent := argsAgent(t)
		late := newFakeNode(t, string(daemon.StateRunning), "org/late-model")
		late.setDown(true)
		gw := startLiveGateway(t, "nodes:\n"+nodeEntry("late", addrOf(late.srv), map[string]string{"gpu": "a100"}), nil)

		workdir := t.TempDir()
		itemsPath := writeItems(t, itemsFile(itemSpec{
			id: "a", dir: workdir, extra: []string{"  tags:", "    - gpu=a100"},
		}))

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan error, 1)
		go func() { done <- Run(ctx, liveConfig(t, gw, itemsPath, agent)) }()

		// The node does not answer: the item waits, no launch.
		time.Sleep(300 * time.Millisecond)
		if st := readState(t, itemsPath).Items["a"]; st.State != "" {
			t.Fatalf("a node that does not answer should hold the item, got %s", st.State)
		}

		late.setDown(false)
		waitFor(t, itemsPath, func(sf stateFile) bool { return sf.Items["a"].State == StateDone })
		cancel()
		if err := <-done; err != nil {
			t.Fatalf("the clean interrupt should end the run without an error, got %v", err)
		}
		args, err := os.ReadFile(filepath.Join(workdir, "args.txt"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(args), "org/late-model") {
			t.Errorf("the item should have run against the node once it appeared, got: %s", args)
		}
	})
}

// TestIntegration_TheLifecycleEndsTheSpecSays is 5.3: the run's end — a
// crash, a clean interrupt, a bad working directory — ends the way the spec
// says, with the loop restarted in the test to prove the record carries over.
func TestIntegration_TheLifecycleEndsTheSpecSays(t *testing.T) {
	t.Run("a crash between starts records a running item as failed without re-running it", func(t *testing.T) {
		agent := argsAgent(t)
		n := newFakeNode(t, string(daemon.StateRunning), "org/m")
		gw := startLiveGateway(t, "nodes:\n"+nodeEntry("n", addrOf(n.srv), map[string]string{"gpu": "a100"})+
			"concurrency:\n  total: 1\n", nil)

		workdir := t.TempDir()
		itemsPath := writeItems(t, itemsFile(itemSpec{
			id: "a", dir: workdir, extra: []string{"  tags:", "    - gpu=a100"},
		}))

		// The first run: the agent takes its time, then the gateway stops
		// answering while the agent is still in flight — the run ends in an
		// error, the way a crashed one would, and the record stays running.
		t.Setenv("AGENT_SLEEP", "1")
		ctx1, cancel1 := context.WithCancel(context.Background())
		defer cancel1()
		errCh := make(chan error, 1)
		go func() { errCh <- Run(ctx1, liveConfig(t, gw, itemsPath, agent)) }()
		waitFor(t, itemsPath, func(sf stateFile) bool { return sf.Items["a"].State == StateRunning })
		gw.Close()
		err := <-errCh
		if err == nil {
			t.Fatal("a gateway that stops answering should end the run in an error")
		}
		if st := readState(t, itemsPath).Items["a"]; st.State != StateRunning {
			t.Fatalf("the ended run should leave its in-flight item recorded running, got %s", st.State)
		}
		// The agent ends on its own; nobody records it, the way a crash does.
		time.Sleep(1200 * time.Millisecond)

		// The second run: a fresh gateway, the same file. The recovery
		// records the item failed, and the loop does not run it again.
		gw2 := startLiveGateway(t, "nodes:\n"+nodeEntry("n", addrOf(n.srv), map[string]string{"gpu": "a100"})+
			"concurrency:\n  total: 1\n", nil)
		ctx2, cancel2 := context.WithCancel(context.Background())
		defer cancel2()
		done := make(chan error, 1)
		go func() { done <- Run(ctx2, liveConfig(t, gw2, itemsPath, agent)) }()

		// The restarted loop does not re-run it: the record stays failed.
		time.Sleep(300 * time.Millisecond)
		if st := readState(t, itemsPath).Items["a"]; st.State != StateFailed {
			t.Fatalf("the restart should record the crashed item failed and not re-run it, got %s (%s)", st.State, st.Why)
		}
		cancel2()
		if err := <-done; err != nil {
			t.Fatalf("the clean interrupt should end the run without an error, got %v", err)
		}
	})

	t.Run("a clean interrupt re-queues and the next start picks the item up", func(t *testing.T) {
		agent := argsAgent(t)
		n := newFakeNode(t, string(daemon.StateRunning), "org/m")
		gw := startLiveGateway(t, "nodes:\n"+nodeEntry("n", addrOf(n.srv), map[string]string{"gpu": "a100"})+
			"concurrency:\n  total: 1\n", nil)

		workdir := t.TempDir()
		itemsPath := writeItems(t, itemsFile(itemSpec{
			id: "a", dir: workdir, extra: []string{"  tags:", "    - gpu=a100"},
		}))

		// The first run: the agent is long, the interrupt ends the run.
		t.Setenv("AGENT_SLEEP", "30")
		ctx1, cancel1 := context.WithCancel(context.Background())
		errCh := make(chan error, 1)
		go func() { errCh <- Run(ctx1, liveConfig(t, gw, itemsPath, agent)) }()
		waitFor(t, itemsPath, func(sf stateFile) bool { return sf.Items["a"].State == StateRunning })
		cancel1()
		if err := <-errCh; err != nil {
			t.Fatalf("the clean interrupt should end the run without an error, got %v", err)
		}
		if sf := readState(t, itemsPath); len(sf.Items) != 0 {
			t.Fatalf("the interrupt should put the in-flight item back in the backlog, got %v", sf.Items)
		}

		// The second run: the same item runs to the end.
		t.Setenv("AGENT_SLEEP", "0.05")
		ctx2, cancel2 := context.WithCancel(context.Background())
		defer cancel2()
		done := make(chan error, 1)
		go func() { done <- Run(ctx2, liveConfig(t, gw, itemsPath, agent)) }()
		waitFor(t, itemsPath, func(sf stateFile) bool { return sf.Items["a"].State == StateDone })
		cancel2()
		if err := <-done; err != nil {
			t.Fatalf("the clean interrupt should end the run without an error, got %v", err)
		}
		if _, err := os.ReadFile(filepath.Join(workdir, "args.txt")); err != nil {
			t.Errorf("the restarted run should have launched the item, got %v", err)
		}
	})

	t.Run("a missing working directory fails one item while others run", func(t *testing.T) {
		agent := argsAgent(t)
		t.Setenv("AGENT_SLEEP", "0.05")
		n := newFakeNode(t, string(daemon.StateRunning), "org/m")
		gw := startLiveGateway(t, "nodes:\n"+nodeEntry("n", addrOf(n.srv), map[string]string{"gpu": "a100"})+
			"concurrency:\n  total: 2\n", nil)

		good := t.TempDir()
		bad := filepath.Join(t.TempDir(), "nowhere")
		itemsPath := writeItems(t, itemsFile(
			itemSpec{id: "bad", dir: bad},
			itemSpec{id: "good", dir: good},
		))

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan error, 1)
		go func() { done <- Run(ctx, liveConfig(t, gw, itemsPath, agent)) }()
		waitFor(t, itemsPath, func(sf stateFile) bool {
			return sf.Items["bad"].State == StateFailed && sf.Items["good"].State == StateDone
		})
		cancel()
		if err := <-done; err != nil {
			t.Fatalf("the clean interrupt should end the run without an error, got %v", err)
		}
		if st := readState(t, itemsPath).Items["bad"]; !strings.Contains(st.Why, "does not exist") {
			t.Errorf("the failed item should name the missing directory, got %q", st.Why)
		}
	})
}

func hostOf(addr string) string {
	host, _, _ := strings.Cut(addr, ":")
	return host
}

func portOf(addr string) string {
	_, port, _ := strings.Cut(addr, ":")
	return port
}
