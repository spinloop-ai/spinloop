package main

import (
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/spinloop-ai/spinloop/internal/spinloop"
)

// upFleet writes a one-node fleet.yaml (and the node's Spinloop) in a temp dir
// pointing at srv, chdirs there, and returns the dir — so up resolves
// ./fleet.yaml the way a user would.
func upFleet(t *testing.T, srv *httptest.Server) string {
	t.Helper()
	t.Setenv("SPINLOOP_CONFIG_DIR", t.TempDir())
	host, port := hostPort(t, srv)
	dir := writeFleetFile(t, fmt.Sprintf(
		"nodes:\n  - name: one\n    host: %s\n    port: %d\n    file: ./one.Spinloop\n",
		host, port))
	mustWrite(t, filepath.Join(dir, "one.Spinloop"), "PROVIDER llamacpp\nMODEL org/m:Q4\n")
	return dir
}

// upServeDir chdirs into an empty temp dir with an isolated spinloop config
// and returns it — a directory with no fleet file and, at first, no Spinloop.
func upServeDir(t *testing.T) string {
	t.Helper()
	t.Setenv("SPINLOOP_CONFIG_DIR", t.TempDir())
	dir := t.TempDir()
	t.Chdir(dir)
	return dir
}

// TestCmdUp_FleetDir_BareStartsEveryNode checks the fleet branch's default:
// a bare up starts the whole fleet, where a bare fleet start refuses to run.
func TestCmdUp_FleetDir_BareStartsEveryNode(t *testing.T) {
	upFleet(t, stubNode(t, "stopped"))
	out := captureStdout(t, func() {
		if err := cmdUp(nil); err != nil {
			t.Fatalf("up: %v", err)
		}
	})
	if !strings.Contains(out, "one: using") || !strings.Contains(out, "one  running") {
		t.Errorf("the node was not started:\n%s", out)
	}
}

func TestCmdUp_FleetDir_NamedNode(t *testing.T) {
	upFleet(t, stubNode(t, "stopped"))
	out := captureStdout(t, func() {
		if err := cmdUp([]string{"one"}); err != nil {
			t.Fatalf("up one: %v", err)
		}
	})
	if !strings.Contains(out, "one: using") || !strings.Contains(out, "one  running") {
		t.Errorf("the node was not started:\n%s", out)
	}
}

func TestCmdUp_FleetDir_UnknownNode(t *testing.T) {
	upFleet(t, stubNode(t, "stopped"))
	err := cmdUp([]string{"nope"})
	if err == nil || !strings.Contains(err.Error(), `no node "nope"`) ||
		!strings.Contains(err.Error(), "one") {
		t.Fatalf("want the unknown-node error naming the known nodes, got %v", err)
	}
}

// TestCmdUp_FleetWinsOverSpinloop checks the dispatch order: a Spinloop beside
// the fleet file is ignored, and the fleet is started. If the serve branch ran
// instead, it would look for a real engine binary and fail.
func TestCmdUp_FleetWinsOverSpinloop(t *testing.T) {
	dir := upFleet(t, stubNode(t, "stopped"))
	mustWrite(t, filepath.Join(dir, spinloop.DefaultFile), "PROVIDER llamacpp\nMODEL org/m:Q4\n")
	out := captureStdout(t, func() {
		if err := cmdUp(nil); err != nil {
			t.Fatalf("up: %v", err)
		}
	})
	if !strings.Contains(out, "one: using") {
		t.Errorf("the fleet was not started:\n%s", out)
	}
}

// TestCmdUp_ServesTheDirectorySpinloop checks the serve branch from a bare up:
// the engine the Spinloop names is actually launched.
func TestCmdUp_ServesTheDirectorySpinloop(t *testing.T) {
	dir := upServeDir(t)
	mustWrite(t, filepath.Join(dir, spinloop.DefaultFile), "PROVIDER llamacpp\nMODEL org/m:Q4\n")
	argsFile := filepath.Join(t.TempDir(), "args")
	stubLlamaServer(t, argsFile)
	if err := cmdUp(nil); err != nil {
		t.Fatalf("up: %v", err)
	}
	if _, err := os.Stat(argsFile); err != nil {
		t.Fatalf("the engine was not run: %v", err)
	}
}

