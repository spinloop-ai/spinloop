package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The command's help reads per the cli-ux conventions: a lowercase
// imperative, the flags the command's contract is built from, and the
// items file's default in place.
func TestOrchestratorHelpReadsPerTheCliConventions(t *testing.T) {
	isolateConfig(t)
	// The one-line description is a lowercase imperative phrase.
	short := orchestratorCmd().Short
	if short == "" || (rune(short[0]) >= 'A' && rune(short[0]) <= 'Z') || strings.HasSuffix(short, ".") {
		t.Errorf("the short description should be a lowercase phrase, got %q", short)
	}
	out := captureStdout(t, func() {
		_ = cmdOrchestrator([]string{"--help"})
	})
	for _, want := range []string{
		"--gateway",
		"--items",
		"./work.yaml",
		"--token-env",
		"OPENAI_API_KEY",
		"-f, --fleet",
		"-H, --harness",
		"--create-item-dirs",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the help should carry %q, got:\n%s", want, out)
		}
	}
}

// Neither a flag nor a file names a gateway: the refusal names both ways to
// give one, before anything is resolved.
func TestCmdOrchestrator_NeitherFlagNorFileNamesBothWays(t *testing.T) {
	isolateConfig(t)
	dir := t.TempDir()
	t.Chdir(dir)
	mustWrite(t, "work.yaml", "- id: a\n  instructions: do\n  dir: .\n")
	err := cmdOrchestrator(nil)
	if err == nil {
		t.Fatal("a run with no gateway should fail")
	}
	for _, want := range []string{"--gateway", "fleet.yaml"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal should name %q, got %v", want, err)
		}
	}
}

// A fleet file with no gateway section is refused, naming the file.
func TestCmdOrchestrator_AFleetFileWithoutAGatewaySectionNamesIt(t *testing.T) {
	isolateConfig(t)
	node := newRoutableNode(t, "qwen3-27b", true, 300)
	dir := t.TempDir()
	t.Chdir(dir)
	fleetFileIn(t, dir, "nodes:\n"+node.entry("gpu-box"))
	mustWrite(t, "work.yaml", "- id: a\n  instructions: do\n  dir: .\n")
	err := cmdOrchestrator(nil)
	if err == nil || !strings.Contains(err.Error(), "fleet.yaml") || !strings.Contains(err.Error(), "--gateway") {
		t.Errorf("a file without a gateway section should fail naming the file and the flag, got %v", err)
	}
}

// The section's token variable is honoured where the token flag was not
// given, and an explicit flag wins over the section.
func TestCmdOrchestrator_TheSectionsTokenVariableIsHonoured(t *testing.T) {
	isolateConfig(t)
	node := newRoutableNode(t, "qwen3-27b", true, 300)
	dir := t.TempDir()
	t.Chdir(dir)
	fleetFileIn(t, dir, "gateway:\n  name: the-fleet\n  url: http://gw:4000\n  tokenEnv: ORCH_FILE_TOKEN\nnodes:\n"+node.entry("gpu-box"))
	mustWrite(t, "work.yaml", "- id: a\n  instructions: do\n  dir: .\n")
	t.Setenv("ORCH_FILE_TOKEN", "")
	t.Setenv("GW_ORCH_TOKEN", "")
	t.Setenv("OPENAI_API_KEY", "")

	err := cmdOrchestrator(nil)
	if err == nil || !strings.Contains(err.Error(), "ORCH_FILE_TOKEN") {
		t.Errorf("without a token flag the section's variable should be in force, got %v", err)
	}
	err = cmdOrchestrator([]string{"--token-env", "GW_ORCH_TOKEN"})
	if err == nil || !strings.Contains(err.Error(), "GW_ORCH_TOKEN") {
		t.Errorf("an explicit token flag should win over the section, got %v", err)
	}
}

