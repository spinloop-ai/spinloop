package orchestrator

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spinloop-ai/spinloop/internal/harness"
)

// dockerRunRecorder is the fake `docker run` seam dockerLauncher tests
// swap in: it records the argv each launch built, and answers with a
// fakeChild (or a launch failure) a test configures.
type dockerRunRecorder struct {
	calls []dockerRunCall
	err   error // returned in place of a Child where set
}

type dockerRunCall struct {
	args          []string
	containerName string
	logPath       string
}

func (r *dockerRunRecorder) run(args []string, containerName, logPath string) (Child, error) {
	r.calls = append(r.calls, dockerRunCall{args: args, containerName: containerName, logPath: logPath})
	if r.err != nil {
		return nil, r.err
	}
	return newFakeChild(nil, nil), nil
}

// testDockerHarness is the real opencode adapter: RenderProviderConfig
// never touches ConfigPath() (see harness_test.go's own coverage of
// that), so no env sandboxing is needed for it here the way Apply's tests
// need XDG_CONFIG_HOME.
func testDockerHarness(t *testing.T) harness.Harness {
	t.Helper()
	h, ok := harness.Lookup("opencode")
	if !ok {
		t.Fatal("opencode should be registered")
	}
	return h
}

func testDockerLauncher(t *testing.T, h harness.Harness, image string) (*dockerLauncher, *dockerRunRecorder, string) {
	t.Helper()
	work := t.TempDir()
	l := NewDockerLauncher(h, "http://gateway:4000", "the-token", image, false)
	rec := &dockerRunRecorder{}
	l.run = rec.run
	return l, rec, work
}

func TestDockerLaunch_RendersAScopedConfigPerItem(t *testing.T) {
	h := testDockerHarness(t)
	l, _, work := testDockerLauncher(t, h, "spinloop/agent:test")

	dirA := filepath.Join(work, "a")
	dirB := filepath.Join(work, "b")
	if err := os.MkdirAll(dirA, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dirB, 0o755); err != nil {
		t.Fatal(err)
	}
	itemA := Item{ID: "a", Instructions: "do a", Dir: dirA}
	itemB := Item{ID: "b", Instructions: "do b", Dir: dirB}
	node := runningNode("n", "org/model", nil)

	if _, err := l.Launch(itemA, node, filepath.Join(work, "a.log")); err != nil {
		t.Fatalf("Launch a: %v", err)
	}
	if _, err := l.Launch(itemB, node, filepath.Join(work, "b.log")); err != nil {
		t.Fatalf("Launch b: %v", err)
	}

	dataA, err := os.ReadFile(filepath.Join(ItemConfigDir(dirA), "opencode.json"))
	if err != nil {
		t.Fatalf("item a's config should be on disk: %v", err)
	}
	if !strings.Contains(string(dataA), "spinloop-orchestrator-n") {
		t.Errorf("the rendered config should carry the node's provider, got %s", dataA)
	}
	if _, err := os.Stat(filepath.Join(ItemConfigDir(dirB), "opencode.json")); err != nil {
		t.Errorf("item b's config should be on disk: %v", err)
	}
}

