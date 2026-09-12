package main

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/spinloop-ai/spinloop/internal/daemon"
	"github.com/spinloop-ai/spinloop/internal/harness"
	"github.com/spinloop-ai/spinloop/internal/spinloop"
)

// routableNode is a daemon answering status for one machine, with a listener
// standing in for its engine so a routed launch has something that answers.
type routableNode struct {
	srv        *httptest.Server
	enginePort int
	engineLn   net.Listener
	started    bool
}

// newRoutableNode serves `model` when running is true, with a live engine.
func newRoutableNode(t *testing.T, model string, running bool, idleSeconds int) *routableNode {
	t.Helper()
	n := &routableNode{}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	n.engineLn = ln
	n.enginePort = ln.Addr().(*net.TCPAddr).Port
	t.Cleanup(func() { ln.Close() })

	state := string(daemon.StateIdle)
	if running {
		state = string(daemon.StateRunning)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/status", func(w http.ResponseWriter, r *http.Request) {
		resp := daemon.StatusResponse{State: state, Model: model}
		if state == string(daemon.StateRunning) {
			resp.Engine = &daemon.EngineEndpoint{Port: n.enginePort}
			resp.LastActiveAt = "2026-08-12T10:00:00Z"
			resp.IdleSeconds = idleSeconds
		}
		json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("/v1/start", func(w http.ResponseWriter, r *http.Request) {
		n.started = true
		state = string(daemon.StateRunning)
		json.NewEncoder(w).Encode(daemon.StatusResponse{State: state, Model: model})
	})
	n.srv = httptest.NewServer(mux)
	t.Cleanup(n.srv.Close)
	return n
}

// entry renders this node as a fleet.yaml entry.
func (n *routableNode) entry(name string) string {
	host, port, _ := net.SplitHostPort(strings.TrimPrefix(n.srv.URL, "http://"))
	return "  - name: " + name + "\n    host: " + host + "\n    port: " + port + "\n"
}

// fleetFileIn puts a fleet.yaml in dir and returns its path. (fleet_test.go's
// writeFleetFile chdirs into a temp dir, which routing tests do not want.)
func fleetFileIn(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, "fleet.yaml")
	mustWrite(t, path, body)
	return path
}

// routedSpinloop writes a plain Spinloop and returns its directory. The fleet
// it routes through is a launch concern, not a Spinloop field: the tests name
// it with the --fleet flag or a routeOptions fleetPath.
func routedSpinloop(t *testing.T, model string) string {
	t.Helper()
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "Spinloop"), "PROVIDER llamacpp\nMODEL "+model+"\n")
	return dir
}

func TestRouteChoosesARunningNode(t *testing.T) {
	node := newRoutableNode(t, "qwen3-27b", true, 300)
	dir := t.TempDir()
	fleetPath := fleetFileIn(t, dir, "nodes:\n"+node.entry("gpu-box"))
	spinloopDir := routedSpinloop(t, "qwen3-27b")

	sel, path, err := readSpinloop("test", spinloopDir)
	if err != nil {
		t.Fatal(err)
	}
	stderr := captureStderr(t, func() {
		c, err := routeThroughFleet(sel, path, routeOptions{fleetPath: fleetPath})
		if err != nil {
			t.Fatalf("routing failed: %v", err)
		}
		if c == nil {
			t.Fatal("a launch naming a fleet should route")
		}
		want := "http://127.0.0.1:" + strconv.Itoa(node.enginePort) + "/v1"
		if c.BaseURL != want {
			t.Errorf("base URL = %q, want %q", c.BaseURL, want)
		}
		if c.Node.Name != "gpu-box" {
			t.Errorf("chose %q", c.Node.Name)
		}
	})
	// The choice is announced before anything launches.
	for _, want := range []string{"gpu-box", "prefer idle"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("the route should be announced with %q, got:\n%s", want, stderr)
		}
	}
}

// No fleet by flag and no fleet.yaml in the working directory: the launch
// routes nowhere and contacts nothing.
func TestNoFleetDoesNotRoute(t *testing.T) {
	spinloopDir := routedSpinloop(t, "qwen3-27b")
	sel, path, err := readSpinloop("test", spinloopDir)
	if err != nil {
		t.Fatal(err)
	}
	// An empty working directory holds no fleet.yaml, so an unnamed Spinloop
	// has no fleet to find.
	t.Chdir(t.TempDir())
	choice, err := routeThroughFleet(sel, path, routeOptions{})
	if err != nil || choice != nil {
		t.Errorf("choice = %+v, err = %v; want no routing at all", choice, err)
	}
}

// The --fleet flag beats the fleet.yaml in the working directory: when both
// are present the flag's file is the one read.
func TestFleetFlagBeatsTheCwdFleet(t *testing.T) {
	flagNode := newRoutableNode(t, "qwen3-27b", true, 10)
	cwdNode := newRoutableNode(t, "qwen3-27b", true, 10)
	flagDir := t.TempDir()
	flagFleet := fleetFileIn(t, flagDir, "nodes:\n"+flagNode.entry("from-flag"))
	cwdDir := t.TempDir()
	fleetFileIn(t, cwdDir, "nodes:\n"+cwdNode.entry("from-cwd"))
	t.Chdir(cwdDir)

	spinloopDir := routedSpinloop(t, "qwen3-27b")
	sel, path, err := readSpinloop("test", spinloopDir)
	if err != nil {
		t.Fatal(err)
	}
	captureStderr(t, func() {
		c, err := routeThroughFleet(sel, path, routeOptions{fleetPath: flagFleet})
		if err != nil {
			t.Fatalf("routing failed: %v", err)
		}
		if c.Node.Name != "from-flag" {
			t.Errorf("chose %q, want the flag's fleet", c.Node.Name)
		}
	})
}

