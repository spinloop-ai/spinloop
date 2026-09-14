package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// codeFleetDir writes a fleet file and a Spinloop into a temp dir and moves
// the test into it: the command's defaults — the fleet.yaml beside it, the
// Spinloop beside it — are working-directory stories.
func codeFleetDir(t *testing.T, fleetBody, spinloopBody string) {
	t.Helper()
	dir := t.TempDir()
	if fleetBody != "" {
		mustWrite(t, filepath.Join(dir, "fleet.yaml"), fleetBody)
	}
	if spinloopBody != "" {
		mustWrite(t, filepath.Join(dir, "Spinloop"), spinloopBody)
	}
	t.Chdir(dir)
}

// No flags at all: the fleet file beside the command names a gateway, so the
// agent is pointed at it with the section's token, and no node is contacted.
func TestCodeFleetUsesTheFleetsGateway(t *testing.T) {
	home := isolateConfig(t)
	t.Setenv("GATEWAY_TOKEN", "gw-token")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENAI_BASE_URL", "")
	argsFile := filepath.Join(t.TempDir(), "args")
	envFile := filepath.Join(t.TempDir(), "env")
	stubHarnessBinaryWithEnv(t, argsFile, envFile)

	codeFleetDir(t,
		"nodes:\n  - name: dead\n    host: 127.0.0.1\n    port: 1\ngateway:\n  url: http://gw.internal:4000\n  tokenEnv: GATEWAY_TOKEN\n",
		"PROVIDER llamacpp\nMODEL qwen3-27b\n")
	stderr := captureStderr(t, func() {
		captureStdout(t, func() {
			if err := cmdCode([]string{"-O"}); err != nil {
				t.Fatalf("cmdCode: %v", err)
			}
		})
	})
	if !strings.Contains(stderr, "Routing at http://gw.internal:4000/v1 — the fleet file names a gateway") {
		t.Errorf("the choice should be reported on stderr before the launch, got:\n%s", stderr)
	}
	if _, err := os.ReadFile(argsFile); err != nil {
		t.Fatalf("the harness was not launched: %v", err)
	}
	data, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatal(err)
	}
	out := string(data)
	if !strings.Contains(out, "BASE=http://gw.internal:4000/v1") {
		t.Errorf("the agent's base URL should be the section's address with the prefix, got:\n%s", out)
	}
	if !strings.Contains(out, "KEY=gw-token") {
		t.Errorf("the agent should carry the token the section's variable holds, got:\n%s", out)
	}
	config, err := os.ReadFile(filepath.Join(home, ".config", "opencode", "opencode.json"))
	if err != nil {
		t.Fatalf("the harness config was not written: %v", err)
	}
	if !strings.Contains(string(config), "http://gw.internal:4000/v1") {
		t.Errorf("the applied provider's base URL should be the section's address, got:\n%s", config)
	}
}

// A fleet file naming no gateway routes to a node, as a fleet-routed launch
// would.
func TestCodeFleetRoutesToANode(t *testing.T) {
	node := newRoutableNode(t, "qwen3-27b", true, 300)
	isolateConfig(t)
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENAI_BASE_URL", "")
	argsFile := filepath.Join(t.TempDir(), "args")
	envFile := filepath.Join(t.TempDir(), "env")
	stubHarnessBinaryWithEnv(t, argsFile, envFile)

	codeFleetDir(t,
		"nodes:\n"+node.entry("gpu-box"),
		"PROVIDER llamacpp\nMODEL qwen3-27b\n")
	captureStdout(t, func() {
		if err := cmdCode([]string{"-O"}); err != nil {
			t.Fatalf("cmdCode: %v", err)
		}
	})
	data, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatal(err)
	}
	want := "BASE=http://127.0.0.1:" + strconv.Itoa(node.enginePort) + "/v1"
	if !strings.Contains(string(data), want) {
		t.Errorf("the agent's base URL should be the node's engine, got:\n%s", data)
	}
}

