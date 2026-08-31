package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/spinloop-ai/spinloop/internal/config"
	"github.com/spinloop-ai/spinloop/internal/remote"
)

// opencodeConfigPath returns where isolateConfig's XDG_CONFIG_HOME points the
// opencode config at, for reading back what apply wrote.
func opencodeConfigPath(home string) string {
	return filepath.Join(home, ".config", "opencode", "opencode.json")
}

// staticServer serves fixed bodies at fixed paths, 404ing anything else.
func staticServer(t *testing.T, files map[string]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(body))
	}))
}

// TestCmdApply_FromURL checks that a Spinloop path may be a URL, fetched
// instead of read from local disk.
func TestCmdApply_FromURL(t *testing.T) {
	home := isolateConfig(t)
	server := staticServer(t, map[string]string{
		"/Spinloop": "PROVIDER llamacpp\nALIAS q3\n",
	})
	defer server.Close()

	captureStdout(t, func() {
		if err := cmdApply([]string{server.URL + "/Spinloop"}); err != nil {
			t.Fatalf("cmdApply: %v", err)
		}
	})

	m := readConfigMap(t, opencodeConfigPath(home))
	models := m["provider"].(map[string]any)["llamacpp"].(map[string]any)["models"].(map[string]any)
	if _, ok := models["q3"]; !ok {
		t.Errorf("expected model %q from the fetched Spinloop, got %v", "q3", models)
	}
}

// TestCmdApply_FromURL_TrailingSlash checks the URL analogue of a directory
// argument: a URL ending in "/" has Spinloop appended.
func TestCmdApply_FromURL_TrailingSlash(t *testing.T) {
	home := isolateConfig(t)
	server := staticServer(t, map[string]string{
		"/team/Spinloop": "PROVIDER llamacpp\nALIAS q3\n",
	})
	defer server.Close()

	captureStdout(t, func() {
		if err := cmdApply([]string{server.URL + "/team/"}); err != nil {
			t.Fatalf("cmdApply: %v", err)
		}
	})

	m := readConfigMap(t, opencodeConfigPath(home))
	models := m["provider"].(map[string]any)["llamacpp"].(map[string]any)["models"].(map[string]any)
	if _, ok := models["q3"]; !ok {
		t.Errorf("expected model %q from the fetched Spinloop, got %v", "q3", models)
	}
}

// TestCmdApply_FromURL_NotFound checks that a 404 surfaces a clear error
// naming the URL, not a filesystem "not found" error.
func TestCmdApply_FromURL_NotFound(t *testing.T) {
	isolateConfig(t)
	server := staticServer(t, map[string]string{})
	defer server.Close()

	err := cmdApply([]string{server.URL + "/Spinloop"})
	if err == nil {
		t.Fatal("expected an error for a 404 Spinloop URL")
	}
	if !strings.Contains(err.Error(), server.URL) {
		t.Errorf("error %q does not name the URL", err)
	}
}

// TestCmdApply_FromURL_Unreachable checks that an unreachable host fails with
// a network error rather than hanging or a filesystem error.
func TestCmdApply_FromURL_Unreachable(t *testing.T) {
	isolateConfig(t)
	server := staticServer(t, map[string]string{"/Spinloop": "PROVIDER llamacpp\nALIAS q3\n"})
	url := server.URL + "/Spinloop"
	server.Close() // now refuses connections

	if err := cmdApply([]string{url}); err == nil {
		t.Fatal("expected an error for an unreachable Spinloop URL")
	}
}