// The short form is the flag: -f names the fleet a launch routes through.
func TestHarnessFleetFlagShortForm(t *testing.T) {
	isolateConfig(t)
	node := newRoutableNode(t, "qwen3-27b", true, 10)
	dir := t.TempDir()
	flagFleet := fleetFileIn(t, dir, "nodes:\n"+node.entry("from-flag"))
	spinloopDir := routedSpinloop(t, "qwen3-27b")

	argsFile := filepath.Join(t.TempDir(), "args")
	stubHarnessBinary(t, "opencode", argsFile)
	stderr := captureStderr(t, func() {
		captureStdout(t, func() {
			if err := cmdHarness([]string{"--spinloop=" + spinloopDir, "-f", flagFleet, "--", "run"}); err != nil {
				t.Fatalf("cmdHarness -f: %v", err)
			}
		})
	})
	if _, err := os.ReadFile(argsFile); err != nil {
		t.Fatalf("harness was not launched: %v", err)
	}
	if !strings.Contains(stderr, "from-flag") {
		t.Errorf("the route announcement should name the flag's node, got:\n%s", stderr)
	}
}

// A pinned BASEURL is the explicit answer, so nothing is selected even though a
// fleet is in force.
func TestPinnedBaseURLSkipsRouting(t *testing.T) {
	node := newRoutableNode(t, "qwen3-27b", true, 10)
	dir := t.TempDir()
	fleetPath := fleetFileIn(t, dir, "nodes:\n"+node.entry("gpu-box"))
	spinloopDir := t.TempDir()
	mustWrite(t, filepath.Join(spinloopDir, "Spinloop"),
		"PROVIDER llamacpp\nMODEL qwen3-27b\nBASEURL http://pinned:9999/v1\n")

	sel, path, err := readSpinloop("test", spinloopDir)
	if err != nil {
		t.Fatal(err)
	}
	stderr := captureStderr(t, func() {
		c, err := routeThroughFleet(sel, path, routeOptions{fleetPath: fleetPath})
		if err != nil {
			t.Fatal(err)
		}
		if c != nil {
			t.Errorf("a pinned BASEURL should not be routed, got %+v", c)
		}
	})
	if !strings.Contains(stderr, "Not routing") || !strings.Contains(stderr, "pinned") {
		t.Errorf("spinloop should say it is not routing, got:\n%s", stderr)
	}
}

// --no-wake refuses to start anything and shows the fleet.
func TestNoWakeFailsWithTheNodeTable(t *testing.T) {
	node := newRoutableNode(t, "", false, 0)
	dir := t.TempDir()
	fleetPath := fleetFileIn(t, dir, "nodes:\n"+node.entry("idle-box"))
	spinloopDir := routedSpinloop(t, "qwen3-27b")

	sel, path, err := readSpinloop("test", spinloopDir)
	if err != nil {
		t.Fatal(err)
	}
	captureStderr(t, func() {
		_, err := routeThroughFleet(sel, path, routeOptions{fleetPath: fleetPath, noWake: true})
		if err == nil {
			t.Fatal("--no-wake with nothing serving should fail")
		}
		for _, want := range []string{"idle-box", "qwen3-27b", "spinloop fleet start"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("message should mention %q, got:\n%s", want, err)
			}
		}
	})
	if node.started {
		t.Error("--no-wake started an engine")
	}
}

// routedSpinloopWith writes a Spinloop carrying extra instructions, for the
// cases where what the Spinloop says about the *engine* decides whether a wake
// can happen at all.
func routedSpinloopWith(t *testing.T, model, extra string) string {
	t.Helper()
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "Spinloop"), "PROVIDER llamacpp\nMODEL "+model+"\n"+extra)
	return dir
}

// A wake turns the Spinloop into something to start, so an unusable PARALLEL
// stops it there — with the reason, rather than launching an engine with a
// slot count nobody could parse.
func TestWakeRefusesAnUnusableParallel(t *testing.T) {
	node := newRoutableNode(t, "", false, 0)
	dir := t.TempDir()
	fleetPath := fleetFileIn(t, dir, "nodes:\n"+node.entry("idle-box"))
	spinloopDir := routedSpinloopWith(t, "qwen3-27b", "PARALLEL 0\n")

	sel, path, err := readSpinloop("test", spinloopDir)
	if err != nil {
		t.Fatal(err)
	}
	captureStderr(t, func() {
		_, err := routeThroughFleet(sel, path, routeOptions{fleetPath: fleetPath})
		if err == nil {
			t.Fatal("a wake with an unusable PARALLEL should fail")
		}
		if !strings.Contains(err.Error(), "cannot be turned into something to start") {
			t.Errorf("error should say the Spinloop cannot start anything, got:\n%v", err)
		}
		if !strings.Contains(err.Error(), "PARALLEL") {
			t.Errorf("error should name the offending instruction, got:\n%v", err)
		}
	})
	if node.started {
		t.Error("an engine was started from a Spinloop that could not be turned into a command")
	}
}

