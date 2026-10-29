package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spinloop-ai/spinloop/internal/cloud"
	"github.com/spinloop-ai/spinloop/internal/fleet"
)

// registerTargetEnv points the registry at a temp config directory and
// registers one environment there, so a --node target has something to
// resolve.
func registerTargetEnv(t *testing.T, name string) {
	t.Helper()
	t.Setenv("SPINLOOP_CONFIG_DIR", t.TempDir())
	registerEnv(t, name, cloud.Config{
		StartURL:    "https://s.example/start",
		StopURL:     "https://s.example/stop",
		Region:      "us-east-1",
		Environment: name,
	})
}

// oneNodeFleetBody is a fleet file naming a single daemon node.
const oneNodeFleetBody = "nodes:\n  - name: box\n    host: box.local\n"

// The three ways a target is named, and the one way it fails to resolve.
func TestResolveFleetTarget(t *testing.T) {
	t.Run("a named environment", func(t *testing.T) {
		registerTargetEnv(t, "prod")
		t.Chdir(t.TempDir())
		cfg, err := resolveFleetTarget(fleetTarget{nodeName: "prod"})
		if err != nil {
			t.Fatal(err)
		}
		if len(cfg.Nodes) != 1 || cfg.Nodes[0].Name != "prod" {
			t.Fatalf("nodes = %+v, want just prod", cfg.Nodes)
		}
		if cfg.Nodes[0].Kind != fleet.KindCloud {
			t.Errorf("kind = %q, want %q", cfg.Nodes[0].Kind, fleet.KindCloud)
		}
	})

	t.Run("a named file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "cluster.yaml")
		if err := os.WriteFile(path, []byte(oneNodeFleetBody), 0o600); err != nil {
			t.Fatal(err)
		}
		cfg, err := resolveFleetTarget(fleetTarget{fleetPath: path})
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Path != path {
			t.Errorf("path = %q, want %q", cfg.Path, path)
		}
	})

	t.Run("the working directory's fleet file", func(t *testing.T) {
		writeFleetFile(t, oneNodeFleetBody)
		cfg, err := resolveFleetTarget(fleetTarget{})
		if err != nil {
			t.Fatal(err)
		}
		if len(cfg.Nodes) != 1 || cfg.Nodes[0].Name != "box" {
			t.Fatalf("nodes = %+v, want the directory's node", cfg.Nodes)
		}
	})

	t.Run("no fleet file names the expected path", func(t *testing.T) {
		t.Chdir(t.TempDir())
		_, err := resolveFleetTarget(fleetTarget{})
		if err == nil {
			t.Fatal("want an error")
		}
		if !strings.Contains(err.Error(), fleet.DefaultFile) {
			t.Errorf("error %q does not name %s", err, fleet.DefaultFile)
		}
	})
}

