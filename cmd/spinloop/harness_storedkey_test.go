package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spinloop-ai/spinloop/internal/cloud"
	"github.com/spinloop-ai/spinloop/internal/fleet"
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

func unslothSel(baseURL string) spinloop.Selection {
	return spinloop.Selection{Provider: "unsloth", Alias: "inkling-2-mini", BaseURL: baseURL}
}

func TestStoredKeyResolver_LaunchCarriesStudioKey(t *testing.T) {
	studioHome(t, "sk-unsloth-stored")
	dir := t.TempDir()

	resolve := storedKeyResolver(opencode.EnvResolver(dir), "", unslothSel("http://127.0.0.1:8084/v1"), true, nil, nil)
	got, ok := envValue(harnessEnv("", resolve, nil), "UNSLOTH_API_KEY")
	if !ok || got != "sk-unsloth-stored" {
		t.Fatalf("UNSLOTH_API_KEY = %q (set %v), want the stored Studio key", got, ok)
	}
}

func TestStoredKeyResolver_DotEnvWinsOverStoredKey(t *testing.T) {
	studioHome(t, "sk-unsloth-stored")
	dir := t.TempDir()
	writeDotEnv(t, dir, "UNSLOTH_API_KEY=sk-from-dotenv\n")

	resolve := storedKeyResolver(opencode.EnvResolver(dir), "", unslothSel(""), true, nil, nil)
	if got, _ := envValue(harnessEnv("", resolve, nil), "UNSLOTH_API_KEY"); got != "sk-from-dotenv" {
		t.Fatalf("UNSLOTH_API_KEY = %q, want the .env value", got)
	}
}

func TestStoredKeyResolver_ExportWinsOverStoredKey(t *testing.T) {
	studioHome(t, "sk-unsloth-stored")
	t.Setenv("UNSLOTH_API_KEY", "sk-exported")

	resolve := storedKeyResolver(opencode.EnvResolver(t.TempDir()), "", unslothSel(""), true, nil, nil)
	if got, _ := envValue(harnessEnv("", resolve, nil), "UNSLOTH_API_KEY"); got != "sk-exported" {
		t.Fatalf("UNSLOTH_API_KEY = %q, want the exported value", got)
	}
}

func TestStoredKeyResolver_NotUsedForRemoteEndpoint(t *testing.T) {
	studioHome(t, "sk-unsloth-stored")

	resolve := storedKeyResolver(opencode.EnvResolver(t.TempDir()), "", unslothSel("https://studio.example.com/v1"), true, nil, nil)
	if got, ok := envValue(harnessEnv("", resolve, nil), "UNSLOTH_API_KEY"); ok && got != "" {
		t.Fatalf("UNSLOTH_API_KEY = %q, want it unset for a remote endpoint", got)
	}
}

func TestStoredKeyResolver_PassesThroughWhenNotApplicable(t *testing.T) {
	studioHome(t, "sk-unsloth-stored")
	base := opencode.EnvResolver(t.TempDir())
	sel := unslothSel("")

	cases := map[string]func(string) string{
		"not worn":     storedKeyResolver(base, "", sel, false, nil, nil),
		"cloud launch": storedKeyResolver(base, "", sel, true, &cloud.Response{}, nil),
		"fleet launch": storedKeyResolver(base, "", sel, true, nil, &fleet.Choice{}),
		"no provider":  storedKeyResolver(base, "", spinloop.Selection{}, true, nil, nil),
		"unknown":      storedKeyResolver(base, "", spinloop.Selection{Provider: "nope"}, true, nil, nil),
		"bad catalog":  storedKeyResolver(base, filepath.Join(t.TempDir(), "missing.yaml"), sel, true, nil, nil),
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

	resolve := storedKeyResolver(opencode.EnvResolver(t.TempDir()), "", sel, true, nil, nil)
	key, ok := lucinateLaunchKey("", resolve, sel, true)
	if !ok || key != "sk-unsloth-stored" {
		t.Fatalf("lucinate launch key = %q (ok %v), want the stored Studio key", key, ok)
	}
}