// -f beats the fleet.yaml beside the command: the section in the named file is
// where the agent is pointed, not the one in the working directory.
func TestCodeFleetFileFlagBeatsTheCwdFleet(t *testing.T) {
	isolateConfig(t)
	t.Setenv("OPENAI_API_KEY", "gw-token")
	t.Setenv("OPENAI_BASE_URL", "")
	argsFile := filepath.Join(t.TempDir(), "args")
	envFile := filepath.Join(t.TempDir(), "env")
	stubHarnessBinaryWithEnv(t, argsFile, envFile)

	fleetPath := fleetFileIn(t, t.TempDir(),
		"nodes:\n  - name: dead\n    host: 127.0.0.1\n    port: 1\ngateway:\n  url: http://b.internal:4000\n")
	codeFleetDir(t,
		"nodes:\n  - name: dead\n    host: 127.0.0.1\n    port: 1\ngateway:\n  url: http://a.internal:4000\n",
		"PROVIDER llamacpp\nMODEL qwen3-27b\n")
	captureStdout(t, func() {
		if err := cmdCode([]string{"-O", "-f", fleetPath}); err != nil {
			t.Fatalf("cmdCode: %v", err)
		}
	})
	data, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "BASE=http://b.internal:4000/v1") {
		t.Errorf("the command's file should be the one routed through, got:\n%s", data)
	}
}

// A named fleet file that does not exist: the failure names the file, not the
// Spinloop also missing — the file is what the launch was pointed at.
func TestCodeFleetNamesTheFleetFileItCannotRead(t *testing.T) {
	isolateConfig(t)
	t.Setenv("SPINLOOP_ALIAS", "")
	argsFile := filepath.Join(t.TempDir(), "args")
	stubHarnessBinary(t, "opencode", argsFile)

	t.Chdir(t.TempDir())
	captureStdout(t, func() {
		err := cmdCode([]string{"--fleet", "fleet.yaml"})
		if err == nil {
			t.Fatal("a launch with no Spinloop should fail")
		}
		if !strings.Contains(err.Error(), "fleet.yaml") {
			t.Errorf("the failure should name the fleet file it could not read, got:\n%v", err)
		}
	})
	if _, err := os.ReadFile(argsFile); err == nil {
		t.Error("the harness was launched with no fleet to route through")
	}
}

// Nothing serving, wake allowed: the command starts the idle node's engine and
// points the agent at it, the way a fleet-routed launch does — the wake
// timeout flag bounds the wait.
func TestCodeFleetWakesANode(t *testing.T) {
	node := newRoutableNode(t, "qwen3-27b", false, 0)
	isolateConfig(t)
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENAI_BASE_URL", "")
	argsFile := filepath.Join(t.TempDir(), "args")
	envFile := filepath.Join(t.TempDir(), "env")
	stubHarnessBinaryWithEnv(t, argsFile, envFile)

	codeFleetDir(t, "nodes:\n"+node.entry("idle-box"), "PROVIDER llamacpp\nMODEL qwen3-27b\n")
	stderr := captureStderr(t, func() {
		captureStdout(t, func() {
			if err := cmdCode([]string{"-O", "--wake-timeout", "5s"}); err != nil {
				t.Fatalf("cmdCode: %v", err)
			}
		})
	})
	if !node.started {
		t.Error("the idle node was not started")
	}
	if !strings.Contains(stderr, "Using idle-box at") {
		t.Errorf("the woken node should be named on stderr, got:\n%s", stderr)
	}
	data, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatalf("the harness was not launched: %v", err)
	}
	want := "BASE=http://127.0.0.1:" + strconv.Itoa(node.enginePort) + "/v1"
	if !strings.Contains(string(data), want) {
		t.Errorf("the agent's base URL should be the woken engine, got:\n%s", data)
	}
}

// --no-wake with nothing serving fails rather than starting an engine, and the
// refusal names the node and the flag to drop.
func TestCodeFleetNoWakeRefuses(t *testing.T) {
	node := newRoutableNode(t, "qwen3-27b", false, 0)
	isolateConfig(t)
	argsFile := filepath.Join(t.TempDir(), "args")
	stubHarnessBinary(t, "opencode", argsFile)

	codeFleetDir(t, "nodes:\n"+node.entry("idle-box"), "PROVIDER llamacpp\nMODEL qwen3-27b\n")
	captureStdout(t, func() {
		err := cmdCode([]string{"-O", "--no-wake"})
		if err == nil {
			t.Fatal("--no-wake with nothing serving should fail")
		}
		for _, want := range []string{"idle-box", "spinloop fleet start", "drop --no-wake"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("the refusal should say %q, got:\n%v", want, err)
			}
		}
	})
	if node.started {
		t.Error("--no-wake started an engine")
	}
	if _, err := os.ReadFile(argsFile); err == nil {
		t.Error("the harness was launched for a route that refused to wake")
	}
}