func TestCmdUp_ServesAPath(t *testing.T) {
	upServeDir(t)
	spinloopPath := filepath.Join(t.TempDir(), spinloop.DefaultFile)
	mustWrite(t, spinloopPath, "PROVIDER llamacpp\nMODEL org/m:Q4\n")
	argsFile := filepath.Join(t.TempDir(), "args")
	stubLlamaServer(t, argsFile)
	if err := cmdUp([]string{spinloopPath}); err != nil {
		t.Fatalf("up %s: %v", spinloopPath, err)
	}
	if _, err := os.Stat(argsFile); err != nil {
		t.Fatalf("the engine was not run: %v", err)
	}
}

// TestCmdUp_ResolvesTheEnvironmentAlias checks a directory with no ./Spinloop
// but a SPINLOOP_ALIAS: up resolves it the way serve does, so a directory
// where serve works is a directory where up works.
func TestCmdUp_ResolvesTheEnvironmentAlias(t *testing.T) {
	upServeDir(t)
	registerSpinloop(t, "PROVIDER llamacpp\nALIAS q3\nMODEL org/m:Q4\n")
	t.Setenv("SPINLOOP_ALIAS", "q3")
	argsFile := filepath.Join(t.TempDir(), "args")
	stubLlamaServer(t, argsFile)
	if err := cmdUp(nil); err != nil {
		t.Fatalf("up: %v", err)
	}
	if _, err := os.Stat(argsFile); err != nil {
		t.Fatalf("the engine was not run: %v", err)
	}
}

func TestCmdUp_ResolvesARegisteredAlias(t *testing.T) {
	upServeDir(t)
	registerSpinloop(t, "PROVIDER llamacpp\nALIAS q3\nMODEL org/m:Q4\n")
	argsFile := filepath.Join(t.TempDir(), "args")
	stubLlamaServer(t, argsFile)
	if err := cmdUp([]string{"q3"}); err != nil {
		t.Fatalf("up q3: %v", err)
	}
	if _, err := os.Stat(argsFile); err != nil {
		t.Fatalf("the engine was not run: %v", err)
	}
}

// TestCmdUp_FailsWhenNothingResolves checks the neither case: no fleet file and
// no resolvable Spinloop fails with serve's own error, not one of up's own.
func TestCmdUp_FailsWhenNothingResolves(t *testing.T) {
	upServeDir(t)
	err := cmdUp(nil)
	if err == nil || !strings.Contains(err.Error(), "no Spinloop found") {
		t.Fatalf("want serve's no-Spinloop error, got %v", err)
	}
}

func TestUpSlot_FleetDirOffersNodeNames(t *testing.T) {
	upFleet(t, stubNode(t, "stopped"))
	cands, dir := upSlot(nil, nil, "")
	if dir != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive = %v, want NoFileComp", dir)
	}
	if !slices.Contains(cands, "one") {
		t.Errorf("node name not offered: %v", cands)
	}
}

func TestUpSlot_OutsideFleetOffersTheSpinloopSlot(t *testing.T) {
	upServeDir(t)
	cands, dir := upSlot(nil, nil, "")
	// The Spinloop slot allows paths (Default) and offers no node names.
	if dir != cobra.ShellCompDirectiveDefault {
		t.Errorf("directive = %v, want Default", dir)
	}
	if slices.Contains(cands, "one") {
		t.Errorf("a node name leaked into the Spinloop slot: %v", cands)
	}
}

func TestUpSlot_UnreadableFleetFileStaysSilent(t *testing.T) {
	dir := upServeDir(t)
	mustWrite(t, filepath.Join(dir, "fleet.yaml"), "nodes: [")
	cands, d := upSlot(nil, nil, "")
	if cands != nil || d != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("got (%v, %v), want (nil, NoFileComp)", cands, d)
	}
}
