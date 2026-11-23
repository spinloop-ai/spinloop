package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/spinloop-ai/spinloop/internal/cloud"
	"github.com/spinloop-ai/spinloop/internal/config"
)

// deployedNodes records which cloud nodes a deploy was asked to create, in the
// order asked. It replaces the assertion that a registry file appeared: the
// plane is the record of what is deployed now, so what a deploy did is what it
// sent to the plane.
type deployedNodes struct {
	mu   sync.Mutex
	envs []string
}

func (d *deployedNodes) add(env string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.envs = append(d.envs, env)
}

func (d *deployedNodes) has(env string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.Contains(d.envs, env)
}

func (d *deployedNodes) all() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.Clone(d.envs)
}

// fleetDeployServer answers every node's deploy call with a
// deterministic base URL, so a fan-out over several nodes can be told apart by
// the cloud node each request named, and records which it was asked for.
func fleetDeployServer(t *testing.T) (*httptest.Server, *deployedNodes) {
	t.Helper()
	deployed := &deployedNodes{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		env := r.URL.Query().Get("env")
		deployed.add(env)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"deployed":true,"environment":%q,"base_url":"http://198.51.100.9:8000/v1"}`, env)
	}))
	t.Cleanup(srv.Close)
	return srv, deployed
}

// stubFleetDeploySeams points the deploy seams at server for every node.
// The plane holds the cloud nodes given — none by default, so no node needs
// --overwrite — and no node's instance is live, unless the caller overrides the
// seams again after this returns.
func stubFleetDeploySeams(t *testing.T, server *httptest.Server, holds ...cloud.Node) {
	t.Helper()
	origDiscover, origStatus, origDetect, origPlane, origInventory :=
		deployDiscoverFn, cloudStatusFn, detectPublicCIDRFn, deployResolvePlaneFn, cloudInventoryFn
	t.Cleanup(func() {
		deployDiscoverFn, cloudStatusFn, detectPublicCIDRFn = origDiscover, origStatus, origDetect
		deployResolvePlaneFn, cloudInventoryFn = origPlane, origInventory
	})
	plane := cloud.ControlPlane{Config: cloud.Config{
		StartURL: server.URL, StopURL: server.URL, DeployURL: server.URL, StatsURL: server.URL,
		Region: "us-east-1",
	}}
	deployDiscoverFn = func(context.Context, aws.Config, string) (cloud.ControlPlane, error) {
		return plane, nil
	}
	deployResolvePlaneFn = func(context.Context, cloud.PlaneRequest) (cloud.ControlPlane, error) {
		return plane, nil
	}
	cloudInventoryFn = func(context.Context, cloud.Config) ([]cloud.Node, error) {
		return holds, nil
	}
	cloudStatusFn = func(context.Context, cloud.Config) (*cloud.Response, error) {
		return &cloud.Response{StatusCode: 200, State: "undeployed"}, nil
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
    kind: cloud
    file: ./gpu-a.Spinloop
  - name: gpu-b
    kind: cloud
    file: ./gpu-b.Spinloop
  - name: aliased
    kind: cloud
  - name: subdir-env
    kind: cloud
  - name: no-source
    kind: cloud
  - name: studio
    host: studio.local
`)
	// The node's own name is the cloud node its deploy creates on the plane;
	// the Spinloop carries only the model.
	write := func(name string) {
		t.Helper()
		body := "PROVIDER llamacpp\nMODEL org/m:Q4\nCONTEXT 8192\n"
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("gpu-a.Spinloop")
	write("gpu-b.Spinloop")

	aliasedPath := filepath.Join(dir, "aliased.Spinloop")
	write("aliased.Spinloop")
	if err := config.Update(func(f *config.File) error {
		f.SetAlias("aliased", aliasedPath)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := os.Mkdir(filepath.Join(dir, "subdir-env"), 0o700); err != nil {
		t.Fatal(err)
	}
	subdirBody := "PROVIDER llamacpp\nMODEL org/m:Q4\nCONTEXT 8192\n"
	if err := os.WriteFile(filepath.Join(dir, "subdir-env", "Spinloop"), []byte(subdirBody), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestCmdFleetDeployAll(t *testing.T) {
	writeFleetDeploySetup(t)
	stubAWSEnv(t)
	server, deployed := fleetDeployServer(t)
	stubFleetDeploySeams(t, server)

	// no-source can never resolve, so --all still fails overall — but must
	// still deploy every node that *does* resolve.
	err := cmdFleet([]string{"deploy", "--all"})
	if err == nil {
		t.Fatal("--all should fail overall because no-source cannot resolve")
	}

	for _, env := range []string{"gpu-a", "gpu-b", "aliased", "subdir-env"} {
		if !deployed.has(env) {
			t.Errorf("cloud node %q was never deployed (deployed: %v)", env, deployed.all())
		}
	}
	if !strings.Contains(err.Error(), "no-source") {
		t.Errorf("error should mention the unresolved node, got %v", err)
	}
}

func TestCmdFleetDeployNamedNodes(t *testing.T) {
	writeFleetDeploySetup(t)
	stubAWSEnv(t)
	server, deployed := fleetDeployServer(t)
	stubFleetDeploySeams(t, server)

	if err := cmdFleet([]string{"deploy", "gpu-a", "gpu-b"}); err != nil {
		t.Fatalf("deploy gpu-a gpu-b: %v", err)
	}
	for _, env := range []string{"gpu-a", "gpu-b"} {
		if !deployed.has(env) {
			t.Errorf("cloud node %q was never deployed (deployed: %v)", env, deployed.all())
		}
	}
	// Untargeted nodes must be left alone.
	if deployed.has("aliased") {
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
			t.Errorf("error %q should list the cloud nodes, missing %q", err, want)
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
	server, deployed := fleetDeployServer(t)
	stubFleetDeploySeams(t, server)

	err := cmdFleet([]string{"deploy", "gpu-a", "nope"})
	if err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("want an unknown-node error naming it, got %v", err)
	}
	if deployed.has("gpu-a") {
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
	server, deployed := fleetDeployServer(t)
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
	if !deployed.has("gpu-a") {
		t.Errorf("gpu-a should still have deployed despite no-source failing (deployed: %v)", deployed.all())
	}
}

// A node whose source *resolves* but whose Spinloop fails to parse (it still
// carries a REMOTE line, here) must fail only that node — resolveNodeSpinloop
// succeeding is not the same as deriveDeployTarget succeeding, and the two
// failure sites must not be conflated.
func TestCmdFleetDeployResolvedButUndeployableSpinloopFailsOnlyThatNode(t *testing.T) {
	dir := writeFleetDeploySetup(t)
	stubAWSEnv(t)
	server, deployed := fleetDeployServer(t)
	stubFleetDeploySeams(t, server)

	// gpu-a's own file, with a REMOTE line — readSpinloop rejects it, but
	// resolveNodeSpinloop has already succeeded by the time it does.
	staleCloud := filepath.Join(dir, "gpu-a.Spinloop")
	if err := os.WriteFile(staleCloud, []byte("PROVIDER llamacpp\nMODEL org/m:Q4\nCONTEXT 8192\nREMOTE gpu-a\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() {
		err := cmdFleet([]string{"deploy", "gpu-a", "gpu-b"})
		if err == nil {
			t.Fatal("want a failure because gpu-a's Spinloop still carries a REMOTE line")
		}
		if !strings.Contains(err.Error(), "gpu-a") {
			t.Errorf("error should name gpu-a, got %v", err)
		}
	})
	if !strings.Contains(out, "REMOTE") {
		t.Errorf("output should explain the removed REMOTE instruction: %s", out)
	}
	if !deployed.has("gpu-b") {
		t.Errorf("gpu-b should still have deployed despite gpu-a's Spinloop being undeployable (deployed: %v)", deployed.all())
	}
}

func TestCmdFleetDeployAliasWinsOverSubdirectory(t *testing.T) {
	dir := writeFleetDeploySetup(t)
	stubAWSEnv(t)
	server, deployed := fleetDeployServer(t)
	stubFleetDeploySeams(t, server)

	// Register an alias under the subdir-env node's own name too, pointing
	// at a *different* Spinloop, and confirm the alias wins.
	altPath := filepath.Join(dir, "alt.Spinloop")
	if err := os.WriteFile(altPath, []byte("PROVIDER llamacpp\nMODEL org/other:Q4\nCONTEXT 8192\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := config.Update(func(f *config.File) error {
		f.SetAlias("subdir-env", altPath)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() {
		if err := cmdFleet([]string{"deploy", "subdir-env"}); err != nil {
			t.Fatalf("deploy subdir-env: %v", err)
		}
	})
	// The deploy names the Spinloop source it used — the alias's target, not
	// the subdirectory's file.
	if !strings.Contains(out, "alt.Spinloop") || !strings.Contains(out, `alias "subdir-env"`) {
		t.Errorf("the alias's Spinloop should have been used, got:\n%s", out)
	}
	if !deployed.has("subdir-env") {
		t.Errorf("the node's own cloud node was never deployed (deployed: %v)", deployed.all())
	}
}

func TestCmdFleetDeployGuardDoesNotBlockSiblings(t *testing.T) {
	writeFleetDeploySetup(t)
	stubAWSEnv(t)
	server, deployed := fleetDeployServer(t)
	// gpu-a's deployer is a colleague: the plane holds the name, so this
	// machine needs --overwrite for it, while gpu-b is still free.
	stubFleetDeploySeams(t, server, cloud.Node{
		Name: "gpu-a", DeployedBy: "colleague@example", State: "stopped",
	})

	err := cmdFleet([]string{"deploy", "gpu-a", "gpu-b"})
	if err == nil {
		t.Fatal("want a failure because gpu-a is guarded")
	}
	if !strings.Contains(err.Error(), "gpu-a") {
		t.Errorf("error should name the guarded node, got %v", err)
	}
	if !deployed.has("gpu-b") {
		t.Errorf("gpu-b should still have deployed despite gpu-a being guarded (deployed: %v)", deployed.all())
	}
}

func TestCmdFleetDeployDryRunTouchesNothing(t *testing.T) {
	writeFleetDeploySetup(t)

	called := false
	origDiscover, origPlane := deployDiscoverFn, deployResolvePlaneFn
	t.Cleanup(func() { deployDiscoverFn, deployResolvePlaneFn = origDiscover, origPlane })
	boom := func(context.Context, cloud.PlaneRequest) (cloud.ControlPlane, error) {
		called = true
		return cloud.ControlPlane{}, fmt.Errorf("must not be called")
	}
	deployResolvePlaneFn = boom
	deployDiscoverFn = func(context.Context, aws.Config, string) (cloud.ControlPlane, error) {
		return boom(context.Background(), cloud.PlaneRequest{})
	}

	out := captureStdout(t, func() {
		if err := cmdFleet([]string{"deploy", "gpu-a", "gpu-b", "--dry-run"}); err != nil {
			t.Errorf("deploy --dry-run: %v", err)
		}
	})
	if called {
		t.Error("--dry-run must touch nothing — not even the plane")
	}
	for _, want := range []string{"gpu-a", "gpu-b", "node: gpu-a", "node: gpu-b"} {
		if !strings.Contains(out, want) {
			t.Errorf("dry-run output missing %q:\n%s", want, out)
		}
	}
}

// A node's instance-type is per-node: `fleet deploy` threads each targeted
// node's own value into its deploy, so one node's machine never leaks to a
// sibling's plan — a node naming a type shows it, one naming none shows the
// control plane's default.
func TestCmdFleetDeployInstanceTypePerNode(t *testing.T) {
	isolateConfig(t)
	dir := writeFleetFile(t, `
nodes:
  - name: typed
    kind: cloud
    file: ./typed.Spinloop
    instance-type: g6e.2xlarge
  - name: untyped
    kind: cloud
    file: ./untyped.Spinloop
`)
	writeSpinloop := func(name string) {
		t.Helper()
		body := "PROVIDER llamacpp\nMODEL org/m:Q4\nCONTEXT 8192\n"
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	writeSpinloop("typed.Spinloop")
	writeSpinloop("untyped.Spinloop")

	boom := func(context.Context, cloud.PlaneRequest) (cloud.ControlPlane, error) {
		return cloud.ControlPlane{}, fmt.Errorf("must not be called")
	}
	deployResolvePlaneFn = boom
	deployDiscoverFn = func(context.Context, aws.Config, string) (cloud.ControlPlane, error) {
		return boom(context.Background(), cloud.PlaneRequest{})
	}
	t.Cleanup(func() {
		deployResolvePlaneFn = cloud.ResolvePlane
		deployDiscoverFn = cloud.DiscoverControlPlane
	})

	// --dry-run touches nothing, so no deploy seams need stubbing.
	outTyped := captureStdout(t, func() {
		if err := cmdFleet([]string{"deploy", "typed", "--dry-run"}); err != nil {
			t.Errorf("deploy typed --dry-run: %v", err)
		}
	})
	if !strings.Contains(outTyped, "instance:  g6e.2xlarge") {
		t.Errorf("typed node's plan should name its instance type, got:\n%s", outTyped)
	}

	outUntyped := captureStdout(t, func() {
		if err := cmdFleet([]string{"deploy", "untyped", "--dry-run"}); err != nil {
			t.Errorf("deploy untyped --dry-run: %v", err)
		}
	})
	if !strings.Contains(outUntyped, "instance:  the control plane's default") {
		t.Errorf("untyped node's plan should state the default machine, got:\n%s", outUntyped)
	}
}