// TestCmdAlias_URL checks that spinloop alias can register a URL, and that it
// round-trips through spinloop apply.
func TestCmdAlias_URL(t *testing.T) {
	home := isolateConfig(t)
	server := staticServer(t, map[string]string{
		"/Spinloop": "PROVIDER llamacpp\nALIAS q3\n",
	})
	defer server.Close()
	url := server.URL + "/Spinloop"

	out := captureStdout(t, func() {
		if err := cmdAlias([]string{"-n", "team-default", url}); err != nil {
			t.Fatalf("cmdAlias: %v", err)
		}
	})
	if !strings.Contains(out, "team-default") {
		t.Errorf("unexpected output:\n%s", out)
	}
	if got := storedAlias(t, "team-default"); got != url {
		t.Errorf("stored alias = %q, want the URL verbatim %q", got, url)
	}

	captureStdout(t, func() {
		if err := cmdApply([]string{"team-default"}); err != nil {
			t.Fatalf("cmdApply via alias: %v", err)
		}
	})
	m := readConfigMap(t, opencodeConfigPath(home))
	models := m["provider"].(map[string]any)["llamacpp"].(map[string]any)["models"].(map[string]any)
	if _, ok := models["q3"]; !ok {
		t.Errorf("expected model %q applied via the URL alias, got %v", "q3", models)
	}
}

// TestCmdAlias_List_DoesNotProbeURL checks that listing a URL-valued alias
// makes no network request, and never marks it "(missing)".
func TestCmdAlias_List_DoesNotProbeURL(t *testing.T) {
	isolateConfig(t)
	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Write([]byte("PROVIDER llamacpp\nALIAS q3\n"))
	}))
	defer server.Close()
	url := server.URL + "/Spinloop"

	if err := config.Update(func(f *config.File) error {
		f.SetAlias("team-default", url)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() {
		if err := cmdAlias([]string{"--list"}); err != nil {
			t.Fatalf("cmdAlias --list: %v", err)
		}
	})
	if !strings.Contains(out, url) {
		t.Errorf("expected the URL in the listing, got:\n%s", out)
	}
	if strings.Contains(out, "(missing)") {
		t.Errorf("a URL entry should never be marked (missing):\n%s", out)
	}
	if got := atomic.LoadInt32(&hits); got != 0 {
		t.Errorf("spinloop alias --list made %d request(s) to the URL alias, want 0", got)
	}
}

// TestCmdAlias_URLRepointNeedsForce checks that re-registering a name already
// pointing at a URL needs --force, matching the local-path behavior.
func TestCmdAlias_URLRepointNeedsForce(t *testing.T) {
	isolateConfig(t)
	server := staticServer(t, map[string]string{
		"/a/Spinloop": "PROVIDER llamacpp\nALIAS q3\n",
		"/b/Spinloop": "PROVIDER llamacpp\nALIAS q3\n",
	})
	defer server.Close()
	first := server.URL + "/a/Spinloop"
	second := server.URL + "/b/Spinloop"

	captureStdout(t, func() {
		if err := cmdAlias([]string{first}); err != nil {
			t.Fatalf("cmdAlias: %v", err)
		}
	})

	err := cmdAlias([]string{second})
	if err == nil {
		t.Fatal("expected an error re-pointing an existing URL alias")
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("error %q does not mention --force", err)
	}
	if got := storedAlias(t, "q3"); got != first {
		t.Errorf("alias moved without --force: %q", got)
	}

	captureStdout(t, func() {
		if err := cmdAlias([]string{"--force", second}); err != nil {
			t.Fatalf("cmdAlias --force: %v", err)
		}
	})
	if got := storedAlias(t, "q3"); got != second {
		t.Errorf("stored path = %q, want %q", got, second)
	}
}

// TestServe_PresetURL checks that a PRESET may itself be a URL, fetched by
// `spinloop serve`.
func TestServe_PresetURL(t *testing.T) {
	server := staticServer(t, map[string]string{
		"/preset.ini": samplePreset,
	})
	defer server.Close()
	presetURL := server.URL + "/preset.ini"

	dir := t.TempDir()
	spinloopPath := filepath.Join(dir, "Spinloop")
	mustWrite(t, spinloopPath, "PROVIDER llamacpp\nALIAS qwen\nPRESET "+presetURL+"\n")

	out := captureStdout(t, func() {
		if err := cmdServe([]string{"--dry-run", spinloopPath}); err != nil {
			t.Fatalf("cmdServe: %v", err)
		}
	})
	if !strings.Contains(out, "Using preset "+presetURL) {
		t.Errorf("unexpected output:\n%s", out)
	}
}