// The other side of the same seam: deriving the start config is deliberately
// not fatal while merely *choosing* a node, so a Spinloop that could not start
// an engine still routes to one already serving the model. Failing here would
// take a working launch away for the sake of a value it never needs.
func TestRoutingToARunningNodeToleratesAnUnusableParallel(t *testing.T) {
	node := newRoutableNode(t, "qwen3-27b", true, 300)
	dir := t.TempDir()
	fleetPath := fleetFileIn(t, dir, "nodes:\n"+node.entry("gpu-box"))
	spinloopDir := routedSpinloopWith(t, "qwen3-27b", "PARALLEL 0\n")

	sel, path, err := readSpinloop("test", spinloopDir)
	if err != nil {
		t.Fatal(err)
	}
	captureStderr(t, func() {
		c, err := routeThroughFleet(sel, path, routeOptions{fleetPath: fleetPath})
		if err != nil {
			t.Fatalf("a node already serving the model should still be chosen: %v", err)
		}
		if c == nil || c.Node.Name != "gpu-box" {
			t.Fatalf("expected to route to gpu-box, got %+v", c)
		}
	})
}

// gatewayFleetFile writes a fleet file naming a gateway whose nodes point at a
// port nothing listens on — so a route that consults them fails loudly rather
// than passing quietly.
func gatewayFleetFile(t *testing.T, section string) string {
	t.Helper()
	return fleetFileIn(t, t.TempDir(),
		"nodes:\n  - name: dead\n    host: 127.0.0.1\n    port: 1\n"+section)
}

// A fleet file naming a gateway routes at it: no node is consulted — the dead
// node below would fail a route that tried.
func TestRouteToFileNamingAGateway(t *testing.T) {
	fleetPath := gatewayFleetFile(t,
		"gateway:\n  url: http://gw.internal:4000\n  tokenEnv: GW_TOKEN\n")
	spinloopDir := routedSpinloop(t, "qwen3-27b")
	sel, path, err := readSpinloop("test", spinloopDir)
	if err != nil {
		t.Fatal(err)
	}
	stderr := captureStderr(t, func() {
		c, err := routeThroughFleet(sel, path, routeOptions{fleetPath: fleetPath, node: "nobody", prefer: "sideways", noWake: true})
		if err != nil {
			t.Fatalf("a file naming a gateway should not consult any node: %v", err)
		}
		if !c.Gateway {
			t.Fatalf("the choice should mark itself as a gateway, got %+v", c)
		}
		if c.BaseURL != "http://gw.internal:4000/v1" {
			t.Errorf("a section without a path gets the prefix, got %s", c.BaseURL)
		}
		if c.GatewayTokenEnv != "GW_TOKEN" {
			t.Errorf("the choice should carry the section's variable, got %q", c.GatewayTokenEnv)
		}
	})
	// The choice is reported before anything launches, the way a node choice is.
	if !strings.Contains(stderr, "Routing at http://gw.internal:4000/v1 — the fleet file names a gateway") {
		t.Errorf("the choice should be reported on stderr, got:\n%s", stderr)
	}
}

// A section url carrying a path is used as given.
func TestRouteToFileNamingAGatewayWithAPath(t *testing.T) {
	fleetPath := gatewayFleetFile(t,
		"gateway:\n  url: http://gw.internal:4000/proxy/v1\n")
	spinloopDir := routedSpinloop(t, "qwen3-27b")
	sel, path, err := readSpinloop("test", spinloopDir)
	if err != nil {
		t.Fatal(err)
	}
	captureStderr(t, func() {
		c, err := routeThroughFleet(sel, path, routeOptions{fleetPath: fleetPath})
		if err != nil {
			t.Fatal(err)
		}
		if !c.Gateway || c.BaseURL != "http://gw.internal:4000/proxy/v1" {
			t.Errorf("a section carrying a path is used as given, got %+v", c)
		}
	})
}

// A pinned BASEURL wins over a gateway section, as it wins over a node fleet.
func TestPinnedBaseURLBeatsAGatewaySection(t *testing.T) {
	fleetPath := gatewayFleetFile(t, "gateway:\n  url: http://gw.internal:4000\n")
	spinloopDir := t.TempDir()
	mustWrite(t, filepath.Join(spinloopDir, "Spinloop"),
		"PROVIDER llamacpp\nMODEL qwen3-27b\nBASEURL http://pinned:9999/v1\n")
	sel, path, err := readSpinloop("test", spinloopDir)
	if err != nil {
		t.Fatal(err)
	}
	stderr := captureStderr(t, func() {
		c, err := routeThroughFleet(sel, path, routeOptions{fleetPath: fleetPath})
		if err != nil {
			t.Fatal(err)
		}
		if c != nil {
			t.Errorf("a pinned BASEURL is not routed, got %+v", c)
		}
	})
	if !strings.Contains(stderr, "Not routing") {
		t.Errorf("spinloop should say it is not routing, got:\n%s", stderr)
	}
}

