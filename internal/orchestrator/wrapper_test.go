package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWrapCommand_NoLifecycleRunsTheHarnessDirectly(t *testing.T) {
	bin, args, env := wrapCommand("opencode", []string{"run", "-m", "x"}, HarnessConfig{})
	if bin != "opencode" || len(args) != 3 || env != nil {
		t.Errorf("no startup or shutdown named should run the harness directly, got bin=%q args=%v env=%v", bin, args, env)
	}
}

func TestWrapCommand_ALifecycleScriptWrapsTheHarness(t *testing.T) {
	bin, args, env := wrapCommand("opencode", []string{"run", "-m", "x"}, HarnessConfig{Startup: "echo hi"})
	if bin != "/bin/sh" {
		t.Errorf("a lifecycle script should run the wrapper, got bin=%q", bin)
	}
	found := false
	for _, e := range env {
		if e == "STARTUP=echo hi" {
			found = true
		}
	}
	if !found {
		t.Errorf("STARTUP should carry the script, got env=%v", env)
	}
	// The harness bin and its args trail the wrapper's own -c arguments,
	// unchanged, so "$@" inside the script sees them.
	want := []string{"-c", wrapperScript, "spinloop-wrapper", "opencode", "run", "-m", "x"}
	if len(args) != len(want) {
		t.Fatalf("args = %v, want %v", args, want)
	}
	for i, w := range want {
		if args[i] != w {
			t.Errorf("args[%d] = %q, want %q", i, args[i], w)
		}
	}
}

func TestDispatch_WrapperRunsStartupThenHarnessThenShutdown(t *testing.T) {
	work := t.TempDir()
	record := filepath.Join(work, "record")
	t.Setenv("RECORD_FILE", record)
	t.Setenv("OPENAI_API_KEY", "")
	bin := stubAgent(t)

	h := &fakeHarness{name: "opencode", bin: bin}
	d := NewDispatcher(h, "http://gateway:4000", "the-token", false).
		WithHarnessConfig(HarnessConfig{Startup: "echo from-startup", Shutdown: "echo from-shutdown"})

	logPath := filepath.Join(work, "a.log")
	child, err := d.Launch(
		Item{ID: "a", Instructions: "fix the parser", Dir: work},
		runningNode("gpu-a", "org/model", nil),
		logPath,
	)
	if err != nil {
		t.Fatalf("the launch should succeed: %v", err)
	}
	if err := child.Wait(); err != nil {
		t.Fatalf("the agent should end cleanly: %v", err)
	}

	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	out := string(log)
	iStart := strings.Index(out, "from-startup")
	iAgent := strings.Index(out, "the agent's output")
	iShutdown := strings.Index(out, "from-shutdown")
	if iStart == -1 || iAgent == -1 || iShutdown == -1 {
		t.Fatalf("all three outputs should be in the log, got:\n%s", out)
	}
	if !(iStart < iAgent && iAgent < iShutdown) {
		t.Errorf("the outputs should be in order startup, harness, shutdown, got:\n%s", out)
	}
}

func TestDispatch_AFailingStartupScriptFailsBeforeTheHarnessRuns(t *testing.T) {
	work := t.TempDir()
	record := filepath.Join(work, "record")
	t.Setenv("RECORD_FILE", record)
	t.Setenv("OPENAI_API_KEY", "")
	bin := stubAgent(t)

	h := &fakeHarness{name: "opencode", bin: bin}
	d := NewDispatcher(h, "http://gateway:4000", "the-token", false).
		WithHarnessConfig(HarnessConfig{Startup: "exit 3", Shutdown: "echo from-shutdown"})

	logPath := filepath.Join(work, "a.log")
	child, err := d.Launch(
		Item{ID: "a", Instructions: "fix the parser", Dir: work},
		runningNode("gpu-a", "org/model", nil),
		logPath,
	)
	if err != nil {
		t.Fatalf("the launch itself should succeed — the failure is the wrapper's own exit: %v", err)
	}
	if err := child.Wait(); err == nil || !strings.Contains(err.Error(), "startup script") {
		t.Errorf("a failing startup script should fail naming it, got %v", err)
	}
	if _, err := os.Stat(record); !os.IsNotExist(err) {
		t.Error("the harness should never have run")
	}
	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(log), "from-shutdown") {
		t.Errorf("shutdown should still run even though startup failed, got:\n%s", log)
	}
}

func TestDispatch_AbortRunsShutdownAfterStoppingTheHarness(t *testing.T) {
	work := t.TempDir()
	t.Setenv("OPENAI_API_KEY", "")
	// A harness stand-in that ignores the default TERM disposition, traps
	// it, records that it arrived, and then exits — so the wrapper's
	// forwarded signal, not a default kill, is what ends it.
	bin := filepath.Join(work, "agent")
	script := "#!/bin/sh\n" +
		"trap 'echo got-term; exit 0' TERM\n" +
		"echo started\n" +
		"while true; do sleep 0.05; done\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	h := &fakeHarness{name: "opencode", bin: bin}
	d := NewDispatcher(h, "http://gateway:4000", "the-token", false).
		WithHarnessConfig(HarnessConfig{Shutdown: "echo from-shutdown"})

	logPath := filepath.Join(work, "a.log")
	child, err := d.Launch(
		Item{ID: "a", Instructions: "do", Dir: work},
		runningNode("gpu-a", "org/model", nil),
		logPath,
	)
	if err != nil {
		t.Fatalf("the launch should succeed: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		if data, _ := os.ReadFile(logPath); strings.Contains(string(data), "started") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the harness never started")
		}
		time.Sleep(5 * time.Millisecond)
	}

	child.Stop()
	if err := child.Wait(); err != nil {
		t.Fatalf("a clean stop should not be reported as a failure: %v", err)
	}

	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	out := string(log)
	if !strings.Contains(out, "got-term") {
		t.Errorf("the stop signal should reach the harness, got:\n%s", out)
	}
	if !strings.Contains(out, "from-shutdown") {
		t.Errorf("shutdown should still run after the stop, got:\n%s", out)
	}
}
