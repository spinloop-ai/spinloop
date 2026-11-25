package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/spinloop-ai/spinloop/internal/cloud"
	"github.com/spinloop-ai/spinloop/internal/fleet"
	"github.com/spinloop-ai/spinloop/internal/harness"
	"github.com/spinloop-ai/spinloop/internal/opencode"
	"github.com/spinloop-ai/spinloop/internal/spinloop"
)

// studioHome points HOME at a temporary directory holding a stored Studio key,
// and clears UNSLOTH_API_KEY so only the file can supply one.
func studioHome(t *testing.T, key string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("UNSLOTH_API_KEY", "")
	auth := filepath.Join(home, ".unsloth", "studio", "auth")
	if err := os.MkdirAll(auth, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(auth, ".cli_api_key_cli_0123abcd"), []byte(key+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

// noState is a harness config holding no providers and no default model.
func noState() (map[string]harness.ProviderState, string, error) { return nil, "", nil }

// stateOf returns a harness config holding the given providers' base URLs and
// default model.
func stateOf(defaultModel string, baseURLs map[string]string) func() (map[string]harness.ProviderState, string, error) {
	return func() (map[string]harness.ProviderState, string, error) {
		states := map[string]harness.ProviderState{}
		for id, u := range baseURLs {
			states[id] = harness.ProviderState{BaseURL: u}
		}
		return states, defaultModel, nil
	}
}

func unslothSel(baseURL string) spinloop.Selection {
	return spinloop.Selection{Provider: "unsloth", Alias: "inkling-2-mini", BaseURL: baseURL}
}

func TestStoredKeyResolver_LaunchCarriesStudioKey(t *testing.T) {
	studioHome(t, "sk-unsloth-stored")
	dir := t.TempDir()

	resolve := storedKeyResolver(opencode.EnvResolver(dir), "", noState, unslothSel("http://127.0.0.1:8084/v1"), true, nil, nil)
	got, ok := envValue(harnessEnv("", resolve, nil), "UNSLOTH_API_KEY")
	if !ok || got != "sk-unsloth-stored" {
		t.Fatalf("UNSLOTH_API_KEY = %q (set %v), want the stored Studio key", got, ok)
	}
}

func TestStoredKeyResolver_DotEnvWinsOverStoredKey(t *testing.T) {
	studioHome(t, "sk-unsloth-stored")
	dir := t.TempDir()
	writeDotEnv(t, dir, "UNSLOTH_API_KEY=sk-from-dotenv\n")

	resolve := storedKeyResolver(opencode.EnvResolver(dir), "", noState, unslothSel(""), true, nil, nil)
	if got, _ := envValue(harnessEnv("", resolve, nil), "UNSLOTH_API_KEY"); got != "sk-from-dotenv" {
		t.Fatalf("UNSLOTH_API_KEY = %q, want the .env value", got)
	}
}

func TestStoredKeyResolver_ExportWinsOverStoredKey(t *testing.T) {
	studioHome(t, "sk-unsloth-stored")
	t.Setenv("UNSLOTH_API_KEY", "sk-exported")

	resolve := storedKeyResolver(opencode.EnvResolver(t.TempDir()), "", noState, unslothSel(""), true, nil, nil)
	if got, _ := envValue(harnessEnv("", resolve, nil), "UNSLOTH_API_KEY"); got != "sk-exported" {
		t.Fatalf("UNSLOTH_API_KEY = %q, want the exported value", got)
	}
}

func TestStoredKeyResolver_NotUsedForRemoteEndpoint(t *testing.T) {
	studioHome(t, "sk-unsloth-stored")

	resolve := storedKeyResolver(opencode.EnvResolver(t.TempDir()), "", noState, unslothSel("https://studio.example.com/v1"), true, nil, nil)
	if got, ok := envValue(harnessEnv("", resolve, nil), "UNSLOTH_API_KEY"); ok && got != "" {
		t.Fatalf("UNSLOTH_API_KEY = %q, want it unset for a remote endpoint", got)
	}
}

func TestStoredKeyResolver_PassesThroughWhenNotApplicable(t *testing.T) {
	studioHome(t, "sk-unsloth-stored")
	base := opencode.EnvResolver(t.TempDir())
	sel := unslothSel("")

	cases := map[string]func(string) string{
		"no config":    storedKeyResolver(base, "", noState, sel, false, nil, nil),
		"cloud launch": storedKeyResolver(base, "", noState, sel, true, &cloud.Response{}, nil),
		"fleet launch": storedKeyResolver(base, "", noState, sel, true, nil, &fleet.Choice{}),
		"no provider":  storedKeyResolver(base, "", noState, spinloop.Selection{}, true, nil, nil),
		"unknown":      storedKeyResolver(base, "", noState, spinloop.Selection{Provider: "nope"}, true, nil, nil),
		"bad catalog":  storedKeyResolver(base, filepath.Join(t.TempDir(), "missing.yaml"), noState, sel, true, nil, nil),
	}
	for name, resolve := range cases {
		if got := resolve("UNSLOTH_API_KEY"); got != "" {
			t.Errorf("%s: UNSLOTH_API_KEY resolved to %q, want empty", name, got)
		}
	}
}

func TestLucinateLaunchKey_UsesStoredStudioKey(t *testing.T) {
	studioHome(t, "sk-unsloth-stored")
	sel := unslothSel("http://127.0.0.1:8084/v1")

	resolve := storedKeyResolver(opencode.EnvResolver(t.TempDir()), "", noState, sel, true, nil, nil)
	key, ok := lucinateLaunchKey("", resolve, sel, true)
	if !ok || key != "sk-unsloth-stored" {
		t.Fatalf("lucinate launch key = %q (ok %v), want the stored Studio key", key, ok)
	}
}

// A bare launch, with no Spinloop applied, uses the provider of the harness's
// default model.
func TestStoredKeyResolver_BareLaunchUsesDefaultModelsProvider(t *testing.T) {
	studioHome(t, "sk-unsloth-stored")
	state := stateOf("unsloth/inkling-2-mini", map[string]string{
		"unsloth":  "http://127.0.0.1:8084/v1",
		"llamacpp": "http://127.0.0.1:8081/v1",
	})

	resolve := storedKeyResolver(opencode.EnvResolver(t.TempDir()), "", state, spinloop.Selection{}, false, nil, nil)
	if got, _ := envValue(harnessEnv("", resolve, nil), "UNSLOTH_API_KEY"); got != "sk-unsloth-stored" {
		t.Fatalf("UNSLOTH_API_KEY = %q, want the stored Studio key", got)
	}
}

// The config may hold the provider without a base URL, which means the
// catalogue's endpoint: local for unsloth.
func TestStoredKeyResolver_BareLaunchFallsBackToCatalogueEndpoint(t *testing.T) {
	studioHome(t, "sk-unsloth-stored")
	state := stateOf("unsloth/inkling-2-mini", map[string]string{"unsloth": ""})

	resolve := storedKeyResolver(opencode.EnvResolver(t.TempDir()), "", state, spinloop.Selection{}, false, nil, nil)
	if got := resolve("UNSLOTH_API_KEY"); got != "sk-unsloth-stored" {
		t.Fatalf("UNSLOTH_API_KEY = %q, want the stored Studio key", got)
	}
}

func TestStoredKeyResolver_BareLaunchSkipsRemoteProvider(t *testing.T) {
	studioHome(t, "sk-unsloth-stored")
	state := stateOf("unsloth/inkling-2-mini", map[string]string{"unsloth": "https://studio.example.com/v1"})

	resolve := storedKeyResolver(opencode.EnvResolver(t.TempDir()), "", state, spinloop.Selection{}, false, nil, nil)
	if got := resolve("UNSLOTH_API_KEY"); got != "" {
		t.Fatalf("UNSLOTH_API_KEY = %q, want it unset for a remote endpoint", got)
	}
}

// Pi and lucinate report no default model, so every configured provider is
// considered.
func TestStoredKeyResolver_BareLaunchWithNoDefaultUsesConfiguredProviders(t *testing.T) {
	studioHome(t, "sk-unsloth-stored")
	state := stateOf("", map[string]string{
		"llamacpp": "http://127.0.0.1:8081/v1",
		"unsloth":  "http://127.0.0.1:8084/v1",
	})

	resolve := storedKeyResolver(opencode.EnvResolver(t.TempDir()), "", state, spinloop.Selection{}, false, nil, nil)
	if got := resolve("UNSLOTH_API_KEY"); got != "sk-unsloth-stored" {
		t.Fatalf("UNSLOTH_API_KEY = %q, want the stored Studio key", got)
	}
}

func TestStoredKeyResolver_BareLaunchLeavesOtherProvidersAlone(t *testing.T) {
	studioHome(t, "sk-unsloth-stored")
	base := opencode.EnvResolver(t.TempDir())
	configured := map[string]string{"unsloth": "http://127.0.0.1:8084/v1", "llamacpp": "http://127.0.0.1:8081/v1"}
	failing := func() (map[string]harness.ProviderState, string, error) {
		return nil, "", errors.New("unreadable config")
	}

	cases := map[string]func(string) string{
		"other default":     storedKeyResolver(base, "", stateOf("llamacpp/qwen", configured), spinloop.Selection{}, false, nil, nil),
		"unknown provider":  storedKeyResolver(base, "", stateOf("dev-2/qwen", map[string]string{"dev-2": "https://x.example.com/v1"}), spinloop.Selection{}, false, nil, nil),
		"unreadable config": storedKeyResolver(base, "", failing, spinloop.Selection{}, false, nil, nil),
		"worn other":        storedKeyResolver(base, "", stateOf("unsloth/inkling-2-mini", configured), spinloop.Selection{Provider: "llamacpp", Model: "m"}, true, nil, nil),
		"cloud launch":      storedKeyResolver(base, "", stateOf("unsloth/inkling-2-mini", configured), spinloop.Selection{}, false, &cloud.Response{}, nil),
	}
	for name, resolve := range cases {
		if got := resolve("UNSLOTH_API_KEY"); got != "" {
			t.Errorf("%s: UNSLOTH_API_KEY resolved to %q, want empty", name, got)
		}
	}
}
