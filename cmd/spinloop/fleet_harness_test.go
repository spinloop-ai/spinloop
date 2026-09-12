package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// fleetHarnessDir writes a fleet file and a Spinloop into a temp dir and moves
// the test into it: the command's defaults — the fleet.yaml beside it, the
// Spinloop beside it — are working-directory stories.
func fleetHarnessDir(t *testing.T, fleetBody, spinloopBody string) {
	t.Helper()
	dir := t.TempDir()
	if fleetBody != "" {
		mustWrite(t, filepath.Join(dir, "fleet.yaml"), fleetBody)
	}
	if spinloopBody != "" {
		mustWrite(t, filepath.Join(dir, "Spinloop"), spinloopBody)
	}
	t.Chdir(dir)
}

// No flags at all: the fleet file beside the command names a gateway, so the
// agent is pointed at it with the section's token, and no node is contacted.
func TestCmdFleetHarnessUsesTheFleetsGateway(t *testing.T) {
	home := isolateConfig(t)
	t.Setenv("GATEWAY_TOKEN", "gw-token")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENAI_BASE_URL", "")
	argsFile := filepath.Join(t.TempDir(), "args")
	envFile := filepath.Join(t.TempDir(), "env")
	stubHarnessBinaryWithEnv(t, argsFile, envFile)

	fleetHarnessDir(t,
		"nodes:\n  - name: dead\n    host: 127.0.0.1\n    port: 1\ngateway:\n  url: http://gw.internal:4000\n  tokenEnv: GATEWAY_TOKEN\n",
		"PROVIDER llamacpp\nMODEL qwen3-27b\n")
	stderr := captureStderr(t, func() {
		captureStdout(t, func() {
			if err := cmdFleetHarness(nil); err != nil {
				t.Fatalf("cmdFleetHarness: %v", err)
			}
		})
	})
	if !strings.Contains(stderr, "Routing at http://gw.internal:4000/v1 — the fleet file names a gateway") {
		t.Errorf("the choice should be reported on stderr before the launch, got:\n%s", stderr)
	}
	if _, err := os.ReadFile(argsFile); err != nil {
		t.Fatalf("the harness was not launched: %v", err)
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

// A fleet file naming no gateway routes to a node, as a fleet-routed launch
// would.
func TestCmdFleetHarnessRoutesToANode(t *testing.T) {
	node := newRoutableNode(t, "qwen3-27b", true, 300)
	isolateConfig(t)
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENAI_BASE_URL", "")
	argsFile := filepath.Join(t.TempDir(), "args")
	envFile := filepath.Join(t.TempDir(), "env")
	stubHarnessBinaryWithEnv(t, argsFile, envFile)

	fleetHarnessDir(t,
		"nodes:\n"+node.entry("gpu-box"),
		"PROVIDER llamacpp\nMODEL qwen3-27b\n")
	captureStdout(t, func() {
		if err := cmdFleetHarness(nil); err != nil {
			t.Fatalf("cmdFleetHarness: %v", err)
		}
	})
	data, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatal(err)
	}
	want := "BASE=http://127.0.0.1:" + strconv.Itoa(node.enginePort) + "/v1"
	if !strings.Contains(string(data), want) {
		t.Errorf("the agent's base URL should be the node's engine, got:\n%s", data)
	}
}

// -f beats the Spinloop's FLEET: the section in the named file is where the
// agent is pointed, not the endpoint the Spinloop names.
func TestCmdFleetHarnessFileFlagBeatsTheSpinloopsFleet(t *testing.T) {
	isolateConfig(t)
	t.Setenv("OPENAI_API_KEY", "gw-token")
	t.Setenv("OPENAI_BASE_URL", "")
	argsFile := filepath.Join(t.TempDir(), "args")
	envFile := filepath.Join(t.TempDir(), "env")
	stubHarnessBinaryWithEnv(t, argsFile, envFile)

	fleetPath := fleetFileIn(t, t.TempDir(),
		"nodes:\n  - name: dead\n    host: 127.0.0.1\n    port: 1\ngateway:\n  url: http://b.internal:4000\n")
	spinloopDir := routedSpinloop(t, "qwen3-27b", "http://a.internal:4000")
	captureStdout(t, func() {
		if err := cmdFleetHarness([]string{"-f", fleetPath, filepath.Join(spinloopDir, "Spinloop")}); err != nil {
			t.Fatalf("cmdFleetHarness: %v", err)
		}
	})
	data, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "BASE=http://b.internal:4000/v1") {
		t.Errorf("the command's file should be the one routed through, got:\n%s", data)
	}
}

