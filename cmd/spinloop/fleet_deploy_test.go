package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/spinloop-ai/spinloop/internal/config"
	"github.com/spinloop-ai/spinloop/internal/remote"
)

// fleetDeployServer answers every environment's deploy call with a
// deterministic base URL, so a fan-out over several nodes can be told apart
// by the environment each request named.
func fleetDeployServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		env := r.URL.Query().Get("env")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"deployed":true,"environment":%q,"base_url":"http://198.51.100.9:8000/v1"}`, env)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// stubFleetDeploySeams points the deploy seams at server for every
// environment, reporting none as registered or live (so no node needs
// --overwrite) unless overridden by the caller after this returns.
func stubFleetDeploySeams(t *testing.T, server *httptest.Server) {
	t.Helper()
	origDiscover, origStatus, origDetect := deployDiscoverFn, remoteStatusFn, detectPublicCIDRFn
	t.Cleanup(func() { deployDiscoverFn, remoteStatusFn, detectPublicCIDRFn = origDiscover, origStatus, origDetect })
	deployDiscoverFn = func(context.Context, aws.Config, string) (remote.ControlPlane, error) {
		return remote.ControlPlane{Config: remote.Config{
			StartURL: server.URL, StopURL: server.URL, DeployURL: server.URL, Region: "us-east-1",
		}}, nil
	}
	remoteStatusFn = func(context.Context, remote.Config) (*remote.Response, error) {
		return &remote.Response{StatusCode: 200, State: "undeployed"}, nil
	}
	detectPublicCIDRFn = func(context.Context) (string, error) { return "203.0.113.7/32", nil }
}

// writeFleetDeploySetup lays out a fleet file exercising every resolution
// tier: gpu-a and gpu-b via an explicit file field, aliased via a
// registered alias, subdir-env via a same-named subdirectory, no-source via
// none of the three, plus a kind: daemon node (studio) fleet deploy must
// never touch. Returns the fleet directory (already the working directory).
func writeFleetDeploySetup(t *testing.T) string {
	t.Helper()
	isolateConfig(t)
	dir := writeFleetFile(t, `
nodes:
  - name: gpu-a
    kind: remote
    file: ./gpu-a.Spinloop
  - name: gpu-b
    kind: remote
    file: ./gpu-b.Spinloop
  - name: aliased
    kind: remote
  - name: subdir-env
    kind: remote
  - name: no-source
    kind: remote
  - name: studio
    host: studio.local