func TestDockerLaunch_TheKeptLogIsAlsoCopiedToTheItemsOwnDirectory(t *testing.T) {
	h := testDockerHarness(t)
	l, _, work := testDockerLauncher(t, h, "spinloop/agent:test")

	itemDir := filepath.Join(work, "a")
	if err := os.MkdirAll(itemDir, 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(work, "a.log")
	// runDockerContainer itself opens and writes logPath; the fake run
	// seam does not, so this stands in for the container's own output.
	if err := os.WriteFile(logPath, []byte("the container's output"), 0o600); err != nil {
		t.Fatal(err)
	}

	item := Item{ID: "a", Instructions: "do a", Dir: itemDir}
	child, err := l.Launch(item, runningNode("n", "org/model", nil), logPath)
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	if err := child.Wait(); err != nil {
		t.Fatalf("Wait: %v", err)
	}

	copied, err := os.ReadFile(ItemLogFile(itemDir))
	if err != nil {
		t.Fatalf("the item's own directory should carry a copy of the log: %v", err)
	}
	if string(copied) != "the container's output" {
		t.Errorf("the copy should match the canonical log, got %q", copied)
	}
}

func TestDockerLaunch_ABaseDirResolvesTheItemsRelativeDirectory(t *testing.T) {
	h := testDockerHarness(t)
	l, rec, work := testDockerLauncher(t, h, "spinloop/agent:test")
	l.WithBaseDir(work)
	if err := os.MkdirAll(filepath.Join(work, "parser"), 0o755); err != nil {
		t.Fatal(err)
	}

	item := Item{ID: "a", Instructions: "do a", Dir: "parser"}
	if _, err := l.Launch(item, runningNode("n", "org/model", nil), filepath.Join(work, "a.log")); err != nil {
		t.Fatalf("Launch: %v", err)
	}

	itemDir := filepath.Join(work, "parser")
	if _, err := os.Stat(filepath.Join(ItemWorkspaceDir(itemDir))); err != nil {
		t.Errorf("the workspace should be under baseDir/parser, got: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ItemConfigDir(itemDir), "opencode.json")); err != nil {
		t.Errorf("the config should be under baseDir/parser, got: %v", err)
	}
	call := rec.calls[0]
	if !strings.Contains(strings.Join(call.args, " "), mustAbs(t, itemDir)+"/workspace:/item/workspace") {
		t.Errorf("the mount should use baseDir/parser, got:\n%s", strings.Join(call.args, " "))
	}
}

func TestDockerLaunch_BuildsTheExpectedInvocation(t *testing.T) {
	h := testDockerHarness(t)
	l, rec, work := testDockerLauncher(t, h, "spinloop/agent:test")
	l.harnessConfig = HarnessConfig{Env: map[string]string{"FOO": "bar"}}

	itemDir := filepath.Join(work, "a")
	if err := os.MkdirAll(itemDir, 0o755); err != nil {
		t.Fatal(err)
	}
	item := Item{ID: "a", Instructions: "fix it", Dir: itemDir}
	node := runningNode("gpu-1", "org/model", nil)
	logPath := filepath.Join(work, "a.log")

	if _, err := l.Launch(item, node, logPath); err != nil {
		t.Fatalf("Launch: %v", err)
	}
	if len(rec.calls) != 1 {
		t.Fatalf("one docker run per launch, got %d", len(rec.calls))
	}
	call := rec.calls[0]
	joined := strings.Join(call.args, " ")

	for _, want := range []string{
		"run", "--rm",
		"--add-host host.docker.internal:host-gateway",
		"-v " + mustAbs(t, filepath.Join(itemDir, "workspace")) + ":/item/workspace",
		"-w /item/workspace",
		"-v " + mustAbs(t, filepath.Join(itemDir, "config")) + "/opencode.json:/item/config/opencode/opencode.json",
		"-e XDG_CONFIG_HOME=/item/config",
		"-e OPENAI_API_KEY=the-token",
		"-e FOO=bar",
		"spinloop/agent:test",
		"opencode run -m spinloop-orchestrator-gpu-1/org/model --auto fix it",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("the invocation should carry %q, got:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "--network") {
		t.Errorf("the invocation should not set --network, got:\n%s", joined)
	}
	if call.containerName != "spinloop-a" {
		t.Errorf("containerName = %q, want spinloop-a", call.containerName)
	}
	if call.logPath != logPath {
		t.Errorf("logPath = %q, want %q", call.logPath, logPath)
	}
}

// TestDockerLaunch_ALoopbackGatewayReachesTheContainerViaDockerInternal is
// the bug a fleet.yaml naming a loopback gateway (http://localhost:4000,
// http://127.0.0.1:4000) hits under the docker backend: the container is
// not the host, and --network host does not reliably put it there on
// Docker Desktop. The rendered config's base URL SHALL name
// host.docker.internal instead, never asking the operator to change their
// fleet file.
func TestDockerLaunch_ALoopbackGatewayReachesTheContainerViaDockerInternal(t *testing.T) {
	h := testDockerHarness(t)
	work := t.TempDir()
	l := NewDockerLauncher(h, "http://localhost:4000", "the-token", "spinloop/agent:test", false)
	rec := &dockerRunRecorder{}
	l.run = rec.run

	item := Item{ID: "a", Instructions: "do", Dir: work}
	if _, err := l.Launch(item, runningNode("n", "org/model", nil), filepath.Join(work, "a.log")); err != nil {
		t.Fatalf("Launch: %v", err)
	}

	rendered, err := os.ReadFile(filepath.Join(ItemConfigDir(work), "opencode.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rendered), "host.docker.internal:4000") {
		t.Errorf("the rendered config should reach the gateway via host.docker.internal, got:\n%s", rendered)
	}
	if strings.Contains(string(rendered), "localhost:4000") || strings.Contains(string(rendered), "127.0.0.1:4000") {
		t.Errorf("the rendered config should not carry the host's own loopback address, got:\n%s", rendered)
	}
}

// TestDockerLaunch_ARoutableGatewayIsUnchanged checks that a gateway
// already reachable from the container's own network — anything but the
// host's own loopback — is not rewritten.
func TestDockerLaunch_ARoutableGatewayIsUnchanged(t *testing.T) {
	h := testDockerHarness(t)
	l, _, work := testDockerLauncher(t, h, "spinloop/agent:test") // gateway: http://gateway:4000

	item := Item{ID: "a", Instructions: "do", Dir: work}
	if _, err := l.Launch(item, runningNode("n", "org/model", nil), filepath.Join(work, "a.log")); err != nil {
		t.Fatalf("Launch: %v", err)
	}

	rendered, err := os.ReadFile(filepath.Join(ItemConfigDir(item.Dir), "opencode.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rendered), "gateway:4000") {
		t.Errorf("a routable gateway should reach the config unchanged, got:\n%s", rendered)
	}
}

func TestDockerReachableGateway(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"http://localhost:4000", "http://host.docker.internal:4000"},
		{"http://127.0.0.1:4000/v1", "http://host.docker.internal:4000/v1"},
		{"http://[::1]:4000", "http://host.docker.internal:4000"},
		{"http://gateway.internal:4000", "http://gateway.internal:4000"},
		{"http://gateway:4000", "http://gateway:4000"},
	} {
		if got := dockerReachableGateway(tc.in); got != tc.want {
			t.Errorf("dockerReachableGateway(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestDefaultDockerImage(t *testing.T) {
	if got, want := DefaultDockerImage("v1.42.0"), "ghcr.io/spinloop-ai/agent:v1.42.0"; got != want {
		t.Errorf("DefaultDockerImage(v1.42.0) = %q, want %q", got, want)
	}
	// "dev" is what main.version defaults to off a release build; there is
	// no matching image tag for it, so the default falls back to latest.
	if got, want := DefaultDockerImage("dev"), "ghcr.io/spinloop-ai/agent:latest"; got != want {
		t.Errorf("DefaultDockerImage(dev) = %q, want %q", got, want)
	}
	if got, want := DefaultDockerImage(""), "ghcr.io/spinloop-ai/agent:latest"; got != want {
		t.Errorf("DefaultDockerImage(\"\") = %q, want %q", got, want)
	}
}

func TestDockerLaunch_AFailureFailsTheItemNamingIt(t *testing.T) {
	h := testDockerHarness(t)
	l, rec, work := testDockerLauncher(t, h, "spinloop/agent:test")
	rec.err = fmt.Errorf("starting the container: exec: \"docker\": executable file not found in $PATH")

	item := Item{ID: "a", Instructions: "do", Dir: work}
	_, err := l.Launch(item, runningNode("n", "org/m", nil), filepath.Join(work, "a.log"))
	if err == nil {
		t.Fatal("a docker failure should fail the launch")
	}
	if !strings.Contains(err.Error(), `item "a"`) || !strings.Contains(err.Error(), "docker") {
		t.Errorf("the failure should name the item and the docker command's own message, got %v", err)
	}
}

func TestDockerChild_WaitReturnsTheContainersExitCode(t *testing.T) {
	cmd := exec.Command("sh", "-c", "exit 3")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	c := &dockerChild{cmd: cmd, name: "irrelevant"}
	err := c.Wait()
	if err == nil {
		t.Fatal("a nonzero exit should be reported as an error")
	}
	var ec exitCoder
	if !errors.As(err, &ec) || ec.ExitCode() != 3 {
		t.Errorf("Wait's error should carry the exit code the way procChild's does, got %v", err)
	}
}

func TestDockerChild_StopAndKill(t *testing.T) {
	var calls [][]string
	orig := dockerCLI
	dockerCLI = func(args ...string) { calls = append(calls, append([]string{}, args...)) }
	defer func() { dockerCLI = orig }()

	c := &dockerChild{name: "spinloop-a"}
	c.Stop()
	c.Kill()

	if len(calls) != 2 {
		t.Fatalf("Stop and Kill should each run one docker command, got %d: %v", len(calls), calls)
	}
	if got := strings.Join(calls[0], " "); got != "stop --time 0 spinloop-a" {
		t.Errorf("Stop's argv = %q", got)
	}
	if got := strings.Join(calls[1], " "); got != "kill spinloop-a" {
		t.Errorf("Kill's argv = %q", got)
	}
}

func mustAbs(t *testing.T, p string) string {
	t.Helper()
	abs, err := filepath.Abs(p)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}