// newModelsGateway serves GET /v1/models in the OpenAI list shape, refusing a
// request whose bearer token does not match wantToken.
func newModelsGateway(t *testing.T, wantToken string, ids []string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer "+wantToken {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		data := make([]map[string]any, len(ids))
		for i, id := range ids {
			data[i] = map[string]any{"id": id, "object": "model"}
		}
		json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": data})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// gatewayKeyForTest is the provider key a gateway-only launch writes its
// config under for a gateway with no explicit gateway.name: mirrors
// GatewayConfig.Label() falling back to the address's host.
func gatewayKeyForTest(gw *httptest.Server) string {
	return gatewayProviderKey(strings.TrimPrefix(gw.URL, "http://"))
}

// No Spinloop given, no directory to find one beside, the fleet file names a
// gateway: the command does not fail. It configures opencode's
// openai-compatible provider at the gateway's address, its models populated
// from the gateway's live listing, and no default model.
func TestCodeFleetNoSpinloopUsesGatewayModels(t *testing.T) {
	home := isolateConfig(t)
	gw := newModelsGateway(t, "gw-token", []string{"m1", "m2"})
	t.Setenv("GATEWAY_TOKEN", "gw-token")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENAI_BASE_URL", "")
	argsFile := filepath.Join(t.TempDir(), "args")
	envFile := filepath.Join(t.TempDir(), "env")
	stubHarnessBinaryWithEnv(t, argsFile, envFile)

	codeFleetDir(t,
		"nodes:\n  - name: dead\n    host: 127.0.0.1\n    port: 1\ngateway:\n  url: "+gw.URL+"\n  tokenEnv: GATEWAY_TOKEN\n",
		"")
	captureStdout(t, func() {
		if err := cmdCode([]string{"--fleet", "fleet.yaml"}); err != nil {
			t.Fatalf("cmdCode: %v", err)
		}
	})
	if _, err := os.ReadFile(argsFile); err != nil {
		t.Fatalf("the harness was not launched: %v", err)
	}
	config, err := os.ReadFile(filepath.Join(home, ".config", "opencode", "opencode.json"))
	if err != nil {
		t.Fatalf("the harness config was not written: %v", err)
	}
	var root map[string]any
	if err := json.Unmarshal(config, &root); err != nil {
		t.Fatalf("opencode.json not valid JSON: %v", err)
	}
	if v, ok := root["model"]; ok {
		t.Errorf("no default model should be set, got: %v", v)
	}
	wantKey := gatewayKeyForTest(gw)
	provider, _ := root["provider"].(map[string]any)
	entry, ok := provider[wantKey].(map[string]any)
	if !ok {
		t.Fatalf("provider %q not found in %v", wantKey, provider)
	}
	models, _ := entry["models"].(map[string]any)
	for _, id := range []string{"m1", "m2"} {
		if _, ok := models[id]; !ok {
			t.Errorf("discovered model %q missing from provider.models: %v", id, models)
		}
	}
	opts, _ := entry["options"].(map[string]any)
	if baseURL, _ := opts["baseURL"].(string); baseURL != gw.URL+"/v1" {
		t.Errorf("baseURL = %q, want %q", baseURL, gw.URL+"/v1")
	}
	host := strings.TrimPrefix(gw.URL, "http://")
	if name, _ := entry["name"].(string); name != "Gateway ("+host+")" {
		t.Errorf("name = %q, want it to read like the remote-environment pattern (e.g. %q)", name, "Gateway ("+host+")")
	}
}

// The same launch with --harness pi: the discovered models land in Pi's own
// models.json provider entry instead.
func TestCodeFleetNoSpinloopUsesGatewayModelsForPi(t *testing.T) {
	home := isolateConfig(t)
	gw := newModelsGateway(t, "gw-token", []string{"m1", "m2"})
	t.Setenv("GATEWAY_TOKEN", "gw-token")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENAI_BASE_URL", "")
	argsFile := filepath.Join(t.TempDir(), "args")
	stubHarnessBinary(t, "pi", argsFile)

	codeFleetDir(t,
		"nodes:\n  - name: dead\n    host: 127.0.0.1\n    port: 1\ngateway:\n  url: "+gw.URL+"\n  tokenEnv: GATEWAY_TOKEN\n",
		"")
	captureStdout(t, func() {
		if err := cmdCode([]string{"--fleet", "fleet.yaml", "-H", "pi"}); err != nil {
			t.Fatalf("cmdCode -H pi: %v", err)
		}
	})
	root := readPiModels(t, home)
	providers, _ := root["providers"].(map[string]any)
	provider, ok := providers[gatewayKeyForTest(gw)].(map[string]any)
	if !ok {
		t.Fatalf("provider %q not found in %v", gatewayKeyForTest(gw), providers)
	}
	models, _ := provider["models"].([]any)
	got := map[string]bool{}
	for _, raw := range models {
		if m, ok := raw.(map[string]any); ok {
			if id, _ := m["id"].(string); id != "" {
				got[id] = true
			}
		}
	}
	for _, id := range []string{"m1", "m2"} {
		if !got[id] {
			t.Errorf("discovered model %q missing from pi's models array: %v", id, models)
		}
	}
}

// The gateway authenticates fine but its models endpoint fails: the launch
// still succeeds, with a warning and no models populated — the models query
// is a convenience for the picker, not something the launch depends on.
func TestCodeFleetNoSpinloopModelsFetchFails(t *testing.T) {
	home := isolateConfig(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	t.Setenv("GATEWAY_TOKEN", "gw-token")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENAI_BASE_URL", "")
	argsFile := filepath.Join(t.TempDir(), "args")
	envFile := filepath.Join(t.TempDir(), "env")
	stubHarnessBinaryWithEnv(t, argsFile, envFile)

	codeFleetDir(t,
		"nodes:\n  - name: dead\n    host: 127.0.0.1\n    port: 1\ngateway:\n  url: "+srv.URL+"\n  tokenEnv: GATEWAY_TOKEN\n",
		"")
	stderr := captureStderr(t, func() {
		captureStdout(t, func() {
			if err := cmdCode([]string{"--fleet", "fleet.yaml"}); err != nil {
				t.Fatalf("cmdCode: %v", err)
			}
		})
	})
	if !strings.Contains(stderr, "Warning: could not list the gateway's models") {
		t.Errorf("expected a warning about the failed models fetch, got:\n%s", stderr)
	}
	if _, err := os.ReadFile(argsFile); err != nil {
		t.Fatalf("the harness should still launch despite the fetch failure: %v", err)
	}
	config, err := os.ReadFile(filepath.Join(home, ".config", "opencode", "opencode.json"))
	if err != nil {
		t.Fatalf("the harness config was not written: %v", err)
	}
	var root map[string]any
	if err := json.Unmarshal(config, &root); err != nil {
		t.Fatalf("opencode.json not valid JSON: %v", err)
	}
	wantKey := gatewayKeyForTest(srv)
	provider, _ := root["provider"].(map[string]any)
	entry, ok := provider[wantKey].(map[string]any)
	if !ok {
		t.Fatalf("provider %q not found in %v", wantKey, provider)
	}
	if models, ok := entry["models"]; ok {
		t.Errorf("no models should be populated after a failed fetch, got: %v", models)
	}
}

// A fleet file's gateway.name overrides the address-derived label: the
// provider is keyed and displayed under the hand-picked name instead.
func TestCodeFleetNoSpinloopUsesGatewayName(t *testing.T) {
	home := isolateConfig(t)
	gw := newModelsGateway(t, "gw-token", []string{"m1"})
	t.Setenv("GATEWAY_TOKEN", "gw-token")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENAI_BASE_URL", "")
	argsFile := filepath.Join(t.TempDir(), "args")
	envFile := filepath.Join(t.TempDir(), "env")
	stubHarnessBinaryWithEnv(t, argsFile, envFile)

	codeFleetDir(t,
		"nodes:\n  - name: dead\n    host: 127.0.0.1\n    port: 1\ngateway:\n  url: "+gw.URL+"\n  tokenEnv: GATEWAY_TOKEN\n  name: remote-llms\n",
		"")
	captureStdout(t, func() {
		if err := cmdCode([]string{"--fleet", "fleet.yaml"}); err != nil {
			t.Fatalf("cmdCode: %v", err)
		}
	})
	config, err := os.ReadFile(filepath.Join(home, ".config", "opencode", "opencode.json"))
	if err != nil {
		t.Fatalf("the harness config was not written: %v", err)
	}
	var root map[string]any
	if err := json.Unmarshal(config, &root); err != nil {
		t.Fatalf("opencode.json not valid JSON: %v", err)
	}
	provider, _ := root["provider"].(map[string]any)
	entry, ok := provider["gateway-remote-llms"].(map[string]any)
	if !ok {
		t.Fatalf("provider %q not found in %v", "gateway-remote-llms", provider)
	}
	if name, _ := entry["name"].(string); name != "Gateway (remote-llms)" {
		t.Errorf("name = %q, want %q", name, "Gateway (remote-llms)")
	}
}

// A fleet file naming no gateway still fails needing a Spinloop, even though
// it resolves fine — only a fleet naming a gateway waives the requirement.
func TestCodeFleetNeedsASpinloopWithNodesOnlyFleet(t *testing.T) {
	isolateConfig(t)
	argsFile := filepath.Join(t.TempDir(), "args")
	stubHarnessBinary(t, "opencode", argsFile)

	codeFleetDir(t, "nodes:\n  - name: dead\n    host: 127.0.0.1\n    port: 1\n", "")
	captureStdout(t, func() {
		err := cmdCode([]string{"--fleet", "fleet.yaml"})
		if err == nil {
			t.Fatal("a launch with no Spinloop and no gateway should fail")
		}
		if !strings.Contains(err.Error(), "--fleet needs a Spinloop") {
			t.Errorf("the failure should say --fleet needs a Spinloop, got:\n%v", err)
		}
	})
	if _, err := os.ReadFile(argsFile); err == nil {
		t.Error("the harness was launched with no Spinloop and no gateway to route through")
	}
}

// No Spinloop, a gateway named, but its token is unset: the command still
// fails naming the fleet file (not an empty path) rather than launching with
// no way to authenticate.
func TestCodeFleetNoSpinloopGatewayNoToken(t *testing.T) {
	isolateConfig(t)
	t.Setenv("GATEWAY_TOKEN", "")
	argsFile := filepath.Join(t.TempDir(), "args")
	stubHarnessBinary(t, "opencode", argsFile)

	codeFleetDir(t,
		"nodes:\n  - name: dead\n    host: 127.0.0.1\n    port: 1\ngateway:\n  url: http://gw.internal:4000\n  tokenEnv: GATEWAY_TOKEN\n",
		"")
	captureStdout(t, func() {
		err := cmdCode([]string{"--fleet", "fleet.yaml"})
		if err == nil {
			t.Fatal("a gateway with no token set should fail")
		}
		if !strings.Contains(err.Error(), "no token to reach the gateway") {
			t.Errorf("the failure should say there is no token, got:\n%v", err)
		}
		if !strings.Contains(err.Error(), "fleet.yaml") {
			t.Errorf("the failure should name the fleet file, not an empty path, got:\n%v", err)
		}
	})
	if _, err := os.ReadFile(argsFile); err == nil {
		t.Error("the harness was launched with no token to reach the gateway")
	}
}

// No Spinloop at all, and the working directory holds no .env of its own: the
// gateway's token still resolves from the .env beside the fleet file, not the
// working directory — matching what the "no token" error already claims, and
// regardless of where the command was run from.
func TestCodeFleetNoSpinloopReadsEnvBesideFleetFile(t *testing.T) {
	isolateConfig(t)
	gw := newModelsGateway(t, "gw-token", []string{"m1"})
	t.Setenv("GATEWAY_TOKEN", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENAI_BASE_URL", "")
	argsFile := filepath.Join(t.TempDir(), "args")
	envFile := filepath.Join(t.TempDir(), "env")
	stubHarnessBinaryWithEnv(t, argsFile, envFile)

	fleetDir := t.TempDir()
	fleetPath := fleetFileIn(t, fleetDir,
		"nodes:\n  - name: dead\n    host: 127.0.0.1\n    port: 1\ngateway:\n  url: "+gw.URL+"\n  tokenEnv: GATEWAY_TOKEN\n")
	mustWrite(t, filepath.Join(fleetDir, ".env"), "GATEWAY_TOKEN=gw-token\n")
	t.Chdir(t.TempDir())

	captureStdout(t, func() {
		if err := cmdCode([]string{"--fleet", "fleet.yaml", "--fleet", fleetPath}); err != nil {
			t.Fatalf("cmdCode: %v", err)
		}
	})
	data, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatalf("the harness was not launched: %v", err)
	}
	if !strings.Contains(string(data), "KEY=gw-token") {
		t.Errorf("the agent should carry the token from the .env beside the fleet file, got:\n%s", data)
	}
}

// A hand-written Spinloop naming only a provider, routed at a gateway, gets
// its model list populated the same way the no-Spinloop case does.
func TestCodeFleetHandwrittenProviderOnlySpinloopGetsModels(t *testing.T) {
	home := isolateConfig(t)
	gw := newModelsGateway(t, "gw-token", []string{"m1"})
	t.Setenv("GATEWAY_TOKEN", "gw-token")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENAI_BASE_URL", "")
	argsFile := filepath.Join(t.TempDir(), "args")
	envFile := filepath.Join(t.TempDir(), "env")
	stubHarnessBinaryWithEnv(t, argsFile, envFile)

	codeFleetDir(t,
		"nodes:\n  - name: dead\n    host: 127.0.0.1\n    port: 1\ngateway:\n  url: "+gw.URL+"\n  tokenEnv: GATEWAY_TOKEN\n",
		"PROVIDER "+gatewayProviderID+"\n")
	captureStdout(t, func() {
		if err := cmdCode([]string{"-O"}); err != nil {
			t.Fatalf("cmdCode: %v", err)
		}
	})
	config, err := os.ReadFile(filepath.Join(home, ".config", "opencode", "opencode.json"))
	if err != nil {
		t.Fatalf("the harness config was not written: %v", err)
	}
	var root map[string]any
	if err := json.Unmarshal(config, &root); err != nil {
		t.Fatalf("opencode.json not valid JSON: %v", err)
	}
	wantKey := gatewayKeyForTest(gw)
	provider, _ := root["provider"].(map[string]any)
	entry, ok := provider[wantKey].(map[string]any)
	if !ok {
		t.Fatalf("provider %q not found in %v", wantKey, provider)
	}
	models, _ := entry["models"].(map[string]any)
	if _, ok := models["m1"]; !ok {
		t.Errorf("discovered model missing from provider.models: %v", models)
	}
}

// A gateway-only launch with --harness lucinate still succeeds — lucinate's
// connection holds one model, so the discovered list is not consulted, but
// the connection itself is written with the gateway's address and no
// defaultModel key.
func TestCodeFleetNoSpinloopLucinateNoModelList(t *testing.T) {
	home := isolateConfig(t)
	gw := newModelsGateway(t, "gw-token", []string{"m1"})
	t.Setenv("GATEWAY_TOKEN", "gw-token")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENAI_BASE_URL", "")
	argsFile := filepath.Join(t.TempDir(), "args")
	stubHarnessBinary(t, "lucinate", argsFile)

	codeFleetDir(t,
		"nodes:\n  - name: dead\n    host: 127.0.0.1\n    port: 1\ngateway:\n  url: "+gw.URL+"\n  tokenEnv: GATEWAY_TOKEN\n",
		"")
	captureStdout(t, func() {
		if err := cmdCode([]string{"--fleet", "fleet.yaml", "-H", "lucinate"}); err != nil {
			t.Fatalf("cmdCode -H lucinate: %v", err)
		}
	})
	root := readLucinateStore(t, home)
	conn := lucinateConn(t, root, gatewayKeyForTest(gw))
	if url, _ := conn["url"].(string); url != gw.URL+"/v1" {
		t.Errorf("url = %q, want %q", url, gw.URL+"/v1")
	}
	if v, ok := conn["defaultModel"]; ok {
		t.Errorf("no defaultModel should be set, got: %v", v)
	}
	host := strings.TrimPrefix(gw.URL, "http://")
	if name, _ := conn["name"].(string); name != "Gateway ("+host+")" {
		t.Errorf("name = %q, want it to read like the remote-environment pattern (e.g. %q)", name, "Gateway ("+host+")")
	}
}

// A combination neither spelling ran before: `fleet harness` forwarded no
// arguments to the agent, and the launch never reached a gateway without a
// Spinloop. Now both hold at once, so a gateway launch forwards its trailing
// arguments the way every other launch does.
func TestCodeFleetGatewayLaunchForwardsTrailingArgs(t *testing.T) {
	isolateConfig(t)
	t.Setenv("GATEWAY_TOKEN", "gw-token")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENAI_BASE_URL", "")
	argsFile := filepath.Join(t.TempDir(), "args")
	stubHarnessBinary(t, "opencode", argsFile)

	codeFleetDir(t,
		"nodes:\n  - name: dead\n    host: 127.0.0.1\n    port: 1\ngateway:\n  url: http://gw.internal:4000\n  tokenEnv: GATEWAY_TOKEN\n",
		"")
	captureStderr(t, func() {
		captureStdout(t, func() {
			if err := cmdCode([]string{"--fleet", "fleet.yaml", "run", "--print"}); err != nil {
				t.Fatalf("cmdCode: %v", err)
			}
		})
	})
	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("the harness was not launched: %v", err)
	}
	got := string(data)
	for _, want := range []string{"run", "--print"} {
		if !strings.Contains(got, want) {
			t.Errorf("the agent should receive %q, got:\n%s", want, got)
		}
	}
}

// The removed spelling names its replacement, the way a moved top-level
// command does, rather than reporting an unknown subcommand.
func TestFleetHarnessMovedNamesItsReplacement(t *testing.T) {
	err := cmdFleet([]string{"harness"})
	if err == nil {
		t.Fatal("`fleet harness` should fail: it moved")
	}
	for _, want := range []string{"fleet harness", "moved", "spinloop code --fleet"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error should mention %q, got:\n%v", want, err)
		}
	}
}