// TestServe_PresetRelativeToURLSpinloop checks that a relative PRESET resolves
// against a URL-sourced Spinloop's own URL, and is fetched from there.
func TestServe_PresetRelativeToURLSpinloop(t *testing.T) {
	server := staticServer(t, map[string]string{
		"/team/Spinloop":   "PROVIDER llamacpp\nALIAS qwen\nPRESET ./preset.ini\n",
		"/team/preset.ini": samplePreset,
	})
	defer server.Close()

	out := captureStdout(t, func() {
		if err := cmdServe([]string{"--dry-run", server.URL + "/team/Spinloop"}); err != nil {
			t.Fatalf("cmdServe: %v", err)
		}
	})
	if !strings.Contains(out, "Using preset "+server.URL+"/team/preset.ini") {
		t.Errorf("unexpected output:\n%s", out)
	}
}

// TestCmdApply_DoesNotFetchPresetURL checks that apply never fetches a
// PRESET, whether it is local or a URL — PRESET is serve's business alone.
func TestCmdApply_DoesNotFetchPresetURL(t *testing.T) {
	isolateConfig(t)
	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Write([]byte(samplePreset))
	}))
	defer server.Close()

	dir := t.TempDir()
	spinloopPath := filepath.Join(dir, "Spinloop")
	mustWrite(t, spinloopPath, "PROVIDER llamacpp\nALIAS qwen\nPRESET "+server.URL+"/preset.ini\n")

	captureStdout(t, func() {
		if err := cmdApply([]string{spinloopPath}); err != nil {
			t.Fatalf("cmdApply: %v", err)
		}
	})
	if got := atomic.LoadInt32(&hits); got != 0 {
		t.Errorf("spinloop apply made %d request(s) to the PRESET URL, want 0", got)
	}
}

