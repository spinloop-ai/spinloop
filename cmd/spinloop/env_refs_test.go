// Values in PARAM lines and preset entries resolve environment references
// ($FOO, ${FOO}, ${FOO:-bar}) when the command that consumes them builds its
// configuration — never when the Spinloop is parsed, shown or exported.

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spinloop-ai/spinloop/internal/config"
	"github.com/spinloop-ai/spinloop/internal/fleet"
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
	t.Setenv("DEPLOY_SEED", "7")
	dc := deployConfigFrom(t,
		"PROVIDER llamacpp\nALIAS qwen\nCONTEXT 32k\nPARAM hf = org/model:$HF_TAG\nPARAM seed = $DEPLOY_SEED\n", "")
	if dc.ModelID != "org/model" || dc.Quant != "Q8_0" {
		t.Errorf("model = %q:%q, want the resolved org/model:Q8_0", dc.ModelID, dc.Quant)
	}
	if args := strings.Join(dc.ServeArgs, " "); !strings.Contains(args, "--seed 7") {
		t.Errorf("serveArgs %q should carry the resolved PARAM value", args)
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

// On paths that apply the Spinloop's own environment before deriving (deploy
// and a node wake do), a PARAM value may reference a variable the Spinloop's
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

// Every scalar keyword resolves wherever a command consumes it: serve picks
// the engine by the resolved PROVIDER and builds its command from the
// resolved MODEL and CONTEXT; a malformed MODEL fails the command naming the
// keyword, and a preset path arrives from a variable.
func TestCmdServe_ScalarEnvRefs(t *testing.T) {
	t.Setenv("SERVE_ENGINE", "llamacpp")
	t.Setenv("HF_TAG", "Q4_K_M")
	t.Setenv("SERVE_CTX", "") // empty takes the default
	dir := t.TempDir()
	path := filepath.Join(dir, "Spinloop")
	mustWrite(t, path, "PROVIDER ${SERVE_ENGINE:-vllm}\nMODEL org/model:$HF_TAG\nCONTEXT ${SERVE_CTX:-32k}\n")

	out := captureStdout(t, func() {
		if err := cmdServe([]string{"--dry-run", path}); err != nil {
			t.Fatalf("serve --dry-run: %v", err)
		}
	})
	for _, want := range []string{"org/model:Q4_K_M", "--ctx-size 32000"} {
		if !strings.Contains(out, want) {
			t.Errorf("printed command is missing %q:\n%s", want, out)
		}
	}
	for _, raw := range []string{"$HF_TAG", "${SERVE_ENGINE", "${SERVE_CTX"} {
		if strings.Contains(out, raw) {
			t.Errorf("printed command leaks the raw reference %q:\n%s", raw, out)
		}
	}

	mustWrite(t, filepath.Join(dir, "Spinloop-bad"), "PROVIDER llamacpp\nMODEL org/model:${HF_TAG\n")
	if err := cmdServe([]string{"--dry-run", filepath.Join(dir, "Spinloop-bad")}); err == nil ||
		!strings.Contains(err.Error(), "MODEL") {
		t.Errorf("a malformed MODEL reference should fail naming MODEL, got %v", err)
	}
}

// A PRESET path may arrive from a variable: it resolves before its path is
// resolved against the Spinloop's source, and the narration names the
// resolved file.
func TestCmdServe_PresetPathFromVariable(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "preset.ini"), "[qwen]\nhf = org/m\nngl = 99\n")
	t.Setenv("SERVE_PRESET_DIR", dir)
	path := filepath.Join(dir, "Spinloop")
	mustWrite(t, path, "PROVIDER llamacpp\nALIAS qwen\nPRESET ${SERVE_PRESET_DIR}/preset.ini\n")

	out := captureStdout(t, func() {
		if err := cmdServe([]string{"--dry-run", path}); err != nil {
			t.Fatalf("serve --dry-run: %v", err)
		}
	})
	if !strings.Contains(out, "Using preset "+filepath.Join(dir, "preset.ini")) {
		t.Errorf("the narration should name the resolved preset path, got:\n%s", out)
	}
}

// The deploy derivation reads resolved scalars: a MODEL's repo:quant falls
// out of the resolved text, and a PRESET path from a variable loads the file
// it names — the chain deriveDeployTarget runs after the environment layers.
func TestDeployConfigFor_ScalarEnvRefs(t *testing.T) {
	t.Setenv("HF_TAG", "Q8_0")
	sel, path, err := readSpinloop("test", writeDeploySpinloop(t,
		"PROVIDER llamacpp\nALIAS qwen\nCONTEXT 32k\nMODEL org/model:$HF_TAG\n", ""))
	if err != nil {
		t.Fatal(err)
	}
	sel, err = resolveSelection(sel)
	if err != nil {
		t.Fatal(err)
	}
	dc, err := deployConfigFor(sel, path)
	if err != nil {
		t.Fatal(err)
	}
	if dc.ModelID != "org/model" || dc.Quant != "Q8_0" {
		t.Errorf("model = %q:%q, want the resolved org/model:Q8_0", dc.ModelID, dc.Quant)
	}

	dir := filepath.Dir(path)
	mustWrite(t, filepath.Join(dir, "preset.ini"), "[qwen]\nhf = other/model:Q4\n")
	t.Setenv("DEPLOY_PRESET_DIR", dir)
	sel, path, err = readSpinloop("test", writeDeploySpinloop(t,
		"PROVIDER llamacpp\nALIAS qwen\nCONTEXT 32k\nPRESET ${DEPLOY_PRESET_DIR}/preset.ini\n", ""))
	if err != nil {
		t.Fatal(err)
	}
	if sel, err = resolveSelection(sel); err != nil {
		t.Fatal(err)
	}
	if dc, err = deployConfigFor(sel, path); err != nil {
		t.Fatal(err)
	}
	if dc.ModelID != "other/model" {
		t.Errorf("model = %q, want the preset's resolved other/model", dc.ModelID)
	}
}

