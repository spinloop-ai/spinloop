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
	itemsPath := filepath.Join(work, "work.yaml")
	l := NewDockerLauncher(h, "http://gateway:4000", "the-token", image, itemsPath, false)
	rec := &dockerRunRecorder{}
	l.run = rec.run
	return l, rec, work
}

func TestDockerLaunch_RendersAScopedConfigPerItem(t *testing.T) {
	h := testDockerHarness(t)
	l, _, work := testDockerLauncher(t, h, "spinloop/agent:test")

	itemA := Item{ID: "a", Instructions: "do a", Dir: work}
	itemB := Item{ID: "b", Instructions: "do b", Dir: work}
	node := runningNode("n", "org/model", nil)

	if _, err := l.Launch(itemA, node, filepath.Join(work, "a.log")); err != nil {
		t.Fatalf("Launch a: %v", err)
	}
	if _, err := l.Launch(itemB, node, filepath.Join(work, "b.log")); err != nil {
		t.Fatalf("Launch b: %v", err)
	}

	dirA := ConfigDirFor(l.itemsPath, "a")
	dirB := ConfigDirFor(l.itemsPath, "b")
	if dirA == dirB {
		t.Fatalf("each item should get its own config directory, both got %s", dirA)
	}
	dataA, err := os.ReadFile(filepath.Join(dirA, "opencode.json"))
	if err != nil {
		t.Fatalf("item a's config should be on disk: %v", err)
	}
	if !strings.Contains(string(dataA), "spinloop-orchestrator-n") {
		t.Errorf("the rendered config should carry the node's provider, got %s", dataA)
	}
	if _, err := os.Stat(filepath.Join(dirB, "opencode.json")); err != nil {
		t.Errorf("item b's config should be on disk: %v", err)
	}
}

func TestDockerLaunch_BuildsTheExpectedInvocation(t *testing.T) {
	h := testDockerHarness(t)
	l, rec, work := testDockerLauncher(t, h, "spinloop/agent:test")
	l.harnessConfig = HarnessConfig{Env: map[string]string{"FOO": "bar"}}

	item := Item{ID: "a", Instructions: "fix it", Dir: work}
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
		"run", "--rm", "--network host",
		"-v " + mustAbs(t, work) + ":/workspace",
		"-w /workspace",
		"-e OPENAI_API_KEY=the-token",
		"-e FOO=bar",
		"spinloop/agent:test",
		"opencode run -m spinloop-orchestrator-gpu-1/org/model --auto fix it",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("the invocation should carry %q, got:\n%s", want, joined)
		}
	}
	if call.containerName != "spinloop-a" {
		t.Errorf("containerName = %q, want spinloop-a", call.containerName)
	}
	if call.logPath != logPath {
		t.Errorf("logPath = %q, want %q", call.logPath, logPath)
	}
	// The config mount lands where opencode resolves its own config under
	// the image's fixed home.
	if !strings.Contains(joined, ":"+dockerHome+"/.config/opencode") {
		t.Errorf("the config mount should target the image's opencode config dir, got:\n%s", joined)
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