// remoteConfigJSON marshals a remote.Config for a static file server.
func remoteConfigJSON(t *testing.T, cfg remote.Config) string {
	t.Helper()
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// TestRemoteStatus_URLRemote checks that a path-form REMOTE may itself be a
// URL, fetched by the remote subcommands.
func TestRemoteStatus_URLRemote(t *testing.T) {
	isolateConfig(t)
	stubAWSEnv(t)
	control := stateServer(t)
	defer control.Close()

	cfgServer := staticServer(t, map[string]string{
		"/remote.json": remoteConfigJSON(t, remote.Config{StartURL: control.URL, StopURL: control.URL, Region: "eu-west-1"}),
	})
	defer cfgServer.Close()

	dir := t.TempDir()
	spinloopPath := filepath.Join(dir, "Spinloop")
	mustWrite(t, spinloopPath, "PROVIDER openai-compatible\nREMOTE "+cfgServer.URL+"/remote.json\n")

	out := captureStdout(t, func() {
		if err := cmdRemoteStatus([]string{spinloopPath}); err != nil {
			t.Fatalf("cmdRemoteStatus: %v", err)
		}
	})
	if !strings.Contains(out, "state: running") {
		t.Errorf("REMOTE URL should resolve the control config, got:\n%s", out)
	}
}

// TestRemoteStatus_RemoteRelativeToURLSpinloop checks that a relative REMOTE
// resolves against a URL-sourced Spinloop's own URL.
func TestRemoteStatus_RemoteRelativeToURLSpinloop(t *testing.T) {
	isolateConfig(t)
	stubAWSEnv(t)
	control := stateServer(t)
	defer control.Close()

	server := staticServer(t, map[string]string{
		"/team/Spinloop": "PROVIDER openai-compatible\nREMOTE ./remote.json\n",
		"/team/remote.json": remoteConfigJSON(t, remote.Config{
			StartURL: control.URL, StopURL: control.URL, Region: "eu-west-1",
		}),
	})
	defer server.Close()

	out := captureStdout(t, func() {
		if err := cmdRemoteStatus([]string{server.URL + "/team/Spinloop"}); err != nil {
			t.Fatalf("cmdRemoteStatus: %v", err)
		}
	})
	if !strings.Contains(out, "state: running") {
		t.Errorf("relative REMOTE should resolve against the Spinloop's URL, got:\n%s", out)
	}
}

// TestCmdApply_BaseURLFallback_FetchesURLRemote checks that apply's base-URL
// fallback fetches a URL-form REMOTE when the Spinloop states no BASEURL.
func TestCmdApply_BaseURLFallback_FetchesURLRemote(t *testing.T) {
	home := isolateConfig(t)
	server := staticServer(t, map[string]string{
		"/remote.json": remoteConfigJSON(t, remote.Config{
			StartURL: "https://example.com/start", StopURL: "https://example.com/stop",
			Region: "eu-west-1", BaseURL: "https://endpoint.example.com/v1", Environment: "prod",
		}),
	})
	defer server.Close()

	dir := t.TempDir()
	spinloopPath := filepath.Join(dir, "Spinloop")
	mustWrite(t, spinloopPath, "PROVIDER llamacpp\nALIAS q3\nREMOTE "+server.URL+"/remote.json\n")

	out := captureStdout(t, func() {
		if err := cmdApply([]string{spinloopPath}); err != nil {
			t.Fatalf("cmdApply: %v", err)
		}
	})
	if !strings.Contains(out, "Taking the base URL from") {
		t.Errorf("expected the base URL to be taken from the fetched REMOTE config, got:\n%s", out)
	}
	m := readConfigMap(t, opencodeConfigPath(home))
	prod := m["provider"].(map[string]any)["prod"].(map[string]any)
	if got := prod["options"].(map[string]any)["baseURL"]; got != "https://endpoint.example.com/v1" {
		t.Errorf("baseURL = %v, want the fetched REMOTE config's base_url", got)
	}
}

// TestCmdServe_DoesNotFetchRemoteURL checks that serve never fetches a
// URL-form REMOTE — it has no use for a remote endpoint's control
// configuration, matching how a local-path REMOTE is already left alone.
func TestCmdServe_DoesNotFetchRemoteURL(t *testing.T) {
	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Write([]byte(remoteConfigJSON(t, remote.Config{StartURL: "x", StopURL: "x", Region: "eu-west-1"})))
	}))
	defer server.Close()

	dir := t.TempDir()
	spinloopPath := filepath.Join(dir, "Spinloop")
	mustWrite(t, spinloopPath, "PROVIDER llamacpp\nALIAS q3\nMODEL unsloth/Qwen:Q4_K_M\nREMOTE "+server.URL+"/remote.json\n")

	captureStdout(t, func() {
		if err := cmdServe([]string{"--dry-run", spinloopPath}); err != nil {
			t.Fatalf("cmdServe: %v", err)
		}
	})
	if got := atomic.LoadInt32(&hits); got != 0 {
		t.Errorf("spinloop serve made %d request(s) to the REMOTE URL, want 0", got)
	}
}

// TestCmdApply_FetchesRemoteURL_ForEnvironmentName_EvenWithBaseURL checks
// that apply still reads a URL-form REMOTE once when the Spinloop already
// states its own BASEURL — to name the harness provider after the
// deployment's environment, the same unconditional read a local-path REMOTE
// already triggers — even though the base-URL fallback itself is skipped.
func TestCmdApply_FetchesRemoteURL_ForEnvironmentName_EvenWithBaseURL(t *testing.T) {
	home := isolateConfig(t)
	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Write([]byte(remoteConfigJSON(t, remote.Config{
			StartURL: "x", StopURL: "x", Region: "eu-west-1", Environment: "prod",
		})))
	}))
	defer server.Close()

	dir := t.TempDir()
	spinloopPath := filepath.Join(dir, "Spinloop")
	mustWrite(t, spinloopPath, "PROVIDER llamacpp\nALIAS q3\nBASEURL http://127.0.0.1:9090/v1\nREMOTE "+server.URL+"/remote.json\n")

	captureStdout(t, func() {
		if err := cmdApply([]string{spinloopPath}); err != nil {
			t.Fatalf("cmdApply: %v", err)
		}
	})
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Errorf("spinloop apply made %d request(s) to the REMOTE URL, want exactly 1 (for the environment name)", got)
	}
	m := readConfigMap(t, opencodeConfigPath(home))
	if _, ok := m["provider"].(map[string]any)["prod"]; !ok {
		t.Errorf("expected the provider renamed to the fetched environment %q, got %v", "prod", m["provider"])
	}
}
