package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// directiveDirs is the completion directive asking the shell for directories
// only (cobra.ShellCompDirectiveFilterDirs).
const directiveDirs = ":16"

// stubHarnessRecordingDir puts a stand-in harness on PATH that writes its
// working directory and its arguments, one per line, to outFile.
func stubHarnessRecordingDir(t *testing.T, name, outFile string) {
	t.Helper()
	dir := t.TempDir()
	body := "#!/bin/sh\npwd -P > " + outFile + "\nprintf '%s\\n' \"$@\" >> " + outFile + "\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// launchRecord returns the working directory the stand-in harness ran in and
// the arguments it was given.
func launchRecord(t *testing.T, outFile string) (string, []string) {
	t.Helper()
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("the harness was not launched: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	return lines[0], lines[1:]
}

// realPath resolves symlinks, so a temp dir compares equal to what `pwd -P`
// reports (macOS's /var is a link to /private/var).
func realPath(t *testing.T, p string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestWorkingDir_HarnessRunsInTheNamedDirectory(t *testing.T) {
	isolateConfig(t)
	out := filepath.Join(t.TempDir(), "launch")
	stubHarnessRecordingDir(t, "opencode", out)
	project := t.TempDir()

	for _, args := range [][]string{
		{"-w", project, "run", "--continue"},
		{"--working-dir", project, "run", "--continue"},
		{"--working-dir=" + project, "run", "--continue"},
	} {
		if err := cmdCode(args); err != nil {
			t.Fatalf("cmdCode %v: %v", args, err)
		}
		dir, forwarded := launchRecord(t, out)
		if dir != realPath(t, project) {
			t.Errorf("%v: harness ran in %q, want %q", args, dir, project)
		}
		if strings.Join(forwarded, " ") != "run --continue" {
			t.Errorf("%v: harness got %q, want only `run --continue`", args, forwarded)
		}
	}
}

func TestWorkingDir_RelativePathResolvesAgainstSpinloopsDirectory(t *testing.T) {
	isolateConfig(t)
	out := filepath.Join(t.TempDir(), "launch")
	stubHarnessRecordingDir(t, "opencode", out)
	base := t.TempDir()
	if err := os.Mkdir(filepath.Join(base, "project"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(base)

	if err := cmdOpen([]string{"-w", "project"}); err != nil {
		t.Fatalf("cmdOpen: %v", err)
	}
	if dir, _ := launchRecord(t, out); dir != realPath(t, filepath.Join(base, "project")) {
		t.Errorf("harness ran in %q, want %s/project", dir, base)
	}
}

func TestWorkingDir_WithoutTheFlagTheHarnessRunsWhereSpinloopRuns(t *testing.T) {
	isolateConfig(t)
	out := filepath.Join(t.TempDir(), "launch")
	stubHarnessRecordingDir(t, "opencode", out)
	here := t.TempDir()
	t.Chdir(here)

	if err := cmdCode(nil); err != nil {
		t.Fatalf("cmdCode: %v", err)
	}
	if dir, _ := launchRecord(t, out); dir != realPath(t, here) {
		t.Errorf("harness ran in %q, want %q", dir, here)
	}
}

// -O still applies the Spinloop beside spinloop, not the one in the harness's
// directory.
func TestWorkingDir_SpinloopLookupIsUnchanged(t *testing.T) {
	home := isolateConfig(t)
	out := filepath.Join(t.TempDir(), "launch")
	stubHarnessRecordingDir(t, "opencode", out)
	here := t.TempDir()
	mustWrite(t, filepath.Join(here, "Spinloop"), "PROVIDER llamacpp\nMODEL from-here\n")
	project := t.TempDir()
	mustWrite(t, filepath.Join(project, "Spinloop"), "PROVIDER llamacpp\nMODEL from-project\n")
	t.Chdir(here)

	captureStdout(t, func() {
		if err := cmdCode([]string{"-O", "-w", project}); err != nil {
			t.Fatalf("cmdCode: %v", err)
		}
	})
	config, err := os.ReadFile(filepath.Join(home, ".config", "opencode", "opencode.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(config), "from-here") || strings.Contains(string(config), "from-project") {
		t.Errorf("applied the wrong Spinloop:\n%s", config)
	}
	if dir, _ := launchRecord(t, out); dir != realPath(t, project) {
		t.Errorf("harness ran in %q, want %q", dir, project)
	}
}

func TestWorkingDir_InvalidPathFailsBeforeApplying(t *testing.T) {
	home := isolateConfig(t)
	out := filepath.Join(t.TempDir(), "launch")
	stubHarnessRecordingDir(t, "opencode", out)
	spinloopPath := filepath.Join(t.TempDir(), "Spinloop")
	mustWrite(t, spinloopPath, "PROVIDER llamacpp\nMODEL gemma\n")
	file := filepath.Join(t.TempDir(), "notes.txt")
	mustWrite(t, file, "x")
	missing := filepath.Join(t.TempDir(), "missing")

	cases := map[string]string{
		missing: "does not exist",
		file:    "is not a directory",
	}
	for path, want := range cases {
		err := cmdCode([]string{"-w", path, spinloopPath})
		if err == nil || !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), want) {
			t.Errorf("-w %s: err = %v, want it to name the path and say %q", path, err, want)
		}
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "opencode", "opencode.json")); !os.IsNotExist(err) {
		t.Errorf("the harness config was written despite the bad directory (stat err %v)", err)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Error("the harness was launched despite the bad directory")
	}
}

func TestComplete_WorkingDirOffersDirectories(t *testing.T) {
	isolateConfig(t)
	registerSpinloop(t, "PROVIDER llamacpp\nALIAS qwen\n")

	for _, words := range [][]string{
		{"code", "-w", ""},
		{"code", "--working-dir", ""},
		{"harness", "open", "-w", ""},
		{"harness", "open", "--working-dir", "=", ""}, // bash splits on "="
		{"code", "--working-dir="},                    // zsh passes one word
		{"code", "--working-dir=../pro"},
	} {
		got, directive := completeQuiet(t, words...)
		if len(got) != 0 {
			t.Errorf("%v: offered %v, want no static candidates", words, got)
		}
		if directive != directiveDirs {
			t.Errorf("%v: directive = %q, want %q (directories only)", words, directive, directiveDirs)
		}
	}
}

// The flag consumes its value, so the Spinloop slot after it is still open.
func TestComplete_SpinloopSlotAfterWorkingDir(t *testing.T) {
	isolateConfig(t)
	registerSpinloop(t, "PROVIDER llamacpp\nALIAS qwen\n")

	got, directive := complete(t, "code", "-w", "../project", "")
	if !hasAll(got, "qwen") {
		t.Errorf("aliases missing after -w <dir>: %v", got)
	}
	if directive != directiveFile {
		t.Errorf("directive = %q, want %q", directive, directiveFile)
	}
}

func TestComplete_WorkingDirFlagIsOffered(t *testing.T) {
	isolateConfig(t)
	for _, prefix := range []string{"--work", "-"} {
		got, _ := complete(t, "code", prefix)
		want := "--working-dir"
		if prefix == "-" {
			want = "-w"
		}
		if !hasAll(got, want) {
			t.Errorf("completing %q: %s missing from %v", prefix, want, got)
		}
	}
}
