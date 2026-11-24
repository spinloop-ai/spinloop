package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spinloop-ai/spinloop/internal/inference"
)

// stubUnsloth points the Unsloth CLI at a script that records its argv, so no
// Unsloth Studio install is needed to pin the command serve builds for it.
func stubUnsloth(t *testing.T, argsFile string) {
	t.Helper()
	stubServer(t, &unslothBinary, "unsloth", argsFile)
}

// serveDryRun writes a Spinloop with body into a fresh directory and returns
// what `serve --dry-run` prints for it.
func serveDryRun(t *testing.T, body string) string {
	t.Helper()
	spinloopPath := filepath.Join(t.TempDir(), "Spinloop")
	mustWrite(t, spinloopPath, body)
	return captureStdout(t, func() {
		if err := cmdServe([]string{"--dry-run", spinloopPath}); err != nil {
			t.Fatalf("cmdServe: %v", err)
		}
	})
}

// TestCmdServe_UnslothDryRun pins the whole Unsloth shape: the `run`
// subcommand, the model reference verbatim as --model, the context length,
// the slot count, a bind address taken from BASEURL, and the two defaults.
func TestCmdServe_UnslothDryRun(t *testing.T) {
	stubUnsloth(t, filepath.Join(t.TempDir(), "args"))
	out := serveDryRun(t,
		"PROVIDER unsloth\nMODEL unsloth/Qwen3.8-27B-GGUF:UD-Q4_K_XL\nALIAS qwen\nCONTEXT 128k\nPARALLEL 2\nBASEURL http://127.0.0.1:8899/v1\n")

	for _, want := range []string{
		"unsloth run --api-only --disable-tools",
		"--model unsloth/Qwen3.8-27B-GGUF:UD-Q4_K_XL",
		"--context-length 128000",
		"--parallel 2",
		"--host 127.0.0.1",
		"--port 8899",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("command missing %q:\n%s", want, out)
		}
	}
	// Studio has no served-name flag, and the context is not scaled by the
	// slot count the way llama.cpp's is.
	for _, unwanted := range []string{"qwen ", "--alias", "--ctx-size", "256000", "--hf-repo"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("command should not contain %q:\n%s", unwanted, out)
		}
	}
}

// TestCmdServe_UnslothDefaultsCanBeSwitchedOff checks the defaults sit under
// a PARAM, so a falsy value drops them.
func TestCmdServe_UnslothDefaultsCanBeSwitchedOff(t *testing.T) {
	stubUnsloth(t, filepath.Join(t.TempDir(), "args"))
	out := serveDryRun(t, "PROVIDER unsloth\nMODEL org/model\nPARAM disable-tools = false\n")

	if strings.Contains(out, "--disable-tools") {
		t.Errorf("disable-tools = false should drop the default:\n%s", out)
	}
	if !strings.Contains(out, "--api-only") {
		t.Errorf("the other default should stand:\n%s", out)
	}
}

// TestCmdServe_UnslothDefaultsUnderPreset checks a preset can switch a
// default off too, and that PARALLEL overrides the preset's own slot count
// however the preset spells it.
func TestCmdServe_UnslothDefaultsUnderPreset(t *testing.T) {
	stubUnsloth(t, filepath.Join(t.TempDir(), "args"))
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "preset.ini"), `[*]
api-only = 0
np = 4

[qwen]
hf = unsloth/Qwen3.8-27B-GGUF:UD-Q4_K_XL
max-seq-length = 8192
`)
	spinloopPath := filepath.Join(dir, "Spinloop")
	mustWrite(t, spinloopPath, "PROVIDER unsloth\nALIAS qwen\nPRESET preset.ini\nPARALLEL 2\n")

	out := captureStdout(t, func() {
		if err := cmdServe([]string{"--dry-run", spinloopPath}); err != nil {
			t.Fatalf("cmdServe: %v", err)
		}
	})

	for _, want := range []string{
		"--disable-tools",
		"--model unsloth/Qwen3.8-27B-GGUF:UD-Q4_K_XL",
		"--context-length 8192",
		"--parallel 2",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("command missing %q:\n%s", want, out)
		}
	}
	for _, unwanted := range []string{"--api-only", "--parallel 4", "--np", "--hf "} {
		if strings.Contains(out, unwanted) {
			t.Errorf("command should not contain %q:\n%s", unwanted, out)
		}
	}
}

// TestCmdServe_UnslothParamNamesModel checks a PARAM in any of Studio's
// spellings of the model flag stands in for MODEL.
func TestCmdServe_UnslothParamNamesModel(t *testing.T) {
	stubUnsloth(t, filepath.Join(t.TempDir(), "args"))
	out := serveDryRun(t, "PROVIDER unsloth\nPARAM hf = unsloth/Qwen3.8-27B-GGUF:UD-Q4_K_XL\n")

	if !strings.Contains(out, "--model unsloth/Qwen3.8-27B-GGUF:UD-Q4_K_XL") {
		t.Errorf("the PARAM should render as --model:\n%s", out)
	}
}