// The fleet file in the working directory names the gateway: with no
// gateway flag, the run works against the section's address, and the banner
// names it. The token sits only in the .env beside the file, the way a
// gateway-routed launch reads it. The item names a tag no node carries, so
// nothing is launched.
func TestCmdOrchestrator_AFleetFileNamesTheGateway(t *testing.T) {
	isolateConfig(t)
	t.Setenv("ORCH_FILE_TOKEN", "")
	t.Setenv("OPENAI_API_KEY", "")
	node := newRoutableNode(t, "qwen3-27b", true, 300)
	dir := t.TempDir()
	t.Chdir(dir)
	fleetFileIn(t, dir, "nodes:\n"+node.entry("gpu-box"))
	mustWrite(t, filepath.Join(dir, ".env"), "ORCH_FILE_TOKEN=the-token\n")
	mustWrite(t, "work.yaml", "- id: a\n  instructions: do\n  dir: .\n  tags:\n    - gpu=a100\n")

	srv, ln, err := newGatewayServer("", "127.0.0.1:0", "", "")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go srv.Serve(ln)
	// The file names the gateway the server just bound, so the command finds
	// it without a flag.
	fleetFileIn(t, dir, "gateway:\n  name: the-fleet\n  url: http://"+ln.Addr().String()+"\n  tokenEnv: ORCH_FILE_TOKEN\nnodes:\n"+node.entry("gpu-box"))

	stdout := filepath.Join(t.TempDir(), "stdout")
	f, err := os.Create(stdout)
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = f
	t.Cleanup(func() {
		os.Stdout = old
		f.Close()
	})

	done := make(chan error, 1)
	go func() { done <- cmdOrchestrator(nil) }()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		data, _ := os.ReadFile(stdout)
		if strings.Contains(string(data), "in the backlog") {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	data, _ := os.ReadFile(stdout)
	if !strings.Contains(string(data), "http://"+ln.Addr().String()) || !strings.Contains(string(data), "in the backlog") {
		t.Fatalf("the banner should name the gateway the fleet file gave, got:\n%s", data)
	}

	interruptSelf(t)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("the interrupt should end the run without an error, got %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("the interrupt did not end the run")
	}
}

// A token variable set nowhere is refused, naming the gateway and the
// variable, before an item is worked.
func TestCmdOrchestrator_AnUnsetTokenVariableNamesTheGatewayAndTheVariable(t *testing.T) {
	isolateConfig(t)
	t.Setenv("GW_ORCH_TOKEN", "")
	dir := t.TempDir()
	t.Chdir(dir)
	mustWrite(t, "work.yaml", "- id: a\n  instructions: do\n  dir: .\n")
	err := cmdOrchestrator([]string{"--gateway", "http://gw:4000", "--token-env", "GW_ORCH_TOKEN"})
	if err == nil {
		t.Fatal("an unset token variable should fail the run")
	}
	for _, want := range []string{"http://gw:4000", "GW_ORCH_TOKEN"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the failure should name %q, got %v", want, err)
		}
	}
}

// A harness without a single-task form is refused at startup, naming it,
// rather than as every item it would work.
func TestCmdOrchestrator_AHarnessWithoutASingleTaskFormNamesIt(t *testing.T) {
	isolateConfig(t)
	t.Setenv("OPENAI_API_KEY", "the-token")
	dir := t.TempDir()
	t.Chdir(dir)
	mustWrite(t, "work.yaml", "- id: a\n  instructions: do\n  dir: .\n")
	err := cmdOrchestrator([]string{"--gateway", "http://gw:4000", "--harness", "lucinate"})
	if err == nil || !strings.Contains(err.Error(), "lucinate") {
		t.Errorf("a harness without a single-task form should fail naming it, got %v", err)
	}
}

// A file that is not a list of items stops the command, naming the file,
// working no item.
func TestCmdOrchestrator_ANUnparseableItemsFileNamesIt(t *testing.T) {
	isolateConfig(t)
	t.Setenv("OPENAI_API_KEY", "the-token")
	dir := t.TempDir()
	t.Chdir(dir)
	mustWrite(t, "work.yaml", "not: [a list\n")
	err := cmdOrchestrator([]string{"--gateway", "http://gw:4000"})
	if err == nil || !strings.Contains(err.Error(), "work.yaml") {
		t.Errorf("an unparseable items file should fail naming the file, got %v", err)
	}
}