// A BASEURL resolves on the way into the harness config, and unapply removes
// the block apply wrote because it resolves the same way; the Spinloop file
// itself still carries the raw text.
func TestCmdApply_Unapply_ResolvesEnvRefs(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("APPLY_TEST_TAG", "v1")
	t.Setenv("APPLY_TEST_HOST", "gpu.internal")

	spinloopFile := filepath.Join(dir, "Spinloop")
	mustWrite(t, spinloopFile,
		"PROVIDER llamacpp\nMODEL org/model:${APPLY_TEST_TAG}\nBASEURL http://${APPLY_TEST_HOST:-localhost}:8080/v1\n")
	captureStdout(t, func() {
		if err := cmdApply([]string{spinloopFile}); err != nil {
			t.Fatalf("cmdApply: %v", err)
		}
	})

	cfgPath := filepath.Join(dir, "opencode", "opencode.json")
	m := readConfigMap(t, cfgPath)
	llamacpp := m["provider"].(map[string]any)["llamacpp"].(map[string]any)
	if got := llamacpp["options"].(map[string]any)["baseURL"]; got != "http://gpu.internal:8080/v1" {
		t.Errorf("baseURL = %v, want the resolved URL", got)
	}
	models := llamacpp["models"].(map[string]any)
	if _, ok := models["org/model:v1"]; !ok {
		t.Fatalf("the model should be keyed by the resolved org/model:v1, got %v", models)
	}
	if raw, err := os.ReadFile(spinloopFile); err != nil || !strings.Contains(string(raw), "$") {
		t.Errorf("the Spinloop file must keep its raw text, got %q, %v", raw, err)
	}

	captureStdout(t, func() {
		if err := cmdUnapply([]string{spinloopFile}); err != nil {
			t.Fatalf("cmdUnapply: %v", err)
		}
	})
	m = readConfigMap(t, cfgPath)
	if p, ok := m["provider"].(map[string]any)["llamacpp"]; ok {
		if models, ok := p.(map[string]any)["models"].(map[string]any); ok && len(models) > 0 {
			t.Errorf("unapply should have removed the resolved model, left %v", models)
		}
	}
}

// A registration name comes from the resolved ALIAS: the entry lives on this
// machine, where this machine's environment decides what the file means.
func TestCmdAlias_RegistersResolvedName(t *testing.T) {
	isolateConfig(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "Spinloop")
	mustWrite(t, path, "PROVIDER llamacpp\nMODEL org/m\nALIAS ${ALIAS_TEST_NICK:-unnamed}\n")
	t.Setenv("ALIAS_TEST_NICK", "qwen")

	captureStdout(t, func() {
		if err := cmdAlias([]string{path}); err != nil {
			t.Fatalf("cmdAlias: %v", err)
		}
	})
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	target, ok := cfg.Alias("qwen")
	if !ok {
		t.Fatalf("the alias should be registered under the resolved name, got %v", cfg.AliasNames())
	}
	if target != path {
		t.Errorf("alias target = %q, want %q", target, path)
	}
}

// Routing runs on the resolved selection (a launch resolves before it
// routes), so the selector wakes the node that can serve the model the file
// actually names.
func TestRouteResolvesTheModelItRoutesBy(t *testing.T) {
	t.Setenv("ROUTE_TEST_MODEL", "qwen3-27b")
	node := newRoutableNode(t, "qwen3-27b", true, 300)
	dir := t.TempDir()
	fleetPath := fleetFileIn(t, dir, "nodes:\n"+node.entry("gpu-box"))
	spinloopDir := t.TempDir()
	mustWrite(t, filepath.Join(spinloopDir, "Spinloop"), "PROVIDER llamacpp\nMODEL $ROUTE_TEST_MODEL\n")

	sel, path, err := readSpinloop("test", spinloopDir)
	if err != nil {
		t.Fatal(err)
	}
	if sel, err = resolveSelection(sel); err != nil {
		t.Fatal(err)
	}
	captureStderr(t, func() {
		c, err := routeThroughFleet(sel, path, routeOptions{fleetPath: fleetPath})
		if err != nil {
			t.Fatalf("routing failed: %v", err)
		}
		if c == nil || c.Node.Name != "gpu-box" {
			t.Errorf("routing by the resolved model should choose gpu-box, got %+v", c)
		}
	})
}

// The config a gateway wakes a node with is derived from the node's own
// Spinloop with its references resolved, so a node file parameterised by the
// machine loads the model it actually names.
func TestNodeDeployConfig_ResolvesEnvRefs(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "fleet.yaml"),
		"nodes:\n  - name: gpu-box\n    host: 127.0.0.1\n    port: 18080\n    file: Spinloop\n")
	mustWrite(t, filepath.Join(dir, "Spinloop"), "PROVIDER llamacpp\nMODEL org/model:$GATEWAY_TEST_TAG\n")
	t.Setenv("GATEWAY_TEST_TAG", "Q4_K_M")

	cfg, err := fleet.Resolve(filepath.Join(dir, "fleet.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := cfg.Node("gpu-box")
	if !ok {
		t.Fatal("gpu-box missing from the fleet")
	}
	dc, err := nodeDeployConfig(cfg, entry)
	if err != nil {
		t.Fatalf("nodeDeployConfig: %v", err)
	}
	if dc.ModelID != "org/model" || dc.Quant != "Q4_K_M" {
		t.Errorf("model = %q:%q, want the resolved org/model:Q4_K_M", dc.ModelID, dc.Quant)
	}
}