// stubHarnessBinaryWithEnv is stubHarnessBinary plus a dump of the two
// variables a routed launch injects, for asserting what the agent actually
// got.
func stubHarnessBinaryWithEnv(t *testing.T, argsFile, envFile string) {
	t.Helper()
	dir := t.TempDir()
	body := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" > " + argsFile + "\n" +
		"printf 'BASE=%s\\nKEY=%s\\n' \"$OPENAI_BASE_URL\" \"$OPENAI_API_KEY\" > " + envFile + "\n"
	if err := os.WriteFile(filepath.Join(dir, "opencode"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// A fleet file naming a gateway points the agent at it: the address is the
// applied provider's base URL, and the token is resolved under the variable
// the section names, not the default.
func TestLaunchWithAGatewaySectionPointsTheAgentAtTheGateway(t *testing.T) {
	home := isolateConfig(t)
	t.Setenv("GATEWAY_TOKEN", "gw-token")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENAI_BASE_URL", "")
	argsFile := filepath.Join(t.TempDir(), "args")
	envFile := filepath.Join(t.TempDir(), "env")
	stubHarnessBinaryWithEnv(t, argsFile, envFile)

	fleetPath := gatewayFleetFile(t,
		"gateway:\n  url: http://gw.internal:4000\n  tokenEnv: GATEWAY_TOKEN\n")
	spinloopDir := routedSpinloop(t, "qwen3-27b")
	captureStdout(t, func() {
		if err := cmdHarness([]string{"--spinloop=" + spinloopDir, "-f", fleetPath, "--", "run"}); err != nil {
			t.Fatalf("cmdHarness: %v", err)
		}
	})
	if _, err := os.ReadFile(argsFile); err != nil {
		t.Fatalf("harness was not launched: %v", err)
	}
	data, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatal(err)
	}
	out := string(data)
	if !strings.Contains(out, "BASE=http://gw.internal:4000/v1") {
		t.Errorf("the agent's base URL should be the section's address with the prefix, got:\n%s", out)
	}
	if !strings.Contains(out, "KEY=gw-token") {
		t.Errorf("the agent should carry the token the section's variable holds, got:\n%s", out)
	}
	config, err := os.ReadFile(filepath.Join(home, ".config", "opencode", "opencode.json"))
	if err != nil {
		t.Fatalf("the harness config was not written: %v", err)
	}
	if !strings.Contains(string(config), "http://gw.internal:4000/v1") {
		t.Errorf("the applied provider's base URL should be the section's address, got:\n%s", config)
	}
}

// A section naming no tokenEnv resolves under the default variable.
func TestLaunchWithAGatewaySectionDefaultsToOpenAIKey(t *testing.T) {
	isolateConfig(t)
	t.Setenv("OPENAI_API_KEY", "gw-token")
	t.Setenv("OPENAI_BASE_URL", "")
	argsFile := filepath.Join(t.TempDir(), "args")
	envFile := filepath.Join(t.TempDir(), "env")
	stubHarnessBinaryWithEnv(t, argsFile, envFile)

	fleetPath := gatewayFleetFile(t, "gateway:\n  url: http://gw.internal:4000\n")
	spinloopDir := routedSpinloop(t, "qwen3-27b")
	captureStdout(t, func() {
		if err := cmdHarness([]string{"--spinloop=" + spinloopDir, "-f", fleetPath, "--", "run"}); err != nil {
			t.Fatalf("cmdHarness: %v", err)
		}
	})
	data, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "KEY=gw-token") {
		t.Errorf("the agent should carry the token under the default variable, got:\n%s", data)
	}
}

// A launch at a gateway section whose variable is set nowhere fails before the
// agent launches and before the harness config is written, naming the variable
// the section names.
func TestLaunchWithAGatewaySectionFailsNamingItsVariable(t *testing.T) {
	home := isolateConfig(t)
	t.Setenv("GATEWAY_TOKEN", "")
	t.Setenv("OPENAI_API_KEY", "")
	argsFile := filepath.Join(t.TempDir(), "args")
	stubHarnessBinary(t, "opencode", argsFile)

	fleetPath := gatewayFleetFile(t,
		"gateway:\n  url: http://gw.internal:4000\n  tokenEnv: GATEWAY_TOKEN\n")
	spinloopDir := routedSpinloop(t, "qwen3-27b")
	captureStdout(t, func() {
		err := cmdHarness([]string{"--spinloop=" + spinloopDir, "-f", fleetPath, "--", "run"})
		if err == nil {
			t.Fatal("a launch that cannot authenticate the gateway should fail")
		}
		if !strings.Contains(err.Error(), "GATEWAY_TOKEN") {
			t.Errorf("the failure should name the variable the section names, got:\n%v", err)
		}
	})
	if _, err := os.ReadFile(argsFile); err == nil {
		t.Error("the agent launched without a token to reach the gateway")
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "opencode", "opencode.json")); err == nil {
		t.Error("the harness config was written for a launch that could not authenticate")
	}
}