// A gateway that does not answer ends the run, naming the gateway, working
// no item: no state is recorded for the backlog.
func TestCmdOrchestrator_AGatewayThatDoesNotAnswerEndsTheRunNamingIt(t *testing.T) {
	isolateConfig(t)
	t.Setenv("OPENAI_API_KEY", "the-token")
	dir := t.TempDir()
	t.Chdir(dir)
	mustWrite(t, "work.yaml", "- id: a\n  instructions: do\n  dir: .\n")
	err := cmdOrchestrator([]string{"--gateway", "http://127.0.0.1:1"})
	if err == nil {
		t.Fatal("a gateway that does not answer should end the run")
	}
	if !strings.Contains(err.Error(), "http://127.0.0.1:1") {
		t.Errorf("the failure should name the gateway, got %v", err)
	}
	if _, err := os.Stat("work.yaml.state.json"); !os.IsNotExist(err) {
		t.Errorf("a run that worked no item records no state, got %v", err)
	}
}

// A gateway that refuses the token ends the run, naming the gateway and its
// answer, working no item.
func TestCmdOrchestrator_AGatewayThatRefusesTheTokenEndsTheRunNamingIt(t *testing.T) {
	isolateConfig(t)
	t.Setenv("OPENAI_API_KEY", "the-wrong-token")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"missing or invalid bearer token"}}`, http.StatusUnauthorized)
	}))
	defer srv.Close()
	dir := t.TempDir()
	t.Chdir(dir)
	mustWrite(t, "work.yaml", "- id: a\n  instructions: do\n  dir: .\n")
	err := cmdOrchestrator([]string{"--gateway", srv.URL})
	if err == nil {
		t.Fatal("a gateway that refuses the token should end the run")
	}
	if !strings.Contains(err.Error(), srv.URL) || !strings.Contains(err.Error(), "401") {
		t.Errorf("the failure should name the gateway and its answer, got %v", err)
	}
}

// The command works against a real gateway on loopback: the banner names the
// file and the gateway, the loop reads the topology, and the interrupt ends
// it cleanly. The item names a tag no node carries, so nothing is launched.
func TestCmdOrchestrator_WorksAgainstAGatewayOnLoopback(t *testing.T) {
	isolateConfig(t)
	t.Setenv("OPENAI_API_KEY", "the-token")
	node := newRoutableNode(t, "qwen3-27b", true, 300)
	dir := t.TempDir()
	fleetFileIn(t, dir, "nodes:\n"+node.entry("gpu-box"))
	t.Chdir(dir)
	mustWrite(t, "work.yaml", "- id: a\n  instructions: do\n  dir: .\n  tags:\n    - gpu=a100\n")

	srv, ln, err := newGatewayServer("", "127.0.0.1:0", "", "")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go srv.Serve(ln)

	// The banner goes to stdout; redirect it the way the gateway test does.
	stdout := filepath.Join(t.TempDir(), "stdout")
	f, err := os.Create(stdout)
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = f
	t.Cleanup(func() {
		os.Stdout = old
		f.Close()
	})

	done := make(chan error, 1)
	go func() { done <- cmdOrchestrator([]string{"--gateway", "http://" + ln.Addr().String()}) }()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		data, _ := os.ReadFile(stdout)
		if strings.Contains(string(data), "in the backlog") {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	data, _ := os.ReadFile(stdout)
	if !strings.Contains(string(data), "work.yaml") || !strings.Contains(string(data), "in the backlog") {
		t.Fatalf("the banner should name the items file and the backlog, got:\n%s", data)
	}

	// Give the loop a few passes to read the topology and find no match.
	time.Sleep(200 * time.Millisecond)
	if st, err := os.ReadFile("work.yaml.state.json"); err == nil && strings.Contains(string(st), `"running"`) {
		t.Fatalf("an item no node matches should not be launched, state:\n%s", st)
	}

	interruptSelf(t)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("the interrupt should end the run without an error, got %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("the interrupt did not end the run")
	}
}