// TestCmdServe_UnslothNeedsModel checks Unsloth will not start without being
// told what to load.
func TestCmdServe_UnslothNeedsModel(t *testing.T) {
	spinloopPath := filepath.Join(t.TempDir(), "Spinloop")
	mustWrite(t, spinloopPath, "PROVIDER unsloth\n")
	if err := cmdServe([]string{"--dry-run", spinloopPath}); err == nil {
		t.Error("expected error when there is neither a PRESET nor a MODEL")
	}
}

// TestCmdServe_UnslothRunsTheStub checks the real launch path hands the
// built argv to the binary.
func TestCmdServe_UnslothRunsTheStub(t *testing.T) {
	argsFile := filepath.Join(t.TempDir(), "args")
	stubUnsloth(t, argsFile)
	spinloopPath := filepath.Join(t.TempDir(), "Spinloop")
	mustWrite(t, spinloopPath, "PROVIDER unsloth\nMODEL ./models/qwen.gguf\n")

	captureStdout(t, func() {
		if err := cmdServe([]string{spinloopPath}); err != nil {
			t.Fatalf("cmdServe: %v", err)
		}
	})
	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Fields(string(data))
	want := []string{"run", "--api-only", "--disable-tools", "--model", "./models/qwen.gguf"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("argv = %v, want %v", got, want)
	}
}

// TestResolveUnslothBinary covers the lookup order: the override, then the
// PATH, then the installer's own location.
func TestResolveUnslothBinary(t *testing.T) {
	orig := unslothBinary
	t.Cleanup(func() { unslothBinary = orig })

	unslothBinary = "/custom/unsloth"
	if got := resolveUnslothBinary(); got != "/custom/unsloth" {
		t.Errorf("override: got %q", got)
	}

	unslothBinary = ""
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", t.TempDir())
	want := filepath.Join(home, unslothInstallRel)
	if got := resolveUnslothBinary(); got != want {
		t.Errorf("install location: got %q, want %q", got, want)
	}

	bin := t.TempDir()
	mustWrite(t, filepath.Join(bin, "unsloth"), "#!/bin/sh\n")
	if err := os.Chmod(filepath.Join(bin, "unsloth"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	if got := resolveUnslothBinary(); got != filepath.Join(bin, "unsloth") {
		t.Errorf("PATH: got %q", got)
	}
}

// TestUnslothDeployConfigArgv checks a daemon-pushed config builds the same
// command shape, defaults included.
func TestUnslothDeployConfigArgv(t *testing.T) {
	stubUnsloth(t, filepath.Join(t.TempDir(), "args"))
	engine, err := engineFor("unsloth")
	if err != nil {
		t.Fatal(err)
	}
	argv, err := argvFromDeployConfig(engine, inference.DeployConfig{
		Runner: "unsloth", ModelID: "unsloth/Qwen3.8-27B-GGUF", Quant: "UD-Q4_K_XL",
		ContextSize: 32768, Parallel: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(argv[1:], " ")
	want := "run --api-only --disable-tools --model unsloth/Qwen3.8-27B-GGUF:UD-Q4_K_XL --context-length 32768 --parallel 2"
	if got != want {
		t.Errorf("argv = %q, want %q", got, want)
	}
}

// TestUnslothCannotBeGated checks a keyed start is refused: Studio has no
// option to read a key from a file.
func TestUnslothCannotBeGated(t *testing.T) {
	_, err := engineKeyArgs(&inference.DeployConfig{Runner: "unsloth"}, "/tmp/key")
	if err == nil || !strings.Contains(err.Error(), "cannot be gated") {
		t.Errorf("err = %v, want the cannot-be-gated refusal", err)
	}
}

// TestUnslothEndpointDefaults checks a Spinloop with no BASEURL is reported on
// Studio's own default port, loopback only.
func TestUnslothEndpointDefaults(t *testing.T) {
	engine, err := engineFor("unsloth")
	if err != nil {
		t.Fatal(err)
	}
	ep := engineEndpointFor(engine, "", []string{"unsloth", "run", "--model", "m"})
	if ep == nil || ep.Port != 8888 || !ep.LoopbackOnly || ep.RequiresKey {
		t.Errorf("endpoint = %+v, want port 8888, loopback, no key", ep)
	}
	target := scrapeTargetFor(engine, "", nil)
	if target.Engine != "" {
		t.Errorf("unsloth has no metrics dialect, got %q", target.Engine)
	}
}