// The one deliberate behaviour change: `fleet harness` defaulted the fleet
// file in every case, while a launch picks one up only when the worn Spinloop
// was not named explicitly. A named Spinloop therefore no longer routes on its
// own, and passing --fleet restores it.
func TestCodeFleetNamedSpinloopNeedsTheFleetFlag(t *testing.T) {
	isolateConfig(t)
	t.Setenv("SPINLOOP_ALIAS", "")
	t.Setenv("GATEWAY_TOKEN", "gw-token")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENAI_BASE_URL", "")
	argsFile := filepath.Join(t.TempDir(), "args")
	envFile := filepath.Join(t.TempDir(), "env")
	stubHarnessBinaryWithEnv(t, argsFile, envFile)

	codeFleetDir(t,
		"nodes:\n  - name: dead\n    host: 127.0.0.1\n    port: 1\ngateway:\n  url: http://gw.internal:4000\n  tokenEnv: GATEWAY_TOKEN\n",
		"PROVIDER llamacpp\nMODEL qwen3-27b\n")

	// Named explicitly: the directory's fleet file is not picked up, so the
	// launch does not route and the agent gets no gateway address.
	captureStderr(t, func() {
		captureStdout(t, func() {
			if err := cmdCode([]string{"-O=Spinloop"}); err != nil {
				t.Fatalf("cmdCode: %v", err)
			}
		})
	})
	data, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "BASE=http://gw.internal:4000") {
		t.Error("a named Spinloop should not pick up the directory's fleet file")
	}

	// The same Spinloop with --fleet routes at the gateway.
	captureStderr(t, func() {
		captureStdout(t, func() {
			if err := cmdCode([]string{"-O=Spinloop", "--fleet", "fleet.yaml"}); err != nil {
				t.Fatalf("cmdCode --fleet: %v", err)
			}
		})
	})
	data, err = os.ReadFile(envFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "BASE=http://gw.internal:4000/v1") {
		t.Errorf("--fleet should route the named Spinloop at the gateway, got:\n%s", data)
	}
}