// With no -f, the Spinloop's FLEET — an endpoint here — is used, exactly as
// --fleet overrides an instruction on spinloop harness.
func TestCmdFleetHarnessSpinloopFleetIsUsedWithoutAFlag(t *testing.T) {
	isolateConfig(t)
	t.Setenv("OPENAI_API_KEY", "gw-token")
	t.Setenv("OPENAI_BASE_URL", "")
	argsFile := filepath.Join(t.TempDir(), "args")
	envFile := filepath.Join(t.TempDir(), "env")
	stubHarnessBinaryWithEnv(t, argsFile, envFile)

	spinloopDir := routedSpinloop(t, "qwen3-27b", "http://a.internal:4000")
	captureStdout(t, func() {
		if err := cmdFleetHarness([]string{filepath.Join(spinloopDir, "Spinloop")}); err != nil {
			t.Fatalf("cmdFleetHarness: %v", err)
		}
	})
	data, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "BASE=http://a.internal:4000/v1") {
		t.Errorf("without -f the Spinloop's FLEET should be routed through, got:\n%s", data)
	}
}

// A directory holding no Spinloop, with none given: the command fails saying a
// launch needs a Spinloop to know which model to route, and launches nothing.
func TestCmdFleetHarnessNeedsASpinloop(t *testing.T) {
	isolateConfig(t)
	t.Setenv("SPINLOOP_ALIAS", "")
	argsFile := filepath.Join(t.TempDir(), "args")
	stubHarnessBinary(t, "opencode", argsFile)

	t.Chdir(t.TempDir())
	captureStdout(t, func() {
		err := cmdFleetHarness(nil)
		if err == nil {
			t.Fatal("a launch with no Spinloop should fail")
		}
		if !strings.Contains(err.Error(), "a launch needs a Spinloop to know which model to route") {
			t.Errorf("the failure should say a launch needs a Spinloop, got:\n%v", err)
		}
	})
	if _, err := os.ReadFile(argsFile); err == nil {
		t.Error("the harness was launched with no Spinloop to route")
	}
}

// A leading argument that is not a Spinloop is refused: there is no forwarding
// on this command, so a stray positional names nothing it could be.
func TestCmdFleetHarnessRefusesANonSpinloopArgument(t *testing.T) {
	isolateConfig(t)
	argsFile := filepath.Join(t.TempDir(), "args")
	stubHarnessBinary(t, "opencode", argsFile)

	fleetHarnessDir(t,
		"nodes:\n  - name: dead\n    host: 127.0.0.1\n    port: 1\ngateway:\n  url: http://gw.internal:4000\n",
		"PROVIDER llamacpp\nMODEL qwen3-27b\n")
	captureStdout(t, func() {
		err := cmdFleetHarness([]string{"not-a-spinloop"})
		if err == nil {
			t.Fatal("a positional that names no Spinloop should fail")
		}
		if !strings.Contains(err.Error(), "does not name a Spinloop") {
			t.Errorf("the failure should say the argument names no Spinloop, got:\n%v", err)
		}
	})
	if _, err := os.ReadFile(argsFile); err == nil {
		t.Error("the harness was launched for an argument that names no Spinloop")
	}
}

// The Spinloop is taken the way spinloop harness takes one: -O, in each of its
// spellings — valueless, space, and equals.
func TestCmdFleetHarnessSpinloopFlag(t *testing.T) {
	node := newRoutableNode(t, "qwen3-27b", true, 300)
	isolateConfig(t)
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENAI_BASE_URL", "")
	argsFile := filepath.Join(t.TempDir(), "args")
	envFile := filepath.Join(t.TempDir(), "env")
	stubHarnessBinaryWithEnv(t, argsFile, envFile)

	fleetHarnessDir(t, "nodes:\n"+node.entry("gpu-box"), "PROVIDER llamacpp\nMODEL qwen3-27b\n")
	forms := []struct {
		name string
		args []string
	}{
		{"valueless", []string{"-O"}},
		{"space", []string{"-O", "Spinloop"}},
		{"equals", []string{"-O=Spinloop"}},
	}
	want := "BASE=http://127.0.0.1:" + strconv.Itoa(node.enginePort) + "/v1"
	for _, form := range forms {
		t.Run(form.name, func(t *testing.T) {
			if err := os.Remove(envFile); err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			captureStdout(t, func() {
				if err := cmdFleetHarness(form.args); err != nil {
					t.Fatalf("cmdFleetHarness %v: %v", form.args, err)
				}
			})
			data, err := os.ReadFile(envFile)
			if err != nil {
				t.Fatalf("the harness was not launched: %v", err)
			}
			if !strings.Contains(string(data), want) {
				t.Errorf("the agent's base URL should be the node's engine, got:\n%s", data)
			}
		})
	}
}