// The section's token may sit in the `.env` beside the Spinloop, the way any
// key the launch resolves does: set nowhere in the environment, found beside
// the file.
func TestLaunchWithAGatewaySectionTokenFromDotEnv(t *testing.T) {
	isolateConfig(t)
	t.Setenv("GATEWAY_TOKEN", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENAI_BASE_URL", "")
	argsFile := filepath.Join(t.TempDir(), "args")
	envFile := filepath.Join(t.TempDir(), "env")
	stubHarnessBinaryWithEnv(t, argsFile, envFile)

	fleetPath := gatewayFleetFile(t,
		"gateway:\n  url: http://gw.internal:4000\n  tokenEnv: GATEWAY_TOKEN\n")
	spinloopDir := routedSpinloop(t, "qwen3-27b")
	mustWrite(t, filepath.Join(spinloopDir, ".env"), "GATEWAY_TOKEN=dotenv-token\n")
	captureStdout(t, func() {
		if err := cmdHarness([]string{"--spinloop=" + spinloopDir, "-f", fleetPath, "--", "run"}); err != nil {
			t.Fatalf("cmdHarness: %v", err)
		}
	})
	if _, err := os.ReadFile(argsFile); err != nil {
		t.Fatalf("harness was not launched: %v", err)
	}
	data, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "KEY=dotenv-token") {
		t.Errorf("the agent should carry the token the .env beside the Spinloop holds, got:\n%s", data)
	}
}

// A fleet that declares wake: off refuses to start anything when nothing is
// serving, and names the node that would have been woken with the command that
// would start it.
func TestWakeOffRefusesNamingTheNode(t *testing.T) {
	node := newRoutableNode(t, "", false, 0)
	dir := t.TempDir()
	fleetPath := fleetFileIn(t, dir, "wake: off\nnodes:\n"+node.entry("idle-box"))
	spinloopDir := routedSpinloop(t, "qwen3-27b")

	sel, path, err := readSpinloop("test", spinloopDir)
	if err != nil {
		t.Fatal(err)
	}
	captureStderr(t, func() {
		_, err := routeThroughFleet(sel, path, routeOptions{fleetPath: fleetPath})
		if err == nil {
			t.Fatal("wake: off with nothing serving should fail")
		}
		for _, want := range []string{"wake is off", "idle-box", "spinloop fleet start idle-box"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("message should mention %q, got:\n%s", want, err)
			}
		}
	})
	if node.started {
		t.Error("wake: off started an engine")
	}
}

func TestUnknownPreferenceIsRefused(t *testing.T) {
	node := newRoutableNode(t, "qwen3-27b", true, 10)
	dir := t.TempDir()
	fleetPath := fleetFileIn(t, dir, "nodes:\n"+node.entry("gpu-box"))
	spinloopDir := routedSpinloop(t, "qwen3-27b")

	sel, path, err := readSpinloop("test", spinloopDir)
	if err != nil {
		t.Fatal(err)
	}
	captureStderr(t, func() {
		if _, err := routeThroughFleet(sel, path, routeOptions{fleetPath: fleetPath, prefer: "sideways"}); err == nil {
			t.Fatal("an unknown preference should fail")
		} else if !strings.Contains(err.Error(), "idle") || !strings.Contains(err.Error(), "active") {
			t.Errorf("error should name both values, got: %v", err)
		}
	})
}

// A routed launch points the agent at the chosen node, and an explicit setting
// in the environment still wins.
func TestRoutedLaunchEnvironment(t *testing.T) {
	node := newRoutableNode(t, "qwen3-27b", true, 10)
	dir := t.TempDir()
	fleetPath := fleetFileIn(t, dir, "nodes:\n"+node.entry("gpu-box"))
	spinloopDir := routedSpinloop(t, "qwen3-27b")

	sel, path, err := readSpinloop("test", spinloopDir)
	if err != nil {
		t.Fatal(err)
	}
	var baseURL string
	captureStderr(t, func() {
		c, err := routeThroughFleet(sel, path, routeOptions{fleetPath: fleetPath})
		if err != nil {
			t.Fatal(err)
		}
		baseURL = c.BaseURL
	})

	t.Setenv("OPENAI_BASE_URL", "")
	env := setEnvIfBlank(os.Environ(), "OPENAI_BASE_URL", baseURL)
	if got, _ := envValue(env, "OPENAI_BASE_URL"); got != baseURL {
		t.Errorf("OPENAI_BASE_URL = %q, want the chosen node %q", got, baseURL)
	}

	// An exported value is a deliberate choice and is left alone.
	exported := append(os.Environ(), "OPENAI_BASE_URL=http://exported/v1")
	exported = setEnvIfBlank(exported, "OPENAI_BASE_URL", baseURL)
	if got, _ := envValue(exported, "OPENAI_BASE_URL"); got != "http://exported/v1" {
		t.Errorf("an exported base URL should win, got %q", got)
	}
}

