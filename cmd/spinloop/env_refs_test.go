// Values in PARAM lines and preset entries resolve environment references
// ($FOO, ${FOO}, ${FOO:-bar}) when the command that consumes them builds its
// configuration — never when the Spinloop is parsed, shown or exported.

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spinloop-ai/spinloop/internal/spinloop"
)

const envRefPreset = `[*]
temp = ${SERVE_TEMP:-0.7}

[qwen]
hf    = unsloth/Qwen:$HF_TAG
ngl   = 99
price = $$5
`

// writeEnvRefSpinloop writes envRefPreset and a Spinloop referencing it into a
// fresh temp dir, returning the Spinloop path.
func writeEnvRefSpinloop(t *testing.T, spinloopBody string) string {
	t.Helper()
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "preset.ini"), envRefPreset)
	path := filepath.Join(dir, "Spinloop")
	mustWrite(t, path, spinloopBody)
	return path
}

// A preset value and a PARAM value both resolve into the printed command, and
// the resolved PARAM still stands over the resolved preset.
func TestCmdServe_EnvRefsResolve(t *testing.T) {
	t.Setenv("HF_TAG", "Q4_K_M")
	t.Setenv("SERVE_TEMP", "") // empty takes the preset's default
	t.Setenv("PARAM_TEMP", "1.0")
	path := writeEnvRefSpinloop(t, "PROVIDER llamacpp\nALIAS qwen\nPRESET preset.ini\nPARAM temp = $PARAM_TEMP\n")

	out := captureStdout(t, func() {
		if err := cmdServe([]string{"--dry-run", path}); err != nil {
			t.Fatalf("serve --dry-run: %v", err)
		}
	})
	// `$$5` resolves to `$5`, which the printed command quotes; the raw
	// spellings are gone.
	for _, want := range []string{"--hf-repo unsloth/Qwen:Q4_K_M", "--temp 1.0", "--price '$5'"} {
		if !strings.Contains(out, want) {
			t.Errorf("printed command is missing %q:\n%s", want, out)
		}
	}
	// The raw spellings never appear.
	for _, raw := range []string{"$HF_TAG", "$PARAM_TEMP", "${SERVE_TEMP"} {
		if strings.Contains(out, raw) {
			t.Errorf("printed command leaks the raw reference %q:\n%s", raw, out)
		}
	}
}

// A malformed PARAM reference fails serve naming the key; a malformed preset
// reference fails it naming the line. Neither starts an engine.
func TestCmdServe_EnvRefMalformed(t *testing.T) {
	dir := t.TempDir()

	badParam := filepath.Join(dir, "Spinloop-param")
	mustWrite(t, badParam, "PROVIDER llamacpp\nMODEL m\nPARAM hf = org/model:${HF_TAG\n")
	if err := cmdServe([]string{"--dry-run", badParam}); err == nil ||
		!strings.Contains(err.Error(), "PARAM hf") {
		t.Errorf("a malformed PARAM reference should fail naming the key, got %v", err)
	}

	mustWrite(t, filepath.Join(dir, "preset.ini"), "[qwen]\nhf = org/model:${HF_TAG:+x}\n")
	badPreset := filepath.Join(dir, "Spinloop-preset")
	mustWrite(t, badPreset, "PROVIDER llamacpp\nALIAS qwen\nPRESET preset.ini\n")
	if err := cmdServe([]string{"--dry-run", badPreset}); err == nil ||
		!strings.Contains(err.Error(), "line 2") {
		t.Errorf("a malformed preset reference should fail naming the line, got %v", err)
	}
}

// Deploy derives from resolved values: the model falls out of the resolved
// PARAM, and the resolved preset value rides into ServeArgs.
func TestDeployConfigFor_ResolvesEnvRefs(t *testing.T) {
	t.Setenv("HF_TAG", "Q8_0")
	t.Setenv("DEPLOY_TEMP", "1.0")
	dc := deployConfigFrom(t,
		"PROVIDER llamacpp\nALIAS qwen\nCONTEXT 32k\nPARAM hf = org/model:$HF_TAG\n", "")
	if dc.ModelID != "org/model" || dc.Quant != "Q8_0" {
		t.Errorf("model = %q:%q, want the resolved org/model:Q8_0", dc.ModelID, dc.Quant)
	}

	dc = deployConfigFrom(t,
		"PROVIDER llamacpp\nALIAS qwen\nCONTEXT 32k\nPRESET ./preset.ini\n",
		"[qwen]\nhf   = other/model:$HF_TAG\ntemp = ${DEPLOY_TEMP:-0.7}\n")
	if dc.ModelID != "other/model" || dc.Quant != "Q8_0" {
		t.Errorf("model = %q:%q, want the preset's resolved other/model:Q8_0", dc.ModelID, dc.Quant)
	}
	if args := strings.Join(dc.ServeArgs, " "); !strings.Contains(args, "--temp 1.0") {
		t.Errorf("serveArgs %q should carry the resolved preset value", args)
	}
}

// A malformed reference fails the derivation naming the PARAM key, so nothing
// is sent holding raw text.
func TestDeployConfigFor_EnvRefMalformed(t *testing.T) {
	path := writeDeploySpinloop(t,
		"PROVIDER llamacpp\nALIAS qwen\nCONTEXT 32k\nPARAM hf = org/model:${HF_TAG\n", "")
	sel, _, err := readSpinloop("test", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := deployConfigFor(sel, path); err == nil ||
		!strings.Contains(err.Error(), "PARAM hf") {
		t.Errorf("deployment should fail naming the PARAM key, got %v", err)
	}
}

// On paths that apply the Spinloop's own environment first (deploy, a
// supervised engine), a PARAM value may reference a variable the Spinloop's
// own ENV contributes.
func TestSelectionParams_EnvInstructionFeedsParam(t *testing.T) {
	sel, err := spinloop.Parse([]byte(
		"PROVIDER llamacpp\nMODEL org/m\nENV ENVREF_TEST_SLOT=3\nPARAM port = ${ENVREF_TEST_SLOT}000\n"))
	if err != nil {
		t.Fatal(err)
	}
	if err := applySpinloopEnv(sel, filepath.Join(t.TempDir(), "Spinloop")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Unsetenv("ENVREF_TEST_SLOT") })
	params, err := selectionParams(sel)
	if err != nil {
		t.Fatal(err)
	}
	if len(params) != 1 || params[0].Value != "3000" {
		t.Errorf("params = %+v, want the ENV-fed value 3000", params)
	}
}

// Parsing and rendering keep the raw text: a Spinloop carrying a reference
// round-trips through Format untouched, whatever the environment holds, so no
// show/export path can leak a resolved value.
func TestParseFormat_EnvRefsStayRaw(t *testing.T) {
	t.Setenv("HF_TAG", "Q8_0")
	sel, err := spinloop.Parse([]byte("PROVIDER llamacpp\nMODEL m\nPARAM hf = org/model:$HF_TAG\n"))
	if err != nil {
		t.Fatal(err)
	}
	if sel.Params[0].Value != "org/model:$HF_TAG" {
		t.Errorf("parsed value = %q, want the raw reference", sel.Params[0].Value)
	}
	out := spinloop.Format(sel)
	if !strings.Contains(out, "PARAM    hf = org/model:$HF_TAG") {
		t.Errorf("rendered Spinloop should keep the raw text, got:\n%s", out)
	}
}