// Naming both a fleet and an environment is refused, naming both values.
func TestResolveFleetTargetRefusesBoth(t *testing.T) {
	registerTargetEnv(t, "prod")
	dir := t.TempDir()
	path := filepath.Join(dir, "cluster.yaml")
	if err := os.WriteFile(path, []byte(oneNodeFleetBody), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := resolveFleetTarget(fleetTarget{nodeName: "prod", fleetPath: path})
	if err == nil {
		t.Fatal("want an error")
	}
	for _, want := range []string{"--node prod", path, "two answers to one target"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

// A fleet.yaml merely sitting in the working directory is not a conflict with
// --node: only a flag states a target, so the environment wins and the file is
// not read (design D4).
func TestResolveFleetTargetIgnoresTheDirectoryFileWithEnv(t *testing.T) {
	registerTargetEnv(t, "prod")
	writeFleetFile(t, oneNodeFleetBody)

	cfg, err := resolveFleetTarget(fleetTarget{nodeName: "prod"})
	if err != nil {
		t.Fatalf("a directory fleet file was treated as a conflict: %v", err)
	}
	if len(cfg.Nodes) != 1 || cfg.Nodes[0].Name != "prod" {
		t.Fatalf("nodes = %+v, want just prod", cfg.Nodes)
	}
}

// An unresolvable environment fails before anything is contacted, naming the
// repair.
func TestResolveFleetTargetUnregisteredEnv(t *testing.T) {
	t.Setenv("SPINLOOP_CONFIG_DIR", t.TempDir())
	t.Chdir(t.TempDir())
	_, err := resolveFleetTarget(fleetTarget{nodeName: "nope"})
	if err == nil {
		t.Fatal("want an error")
	}
	if !strings.Contains(err.Error(), "clouds/nope/cloud.json") {
		t.Errorf("error %q does not name the environment's registry path", err)
	}
}

// checkOneTarget is the rule both the fleet commands and the launch enforce,
// so it passes whenever fewer than two targets are named.
func TestCheckOneTarget(t *testing.T) {
	tests := []struct {
		name      string
		env       string
		fleet     string
		wantError bool
	}{
		{"neither", "", "", false},
		{"only an environment", "prod", "", false},
		{"only a fleet", "", "./fleet.yaml", false},
		{"both", "prod", "./fleet.yaml", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkOneTarget(tt.env, tt.fleet)
			if tt.wantError && err == nil {
				t.Fatal("want an error")
			}
			if !tt.wantError && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// A fleet of one has no path, so what reports the target names the
// environment instead of printing an empty string.
func TestDescribeTarget(t *testing.T) {
	registerTargetEnv(t, "prod")
	fromEnv, err := fleet.ForEnvironment("prod")
	if err != nil {
		t.Fatal(err)
	}
	if got := describeTarget(fromEnv); got != "environment prod" {
		t.Errorf("describeTarget = %q, want %q", got, "environment prod")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "cluster.yaml")
	if err := os.WriteFile(path, []byte(oneNodeFleetBody), 0o600); err != nil {
		t.Fatal(err)
	}
	fromFile, err := fleet.Resolve(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := describeTarget(fromFile); got != path {
		t.Errorf("describeTarget = %q, want %q", got, path)
	}
}

// Every fleet command that takes a target completes --node from the registered
// environments, the same source `cloud --node` completes from.
func TestFleetCommandsCompleteEnv(t *testing.T) {
	registerTargetEnv(t, "prod")
	registerEnv(t, "staging", cloud.Config{
		StartURL:    "https://s.example/start",
		StopURL:     "https://s.example/stop",
		Region:      "us-east-1",
		Environment: "staging",
	})
	t.Chdir(t.TempDir())

	for _, sub := range []string{"start", "stop", "deploy", "route"} {
		t.Run(sub, func(t *testing.T) {
			got, _ := complete(t, "fleet", sub, "--node", "")
			want := map[string]bool{"prod": false, "staging": false}
			for _, c := range got {
				if _, ok := want[c]; ok {
					want[c] = true
				}
			}
			for name, seen := range want {
				if !seen {
					t.Errorf("fleet %s --node did not offer %q (got %v)", sub, name, got)
				}
			}
		})
	}
}

// The whole claim of a fleet of one: the same environment named by --node and
// named by a one-node fleet file produces the same output, so what a command
// prints does not depend on how its target was named.
func TestFleetStatusEnvMatchesAOneNodeFile(t *testing.T) {
	stubAWSEnv(t)
	up := stateServer(t)
	t.Setenv("SPINLOOP_CONFIG_DIR", t.TempDir())
	registerEnv(t, "prod", cloud.Config{
		StartURL:    up.URL,
		StopURL:     up.URL,
		StatsURL:    up.URL,
		Region:      "us-east-1",
		Environment: "prod",
	})

	// Named by a fleet file holding exactly that node.
	writeFleetFile(t, "nodes:\n  - name: prod\n    kind: cloud\n")
	fromFile := captureStdout(t, func() {
		if err := cmdStatus(nil); err != nil {
			t.Errorf("fleet status returned %v", err)
		}
	})

	// Named by the flag, from a directory holding no fleet file at all.
	t.Chdir(t.TempDir())
	fromEnv := captureStdout(t, func() {
		if err := cmdStatus([]string{"--node", "prod"}); err != nil {
			t.Errorf("fleet status --node returned %v", err)
		}
	})

	if fromFile != fromEnv {
		t.Errorf("output differs by how the target was named:\nfile:\n%s\nenv:\n%s", fromFile, fromEnv)
	}
	if !strings.Contains(fromEnv, "prod") {
		t.Errorf("the environment is not named in the output:\n%s", fromEnv)
	}
}

// A --node target needs no fleet file: the command works in a directory that
// holds none, which is the point of the flag.
func TestFleetStatusEnvNeedsNoFleetFile(t *testing.T) {
	stubAWSEnv(t)
	up := stateServer(t)
	t.Setenv("SPINLOOP_CONFIG_DIR", t.TempDir())
	registerEnv(t, "prod", cloud.Config{
		StartURL:    up.URL,
		StopURL:     up.URL,
		StatsURL:    up.URL,
		Region:      "us-east-1",
		Environment: "prod",
	})
	t.Chdir(t.TempDir())

	out := captureStdout(t, func() {
		if err := cmdStatus([]string{"--node", "prod"}); err != nil {
			t.Errorf("fleet status --node returned %v", err)
		}
	})
	if !strings.Contains(out, "prod") {
		t.Errorf("no row for the environment:\n%s", out)
	}
}

// Naming both through the real command line is refused, not just through the
// resolver directly.
func TestFleetStatusRefusesEnvAndFleet(t *testing.T) {
	t.Setenv("SPINLOOP_CONFIG_DIR", t.TempDir())
	registerEnv(t, "prod", cloud.Config{
		StartURL:    "https://s.example/start",
		StopURL:     "https://s.example/stop",
		Region:      "us-east-1",
		Environment: "prod",
	})
	writeFleetFile(t, oneNodeFleetBody)

	err := cmdStatus([]string{"--node", "prod", "--fleet", "fleet.yaml"})
	if err == nil {
		t.Fatal("want an error")
	}
	for _, want := range []string{"--node prod", "fleet.yaml", "two answers to one target"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

// With no fleet in force and no file to check, --node resolves as the
// environment; an unregistered name is refused. (Both branches of the
// discovery check, minus the file.)
func TestResolveTargetNoFileResolvesRegistry(t *testing.T) {
	registerTargetEnv(t, "prod")
	t.Chdir(t.TempDir())
	cfg, err := resolveFleetTarget(fleetTarget{nodeName: "prod"})
	if err != nil {
		t.Fatalf("a registered environment with no fleet in force: %v", err)
	}
	if len(cfg.Nodes) != 1 || cfg.Nodes[0].Name != "prod" {
		t.Errorf("want the one prod node, got %+v", cfg.Nodes)
	}
	if _, err := resolveFleetTarget(fleetTarget{nodeName: "nope"}); err == nil {
		t.Error("an unregistered name with no fleet should fail")
	}
}

// Beside a discovered fleet the flag's node narrows the fan-out to it alone;
// a name the fleet holds resolves as the environment; a name nothing answers
// fails naming both places it looked.
func TestResolveTargetNarrowsDiscoveredFleet(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	mustWrite(t, filepath.Join(dir, fleet.DefaultFile),
		"nodes:\n  - name: gpu-a\n    host: a.local\n  - name: gpu-b\n    host: b.local\n")

	cfg, err := resolveFleetTarget(fleetTarget{nodeName: "gpu-a"})
	if err != nil {
		t.Fatalf("--node gpu-a beside a fleet holding it: %v", err)
	}
	narrowed := narrowedTarget(cfg, "gpu-a")
	if len(narrowed.Nodes) != 1 || narrowed.Nodes[0].Name != "gpu-a" {
		t.Errorf("want only gpu-a, got %+v", narrowed.Nodes)
	}

	registerTargetEnv(t, "prod")
	cfg, err = resolveFleetTarget(fleetTarget{nodeName: "prod"})
	if err != nil {
		t.Fatalf("a registry name beside a fleet without it: %v", err)
	}
	if len(cfg.Nodes) != 1 || cfg.Nodes[0].Name != "prod" {
		t.Errorf("want the prod environment's node, got %+v", cfg.Nodes)
	}

	_, err = resolveFleetTarget(fleetTarget{nodeName: "ghost"})
	if err == nil {
		t.Fatal("a name the fleet and the registry both refuse should fail")
	}
	for _, want := range []string{"ghost", fleet.DefaultFile, "gpu-a", "gpu-b"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}
}