// A failed route must leave the harness config exactly as it was.
func TestFailedRouteLeavesTheConfigUntouched(t *testing.T) {
	home := isolateConfig(t)
	node := newRoutableNode(t, "", false, 0)
	dir := t.TempDir()
	fleetPath := fleetFileIn(t, dir, "nodes:\n"+node.entry("idle-box"))
	spinloopDir := routedSpinloop(t, "qwen3-27b")

	h, _ := harness.Lookup("opencode")
	var err error
	captureStderr(t, func() {
		captureStdout(t, func() {
			_, _, _, _, err = applyBeforeLaunch(
				spinloopPathFlag{set: true, path: spinloopDir}, "", h, nil,
				routeOptions{noWake: true, fleetPath: fleetPath})
		})
	})
	if err == nil {
		t.Fatal("a launch with nothing to route to should fail")
	}
	if _, statErr := os.Stat(filepath.Join(home, ".config", "opencode", "opencode.json")); !os.IsNotExist(statErr) {
		t.Errorf("the config was written by a launch that then failed (stat: %v)", statErr)
	}
}

// The chosen node's address is what the apply writes, in the slot a REMOTE
// endpoint's address goes to.
func TestRoutedApplyWritesTheChosenBaseURL(t *testing.T) {
	isolateConfig(t)
	node := newRoutableNode(t, "qwen3-27b", true, 10)
	dir := t.TempDir()
	fleetPath := fleetFileIn(t, dir, "nodes:\n"+node.entry("gpu-box"))
	spinloopDir := routedSpinloop(t, "qwen3-27b")

	h, _ := harness.Lookup("opencode")
	var sel spinloop.Selection
	var err error
	captureStderr(t, func() {
		captureStdout(t, func() {
			sel, _, _, _, err = applyBeforeLaunch(
				spinloopPathFlag{set: true, path: spinloopDir}, "", h, nil,
				routeOptions{fleetPath: fleetPath})
		})
	})
	if err != nil {
		t.Fatalf("routed apply failed: %v", err)
	}
	want := "http://127.0.0.1:" + strconv.Itoa(node.enginePort) + "/v1"
	if sel.BaseURL != want {
		t.Errorf("applied base URL = %q, want the chosen node's %q", sel.BaseURL, want)
	}
}

// `fleet route` explains the choice a launch would make.
func TestCmdFleetRouteExplainsTheChoice(t *testing.T) {
	node := newRoutableNode(t, "qwen3-27b", true, 42)
	dir := t.TempDir()
	fleetPath := fleetFileIn(t, dir, "prefer: active\nnodes:\n"+node.entry("gpu-box"))
	spinloopDir := routedSpinloop(t, "qwen3-27b")

	out := captureStdout(t, func() {
		if err := cmdFleetRoute([]string{"-f", fleetPath, filepath.Join(spinloopDir, "Spinloop")}); err != nil {
			t.Fatal(err)
		}
	})
	for _, want := range []string{"gpu-box", "Prefer: active", strconv.Itoa(node.enginePort), "42s ago"} {
		if !strings.Contains(out, want) {
			t.Errorf("output should mention %q, got:\n%s", want, out)
		}
	}
}

// A fleet file naming a gateway is answered without querying a node: the
// address is named, and the dead node below proves none is queried.
func TestCmdFleetRouteAgainstAFileNamingAGateway(t *testing.T) {
	fleetPath := gatewayFleetFile(t, "gateway:\n  url: http://gw.internal:4000\n")
	spinloopDir := routedSpinloop(t, "qwen3-27b")

	out := captureStdout(t, func() {
		if err := cmdFleetRoute([]string{"-f", fleetPath, filepath.Join(spinloopDir, "Spinloop")}); err != nil {
			t.Fatal(err)
		}
	})
	for _, want := range []string{
		"names a gateway",
		"would point the agent at http://gw.internal:4000/v1",
		"nothing is started",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output should mention %q, got:\n%s", want, out)
		}
	}
}