`)
	write := func(name, env string) {
		t.Helper()
		body := fmt.Sprintf("PROVIDER llamacpp\nMODEL org/m:Q4\nCONTEXT 8192\nREMOTE %s\n", env)
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("gpu-a.Spinloop", "gpu-a")
	write("gpu-b.Spinloop", "gpu-b")

	aliasedPath := filepath.Join(dir, "aliased.Spinloop")
	write("aliased.Spinloop", "aliased")
	if err := config.Update(func(f *config.File) error {
		f.SetAlias("aliased", aliasedPath)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := os.Mkdir(filepath.Join(dir, "subdir-env"), 0o700); err != nil {
		t.Fatal(err)
	}
	subdirBody := "PROVIDER llamacpp\nMODEL org/m:Q4\nCONTEXT 8192\nREMOTE subdir-env\n"
	if err := os.WriteFile(filepath.Join(dir, "subdir-env", "Spinloop"), []byte(subdirBody), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestCmdFleetDeployAll(t *testing.T) {
	writeFleetDeploySetup(t)
	stubAWSEnv(t)
	server := fleetDeployServer(t)
	stubFleetDeploySeams(t, server)

	// no-source can never resolve, so --all still fails overall — but must
	// still deploy every node that *does* resolve.
	err := cmdFleet([]string{"deploy", "--all"})
	if err == nil {
		t.Fatal("--all should fail overall because no-source cannot resolve")
	}

	for _, env := range []string{"gpu-a", "gpu-b", "aliased", "subdir-env"} {
		if _, statErr := os.Stat(mustEnvConfigPath(t, env)); statErr != nil {
			t.Errorf("environment %q was not registered: %v", env, statErr)
		}
	}
	if !strings.Contains(err.Error(), "no-source") {
		t.Errorf("error should mention the unresolved node, got %v", err)
	}
}

func TestCmdFleetDeployNamedNodes(t *testing.T) {
	writeFleetDeploySetup(t)
	stubAWSEnv(t)
	server := fleetDeployServer(t)
	stubFleetDeploySeams(t, server)

	if err := cmdFleet([]string{"deploy", "gpu-a", "gpu-b"}); err != nil {
		t.Fatalf("deploy gpu-a gpu-b: %v", err)
	}
	for _, env := range []string{"gpu-a", "gpu-b"} {
		if _, statErr := os.Stat(mustEnvConfigPath(t, env)); statErr != nil {
			t.Errorf("environment %q was not registered: %v", env, statErr)
		}
	}
	// Untargeted nodes must be left alone.
	if _, statErr := os.Stat(mustEnvConfigPath(t, "aliased")); statErr == nil {
		t.Error("aliased was deployed despite not being named")
	}
}

func TestCmdFleetDeployNoTargetIsAnError(t *testing.T) {
	writeFleetDeploySetup(t)
	err := cmdFleet([]string{"deploy"})
	if err == nil {
		t.Fatal("fleet deploy with no node and no --all was accepted")
	}
	for _, want := range []string{"gpu-a", "gpu-b", "aliased"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q should list the remote nodes, missing %q", err, want)
		}
	}
	// studio (kind: daemon) must not be offered as a deploy target.
	if strings.Contains(err.Error(), "studio") {
		t.Errorf("error %q should not list the daemon node", err)
	}
}

func TestCmdFleetDeployAllPlusNamesIsAmbiguous(t *testing.T) {
	writeFleetDeploySetup(t)
	err := cmdFleet([]string{"deploy", "--all", "gpu-a"})
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("want an ambiguous-target error, got %v", err)
	}
}

func TestCmdFleetDeployUnknownNodeFailsBeforeDeploying(t *testing.T) {
	writeFleetDeploySetup(t)
	stubAWSEnv(t)
	server := fleetDeployServer(t)
	stubFleetDeploySeams(t, server)

	err := cmdFleet([]string{"deploy", "gpu-a", "nope"})
	if err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("want an unknown-node error naming it, got %v", err)
	}
	if _, statErr := os.Stat(mustEnvConfigPath(t, "gpu-a")); statErr == nil {
		t.Error("gpu-a was deployed even though the command should have failed before touching anything")
	}
}

func TestCmdFleetDeployNamingADaemonNodeFails(t *testing.T) {
	writeFleetDeploySetup(t)
	err := cmdFleet([]string{"deploy", "studio"})
	if err == nil || !strings.Contains(err.Error(), "studio") {
		t.Fatalf("want an error naming studio, got %v", err)
	}
}

func TestCmdFleetDeployUnresolvedNodeFailsOnlyThatNode(t *testing.T) {
	writeFleetDeploySetup(t)
	stubAWSEnv(t)
	server := fleetDeployServer(t)
	stubFleetDeploySeams(t, server)

	out := captureStdout(t, func() {
		err := cmdFleet([]string{"deploy", "gpu-a", "no-source"})
		if err == nil {
			t.Fatal("want a failure because no-source cannot resolve")
		}
		if !strings.Contains(err.Error(), "no-source") {
			t.Errorf("error should name the failed node, got %v", err)
		}
	})
	if !strings.Contains(out, "gpu-a") {
		t.Errorf("output should still show gpu-a's deploy: %s", out)
	}
	if _, statErr := os.Stat(mustEnvConfigPath(t, "gpu-a")); statErr != nil {
		t.Errorf("gpu-a should still have deployed despite no-source failing: %v", statErr)
	}
}

func TestCmdFleetDeployAliasWinsOverSubdirectory(t *testing.T) {
	dir := writeFleetDeploySetup(t)
	stubAWSEnv(t)
	server := fleetDeployServer(t)
	stubFleetDeploySeams(t, server)

	// Register an alias under the subdir-env node's own name too, pointing
	// at a *different* Spinloop (a different REMOTE), and confirm the alias
	// wins.
	altPath := filepath.Join(dir, "alt.Spinloop")
	if err := os.WriteFile(altPath, []byte("PROVIDER llamacpp\nMODEL org/m:Q4\nCONTEXT 8192\nREMOTE alt-env\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := config.Update(func(f *config.File) error {
		f.SetAlias("subdir-env", altPath)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := cmdFleet([]string{"deploy", "subdir-env"}); err != nil {
		t.Fatalf("deploy subdir-env: %v", err)
	}
	if _, statErr := os.Stat(mustEnvConfigPath(t, "alt-env")); statErr != nil {
		t.Errorf("the alias's environment (alt-env) should have been used: %v", statErr)
	}
	if _, statErr := os.Stat(mustEnvConfigPath(t, "subdir-env")); statErr == nil {
		t.Error("the subdirectory's environment should not have been used once an alias exists")
	}
}

func TestCmdFleetDeployGuardDoesNotBlockSiblings(t *testing.T) {
	writeFleetDeploySetup(t)
	stubAWSEnv(t)
	server := fleetDeployServer(t)
	stubFleetDeploySeams(t, server)

	if err := remote.SaveEnvironment("gpu-a", remote.Config{StartURL: "https://s", StopURL: "https://x", Region: "us-east-1"}); err != nil {
		t.Fatal(err)
	}

	err := cmdFleet([]string{"deploy", "gpu-a", "gpu-b"})
	if err == nil {
		t.Fatal("want a failure because gpu-a is guarded")
	}
	if !strings.Contains(err.Error(), "gpu-a") {
		t.Errorf("error should name the guarded node, got %v", err)
	}
	if _, statErr := os.Stat(mustEnvConfigPath(t, "gpu-b")); statErr != nil {
		t.Errorf("gpu-b should still have deployed despite gpu-a being guarded: %v", statErr)
	}
}

func TestCmdFleetDeployDryRunTouchesNothing(t *testing.T) {
	writeFleetDeploySetup(t)

	called := false
	origDiscover := deployDiscoverFn
	t.Cleanup(func() { deployDiscoverFn = origDiscover })
	deployDiscoverFn = func(context.Context, aws.Config, string) (remote.ControlPlane, error) {
		called = true
		return remote.ControlPlane{}, fmt.Errorf("must not be called")
	}

	out := captureStdout(t, func() {
		if err := cmdFleet([]string{"deploy", "gpu-a", "gpu-b", "--dry-run"}); err != nil {
			t.Errorf("deploy --dry-run: %v", err)
		}
	})
	if called {
		t.Error("--dry-run must touch nothing — not even discovery")
	}
	for _, want := range []string{"gpu-a", "gpu-b", "environment: gpu-a", "environment: gpu-b"} {
		if !strings.Contains(out, want) {
			t.Errorf("dry-run output missing %q:\n%s", want, out)
		}
	}
}

// mustEnvConfigPath resolves where a deployed environment would be
// registered, under the isolated config dir this test's HOME points at.
func mustEnvConfigPath(t *testing.T, env string) string {
	t.Helper()
	path, err := remote.EnvConfigPath(env)
	if err != nil {
		t.Fatal(err)
	}
	return path
}
