package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/spinloop-ai/spinloop/internal/cloud"
)

// The rename's flat rule: `--env` is gone from the launch, and it is not
// even forwarded to the harness — the command names the new spelling.
func TestCodeRejectsRemovedEnvFlag(t *testing.T) {
	isolateConfig(t)
	err := cmdCode([]string{"--env", "dev-1"})
	if err == nil || !strings.Contains(err.Error(), "--env") || !strings.Contains(err.Error(), "renamed --node") {
		t.Fatalf("want a rename failure pointing at --node, got %v", err)
	}
	err = cmdCode([]string{"-e", "dev-1"})
	if err == nil || !strings.Contains(err.Error(), "renamed --node") {
		t.Fatalf("the -e shorthand should say the same, got %v", err)
	}
}

func TestApplyRejectsRemovedEnvFlag(t *testing.T) {
	isolateConfig(t)
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "Spinloop"), "PROVIDER llamacpp\nMODEL qwen3-27b\n")
	err := cmdApply([]string{"--env", "dev-1", filepath.Join(dir, "Spinloop")})
	if err == nil || !strings.Contains(err.Error(), "unknown flag") {
		t.Fatalf("want an unknown-flag failure for --env, got %v", err)
	}
	err = cmdApply([]string{"-e", "dev-1", filepath.Join(dir, "Spinloop")})
	if err == nil || !strings.Contains(err.Error(), "unknown shorthand flag") {
		t.Fatalf("want an unknown-shorthand failure for -e, got %v", err)
	}
}

// With a fleet in force the one --node pins within it: the pinned node answers
// even when another would be preferred by the activity ranking.
func TestCodeNodePinsFleetNode(t *testing.T) {
	idleFirst := newRoutableNode(t, "qwen3-27b", true, 3000)
	pinned := newRoutableNode(t, "qwen3-27b", true, 300)
	isolateConfig(t)
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENAI_BASE_URL", "")
	argsFile := filepath.Join(t.TempDir(), "args")
	envFile := filepath.Join(t.TempDir(), "env")
	stubHarnessBinaryWithEnv(t, argsFile, envFile)

	codeFleetDir(t,
		"prefer: idle\nnodes:\n"+idleFirst.entry("gpu-a")+pinned.entry("gpu-b"),
		"PROVIDER llamacpp\nMODEL qwen3-27b\n")
	captureStdout(t, func() {
		if err := cmdCode([]string{"-O", "--node", "gpu-b"}); err != nil {
			t.Fatalf("cmdCode: %v", err)
		}
	})
	data, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatal(err)
	}
	want := "BASE=http://127.0.0.1:" + strconv.Itoa(pinned.enginePort) + "/v1"
	if !strings.Contains(string(data), want) {
		t.Errorf("the pinned node should serve, got:\n%s", data)
	}
}

// --node with no fleet in force names a registered environment: the bare
// launch (no Spinloop) is configured from what that environment reports
// deployed — the job `--env` did.
func TestCodeNodeWithoutFleetIsAnEnvironment(t *testing.T) {
	home := isolateConfig(t)
	stubAWSEnv(t)
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENAI_BASE_URL", "")
	codeFleetDir(t, "", "")

	server := deployedEnvServer(t)
	defer server.Close()
	registerEnv(t, "dev-3", cloud.Config{
		StartURL: server.URL, StopURL: server.URL, EnvURL: server.URL,
		BaseURL: "http://198.51.100.1:8000/v1", Region: "eu-west-1", Environment: "dev-3",
	})

	argsFile := filepath.Join(t.TempDir(), "args")
	envFile := filepath.Join(t.TempDir(), "env")
	stubHarnessBinaryWithEnv(t, argsFile, envFile)

	captureStdout(t, func() {
		if err := cmdCode([]string{"-n", "dev-3"}); err != nil {
			t.Fatalf("cmdCode: %v", err)
		}
	})
	data, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "KEY=") || !strings.Contains(string(data), "BASE=") {
		t.Errorf("the agent should carry the environment's fetched endpoint, got:\n%s", data)
	}
	config, err := os.ReadFile(filepath.Join(home, ".config", "opencode", "opencode.json"))
	if err != nil {
		t.Fatalf("the harness config was not written: %v", err)
	}
	if !strings.Contains(string(config), "dev-3") {
		t.Errorf("the provider should be keyed by the environment, got:\n%s", config)
	}
}

// The model-gated demand for a Spinloop belongs to --fleet only: a fleet
// named by flag with no gateway cannot route by model. A fleet merely
// discovered in the working directory is different — naming --node there is
// the operator's whole intent — and a pin answers without a model.
func TestCodeNodeWithGatewaylessFleetWantsSpinloop(t *testing.T) {
	isolateConfig(t)
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENAI_BASE_URL", "")
	dir := t.TempDir()
	fleetPath := filepath.Join(dir, "fleet.yaml")
	mustWrite(t, fleetPath, "wake: off\nnodes:\n  - name: gpu-a\n    host: 127.0.0.1\n    port: 1\n")
	t.Chdir(t.TempDir()) // no fleet.yaml in force: only the flag brings one

	err := cmdCode([]string{"--fleet", fleetPath})
	if err == nil || !strings.Contains(err.Error(), "--fleet needs a Spinloop") {
		t.Fatalf("a bare --fleet with no gateway should ask for a Spinloop, got %v", err)
	}
}

// A discovered gateway-less fleet with a node pin and nothing to route by:
// the pin still says which node the launch wants, so the launch proceeds to
// it rather than demanding a Spinloop it cannot answer with. (It reaches a
// dead node, so it fails there — that is a dead node's failure, not the
// rename's: the message names the node, not a Spinloop.)
func TestCodeNodePinWithDiscoveredFleetReachesNode(t *testing.T) {
	isolateConfig(t)
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENAI_BASE_URL", "")
	codeFleetDir(t, "wake: off\nnodes:\n  - name: gpu-a\n    host: 127.0.0.1\n    port: 1\n", "")
	err := cmdCode([]string{"-n", "gpu-a"})
	if err == nil {
		t.Fatal("a pin to a dead node should fail")
	}
	if strings.Contains(err.Error(), "Spinloop") {
		t.Errorf("a pinned node should not be asked to name a Spinloop: %v", err)
	}
	if !strings.Contains(err.Error(), "gpu-a") {
		t.Errorf("the failure should name the pinned node, got %v", err)
	}
}

// The renamed flag's shape on the launch: --node/-n registered, --env gone,
// and the fleet's nodes completing it beside the registered environments.
func TestCodeNodeFlagAndCompletion(t *testing.T) {
	isolateConfig(t)
	codeFleetDir(t, "nodes:\n  - name: gpu-a\n    host: 127.0.0.1\n    port: 1\n", "")

	c := codeCmd()
	f := c.Flags().Lookup("node")
	if f == nil {
		t.Fatal("--node is not registered on the launch")
	}
	if f.Shorthand != "n" {
		t.Errorf("-n is the --node shorthand, got %q", f.Shorthand)
	}
	if c.Flags().Lookup("env") != nil {
		t.Error("the old --env flag is still registered on the launch")
	}

	values, directive := launchSlot(c, []string{"--node"}, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("a flag's value never completes paths, directive %d", directive)
	}
	var offered bool
	for _, v := range values {
		if v == "gpu-a" {
			offered = true
		}
	}
	if !offered {
		t.Errorf("--node should complete the fleet's nodes, got %v", values)
	}
}