// The flag lets the two preferences be compared on a live fleet without
// editing the file.
func TestCmdFleetRoutePreferenceFlagBeatsTheFile(t *testing.T) {
	recent := newRoutableNode(t, "qwen3-27b", true, 5)
	stale := newRoutableNode(t, "qwen3-27b", true, 5000)
	dir := t.TempDir()
	fleetPath := fleetFileIn(t, dir,
		"prefer: idle\nnodes:\n"+recent.entry("recent")+stale.entry("stale"))
	spinloopDir := routedSpinloop(t, "qwen3-27b")
	spinloopFile := filepath.Join(spinloopDir, "Spinloop")

	// The file says idle, so the long-idle node wins.
	out := captureStdout(t, func() {
		if err := cmdFleetRoute([]string{"-f", fleetPath, spinloopFile}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "Would use stale") {
		t.Errorf("the file's preference should choose stale, got:\n%s", out)
	}

	// The flag overrides it, and says so.
	out = captureStdout(t, func() {
		if err := cmdFleetRoute([]string{"-f", fleetPath, "--prefer", "active", spinloopFile}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "Would use recent") || !strings.Contains(out, "Prefer: active") {
		t.Errorf("the flag should choose recent and name itself, got:\n%s", out)
	}

	// ...and the file is untouched.
	body, err := os.ReadFile(fleetPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "prefer: idle") {
		t.Errorf("the fleet file was rewritten:\n%s", body)
	}
}

// Routing changes nothing: nothing is started, nothing is written.
func TestCmdFleetRouteStartsNothing(t *testing.T) {
	home := isolateConfig(t)
	node := newRoutableNode(t, "", false, 0)
	dir := t.TempDir()
	fleetPath := fleetFileIn(t, dir, "nodes:\n"+node.entry("idle-box"))
	spinloopDir := routedSpinloop(t, "qwen3-27b")

	out := captureStdout(t, func() {
		if err := cmdFleetRoute([]string{"-f", fleetPath, filepath.Join(spinloopDir, "Spinloop")}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "would wake idle-box") {
		t.Errorf("output should name the node a launch would wake, got:\n%s", out)
	}
	if !strings.Contains(out, "Nothing has been started") {
		t.Errorf("output should say nothing was started, got:\n%s", out)
	}
	if node.started {
		t.Error("fleet route started an engine")
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "opencode", "opencode.json")); !os.IsNotExist(err) {
		t.Errorf("fleet route wrote a harness config (stat: %v)", err)
	}
}

// A fleet that declares wake: off still reports, when nothing is running, the
// node whose source describes the model and the command that would start it —
// but says a launch would refuse.
func TestCmdFleetRouteWakeOffRefusal(t *testing.T) {
	node := newRoutableNode(t, "", false, 0)
	dir := t.TempDir()
	fleetPath := fleetFileIn(t, dir, "wake: off\nnodes:\n"+node.entry("idle-box"))
	spinloopDir := routedSpinloop(t, "qwen3-27b")

	out := captureStdout(t, func() {
		if err := cmdFleetRoute([]string{"-f", fleetPath, filepath.Join(spinloopDir, "Spinloop")}); err != nil {
			t.Fatal(err)
		}
	})
	for _, want := range []string{"wake is off", "idle-box", "spinloop fleet start idle-box", "Nothing has been started"} {
		if !strings.Contains(out, want) {
			t.Errorf("output should mention %q, got:\n%s", want, out)
		}
	}
	if node.started {
		t.Error("fleet route started an engine")
	}
}

// A named Spinloop, with no --fleet, has nothing to route through: the failure
// names the flag that supplies the fleet.
func TestCmdFleetRouteNeedsAFleet(t *testing.T) {
	spinloopDir := routedSpinloop(t, "qwen3-27b")
	err := cmdFleetRoute([]string{filepath.Join(spinloopDir, "Spinloop")})
	if err == nil {
		t.Fatal("expected a failure")
	}
	if !strings.Contains(err.Error(), "--fleet") {
		t.Errorf("error should suggest --fleet, got: %v", err)
	}
}

// Discovery: a fleet.yaml in the working directory is the fleet for a Spinloop
// the user did not name. A named Spinloop — a flag value, a positional, or the
// alias SPINLOOP_ALIAS names — travels to its fleet only by flag, so a
// fleet.yaml beside the working directory is not picked up for it.

// An explicitly named Spinloop does not pick up the working directory's
// fleet.yaml.
func TestNamedSpinloopDoesNotPickUpCwdFleet(t *testing.T) {
	node := newRoutableNode(t, "qwen3-27b", true, 10)
	dir := t.TempDir()
	fleetFileIn(t, dir, "nodes:\n"+node.entry("gpu-box"))
	t.Chdir(dir)

	spinloopDir := t.TempDir()
	mustWrite(t, filepath.Join(spinloopDir, "Spinloop"), "PROVIDER llamacpp\nMODEL qwen3-27b\n")
	sel, path, err := readSpinloop("test", filepath.Join(spinloopDir, "Spinloop"))
	if err != nil {
		t.Fatal(err)
	}
	choice, err := routeThroughFleet(sel, path, routeOptions{spinloopNamed: true})
	if err != nil || choice != nil {
		t.Errorf("choice = %+v, err = %v; a named Spinloop should not pick up the working directory's fleet", choice, err)
	}
}

// The other side of the same rule: a Spinloop the user did not name finds the
// working directory's fleet.yaml.
func TestUnnamedSpinloopPicksUpCwdFleet(t *testing.T) {
	node := newRoutableNode(t, "qwen3-27b", true, 10)
	dir := t.TempDir()
	fleetFileIn(t, dir, "nodes:\n"+node.entry("gpu-box"))
	t.Chdir(dir)

	spinloopDir := t.TempDir()
	mustWrite(t, filepath.Join(spinloopDir, "Spinloop"), "PROVIDER llamacpp\nMODEL qwen3-27b\n")
	sel, path, err := readSpinloop("test", filepath.Join(spinloopDir, "Spinloop"))
	if err != nil {
		t.Fatal(err)
	}
	captureStderr(t, func() {
		c, err := routeThroughFleet(sel, path, routeOptions{spinloopNamed: false})
		if err != nil {
			t.Fatalf("an unnamed Spinloop should find the working directory's fleet: %v", err)
		}
		if c == nil || c.Node.Name != "gpu-box" {
			t.Fatalf("expected the working directory's fleet, got %+v", c)
		}
	})
}

// --fleet routes a Spinloop the user named, where the working directory would
// otherwise give no fleet at all.
func TestFleetFlagRoutesANamedSpinloop(t *testing.T) {
	node := newRoutableNode(t, "qwen3-27b", true, 10)
	dir := t.TempDir()
	flagFleet := fleetFileIn(t, dir, "nodes:\n"+node.entry("from-flag"))
	t.Chdir(t.TempDir()) // an empty working directory: no fleet.yaml to fall back on

	spinloopDir := t.TempDir()
	mustWrite(t, filepath.Join(spinloopDir, "Spinloop"), "PROVIDER llamacpp\nMODEL qwen3-27b\n")
	sel, path, err := readSpinloop("test", filepath.Join(spinloopDir, "Spinloop"))
	if err != nil {
		t.Fatal(err)
	}
	captureStderr(t, func() {
		c, err := routeThroughFleet(sel, path, routeOptions{spinloopNamed: true, fleetPath: flagFleet})
		if err != nil {
			t.Fatalf("routing failed: %v", err)
		}
		if c == nil || c.Node.Name != "from-flag" {
			t.Fatalf("--fleet should route a named Spinloop, got %+v", c)
		}
	})
}

// The alias SPINLOOP_ALIAS names is a Spinloop the user named: a fleet.yaml in
// the working directory is not picked up for it, even though no path was typed.
func TestAliasNamedSpinloopDoesNotPickUpCwdFleet(t *testing.T) {
	isolateConfig(t)
	t.Setenv("OPENAI_BASE_URL", "")
	node := newRoutableNode(t, "qwen3-27b", true, 10)
	registerSpinloop(t, "PROVIDER llamacpp\nMODEL qwen3-27b\nALIAS q3\n")

	dir := t.TempDir()
	fleetFileIn(t, dir, "nodes:\n"+node.entry("gpu-box"))
	t.Chdir(dir) // the fleet.yaml, but no Spinloop: the alias supplies the Spinloop
	t.Setenv("SPINLOOP_ALIAS", "q3")

	argsFile := filepath.Join(t.TempDir(), "args")
	stubHarnessBinary(t, "opencode", argsFile)
	stderr := captureStderr(t, func() {
		captureStdout(t, func() {
			if err := cmdHarness([]string{"-O", "--", "run"}); err != nil {
				t.Fatalf("cmdHarness: %v", err)
			}
		})
	})
	if _, err := os.ReadFile(argsFile); err != nil {
		t.Fatalf("harness was not launched: %v", err)
	}
	if strings.Contains(stderr, "Routing through") {
		t.Errorf("a Spinloop named by SPINLOOP_ALIAS should not pick up the working directory's fleet, got:\n%s", stderr)
	}
}

// The valueless --spinloop wears the default Spinloop and is not named: a
// launch run from a directory holding both a Spinloop and a fleet.yaml routes
// through that fleet.
func TestValuelessSpinloopFlagRoutesThroughCwdFleet(t *testing.T) {
	home := isolateConfig(t)
	t.Setenv("OPENAI_BASE_URL", "")
	node := newRoutableNode(t, "qwen3-27b", true, 10)
	dir := t.TempDir()
	fleetFileIn(t, dir, "nodes:\n"+node.entry("gpu-box"))
	mustWrite(t, filepath.Join(dir, "Spinloop"), "PROVIDER llamacpp\nMODEL qwen3-27b\n")
	t.Chdir(dir)

	argsFile := filepath.Join(t.TempDir(), "args")
	envFile := filepath.Join(t.TempDir(), "env")
	stubHarnessBinaryWithEnv(t, argsFile, envFile)
	stderr := captureStderr(t, func() {
		captureStdout(t, func() {
			if err := cmdHarness([]string{"-O", "--", "run"}); err != nil {
				t.Fatalf("cmdHarness -O: %v", err)
			}
		})
	})
	if !strings.Contains(stderr, "gpu-box") {
		t.Errorf("the valueless --spinloop should route through the working directory's fleet, got:\n%s", stderr)
	}
	data, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatal(err)
	}
	want := "BASE=http://127.0.0.1:" + strconv.Itoa(node.enginePort) + "/v1"
	if !strings.Contains(string(data), want) {
		t.Errorf("the agent should point at the chosen node, got:\n%s", data)
	}
	_ = home
}

// A bare `spinloop harness` wears no Spinloop at all, so it routes nowhere even
// from a directory holding both a Spinloop and a fleet.yaml.
func TestBareHarnessWearsNothingAndRoutesNothing(t *testing.T) {
	isolateConfig(t)
	t.Setenv("OPENAI_BASE_URL", "")
	node := newRoutableNode(t, "qwen3-27b", true, 10)
	dir := t.TempDir()
	fleetFileIn(t, dir, "nodes:\n"+node.entry("gpu-box"))
	mustWrite(t, filepath.Join(dir, "Spinloop"), "PROVIDER llamacpp\nMODEL qwen3-27b\n")
	t.Chdir(dir)

	argsFile := filepath.Join(t.TempDir(), "args")
	stubHarnessBinary(t, "opencode", argsFile)
	stderr := captureStderr(t, func() {
		captureStdout(t, func() {
			if err := cmdHarness([]string{"--", "run"}); err != nil {
				t.Fatalf("cmdHarness: %v", err)
			}
		})
	})
	if _, err := os.ReadFile(argsFile); err != nil {
		t.Fatalf("harness was not launched: %v", err)
	}
	if strings.Contains(stderr, "Routing through") {
		t.Errorf("a bare harness wears nothing and routes nowhere, got:\n%s", stderr)
	}
}