// A -O that names a Spinloop there is no read of fails naming the path, and
// launches nothing.
func TestCmdFleetHarnessNamesTheSpinloopItCannotRead(t *testing.T) {
	isolateConfig(t)
	argsFile := filepath.Join(t.TempDir(), "args")
	stubHarnessBinary(t, "opencode", argsFile)

	missing := filepath.Join(t.TempDir(), "Spinloop")
	fleetHarnessDir(t, "nodes:\n  - name: dead\n    host: 127.0.0.1\n    port: 1\n", "")
	captureStdout(t, func() {
		err := cmdFleetHarness([]string{"-O=" + missing})
		if err == nil {
			t.Fatal("a -O naming a missing Spinloop should fail")
		}
		if !strings.Contains(err.Error(), missing) {
			t.Errorf("the failure should name the path, got:\n%v", err)
		}
	})
	if _, err := os.ReadFile(argsFile); err == nil {
		t.Error("the harness was launched for a Spinloop that could not be read")
	}
}

// A harness the command cannot resolve fails before anything is routed or
// written.
func TestCmdFleetHarnessRefusesAnUnknownHarness(t *testing.T) {
	home := isolateConfig(t)
	argsFile := filepath.Join(t.TempDir(), "args")
	stubHarnessBinary(t, "opencode", argsFile)

	fleetHarnessDir(t, "nodes:\n  - name: dead\n    host: 127.0.0.1\n    port: 1\n",
		"PROVIDER llamacpp\nMODEL qwen3-27b\n")
	captureStdout(t, func() {
		err := cmdFleetHarness([]string{"-H", "nosuchharness"})
		if err == nil {
			t.Fatal("an unresolvable harness should fail")
		}
		if !strings.Contains(err.Error(), "nosuchharness") {
			t.Errorf("the failure should name the harness, got:\n%v", err)
		}
	})
	if _, err := os.ReadFile(argsFile); err == nil {
		t.Error("the harness was launched for a name that resolves to nothing")
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "opencode", "opencode.json")); !os.IsNotExist(err) {
		t.Errorf("a harness config was written for a launch that never routed (stat: %v)", err)
	}
}

// Nothing serving, wake allowed: the command starts the idle node's engine and
// points the agent at it, the way a fleet-routed launch does — the wake
// timeout flag bounds the wait.
func TestCmdFleetHarnessWakesANode(t *testing.T) {
	node := newRoutableNode(t, "qwen3-27b", false, 0)
	isolateConfig(t)
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENAI_BASE_URL", "")
	argsFile := filepath.Join(t.TempDir(), "args")
	envFile := filepath.Join(t.TempDir(), "env")
	stubHarnessBinaryWithEnv(t, argsFile, envFile)

	fleetHarnessDir(t, "nodes:\n"+node.entry("idle-box"), "PROVIDER llamacpp\nMODEL qwen3-27b\n")
	stderr := captureStderr(t, func() {
		captureStdout(t, func() {
			if err := cmdFleetHarness([]string{"--wake-timeout", "5s"}); err != nil {
				t.Fatalf("cmdFleetHarness: %v", err)
			}
		})
	})
	if !node.started {
		t.Error("the idle node was not started")
	}
	if !strings.Contains(stderr, "Using idle-box at") {
		t.Errorf("the woken node should be named on stderr, got:\n%s", stderr)
	}
	data, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatalf("the harness was not launched: %v", err)
	}
	want := "BASE=http://127.0.0.1:" + strconv.Itoa(node.enginePort) + "/v1"
	if !strings.Contains(string(data), want) {
		t.Errorf("the agent's base URL should be the woken engine, got:\n%s", data)
	}
}

// --no-wake with nothing serving fails rather than starting an engine, and the
// refusal names the node and the flag to drop.
func TestCmdFleetHarnessNoWakeRefuses(t *testing.T) {
	node := newRoutableNode(t, "qwen3-27b", false, 0)
	isolateConfig(t)
	argsFile := filepath.Join(t.TempDir(), "args")
	stubHarnessBinary(t, "opencode", argsFile)

	fleetHarnessDir(t, "nodes:\n"+node.entry("idle-box"), "PROVIDER llamacpp\nMODEL qwen3-27b\n")
	captureStdout(t, func() {
		err := cmdFleetHarness([]string{"--no-wake"})
		if err == nil {
			t.Fatal("--no-wake with nothing serving should fail")
		}
		for _, want := range []string{"idle-box", "spinloop fleet start", "drop --no-wake"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("the refusal should say %q, got:\n%v", want, err)
			}
		}
	})
	if node.started {
		t.Error("--no-wake started an engine")
	}
	if _, err := os.ReadFile(argsFile); err == nil {
		t.Error("the harness was launched for a route that refused to wake")
	}
}
