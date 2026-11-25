package catalog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeKey writes a key file under dir with the given modification time.
func writeKey(t *testing.T, dir, name, content string, mod time.Time) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mod, mod); err != nil {
		t.Fatal(err)
	}
}

func TestStoredKey(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name  string
		files map[string]string // name -> content; written oldest first in key order
		want  string
	}{
		{name: "no match", want: ""},
		{name: "empty file", files: map[string]string{".key_a": ""}, want: ""},
		{name: "whitespace trimmed", files: map[string]string{".key_a": "  sk-one\n"}, want: "sk-one"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for n, c := range tc.files {
				writeKey(t, dir, n, c, now)
			}
			p := &Provider{APIKeyEnv: "K", APIKeyFile: filepath.Join(dir, ".key_*")}
			if got := p.StoredKey(); got != tc.want {
				t.Fatalf("StoredKey() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestStoredKeyNewestMatchWins(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	writeKey(t, dir, ".key_old", "sk-old\n", now.Add(-time.Hour))
	writeKey(t, dir, ".key_new", "sk-new\n", now)
	writeKey(t, dir, ".key_mid", "sk-mid\n", now.Add(-time.Minute))
	p := &Provider{APIKeyEnv: "K", APIKeyFile: filepath.Join(dir, ".key_*")}
	if got := p.StoredKey(); got != "sk-new" {
		t.Fatalf("StoredKey() = %q, want the newest file's key", got)
	}
}

func TestStoredKeyExpandsEnvRefs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeKey(t, home, ".key_x", "sk-home", time.Now())
	p := &Provider{APIKeyEnv: "K", APIKeyFile: "$HOME/.key_*"}
	if got := p.StoredKey(); got != "sk-home" {
		t.Fatalf("StoredKey() = %q, want sk-home", got)
	}
}

func TestStoredKeyNoPatternOrNilProvider(t *testing.T) {
	if got := (&Provider{APIKeyEnv: "K"}).StoredKey(); got != "" {
		t.Fatalf("no apiKeyFile: StoredKey() = %q, want empty", got)
	}
	var p *Provider
	if got := p.StoredKey(); got != "" {
		t.Fatalf("nil provider: StoredKey() = %q, want empty", got)
	}
	bad := &Provider{APIKeyEnv: "K", APIKeyFile: "[unclosed"}
	if got := bad.StoredKey(); got != "" {
		t.Fatalf("malformed pattern: StoredKey() = %q, want empty", got)
	}
}

func TestStoredKeySkipsDirectories(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".key_dir"), 0o700); err != nil {
		t.Fatal(err)
	}
	p := &Provider{APIKeyEnv: "K", APIKeyFile: filepath.Join(dir, ".key_*")}
	if got := p.StoredKey(); got != "" {
		t.Fatalf("StoredKey() = %q, want empty for a directory match", got)
	}
}

func TestKeyResolver(t *testing.T) {
	dir := t.TempDir()
	writeKey(t, dir, ".key_a", "sk-stored\n", time.Now())
	p := &Provider{APIKeyEnv: "K", APIKeyFile: filepath.Join(dir, ".key_*")}
	set := func(name string) string {
		if name == "K" {
			return "sk-set"
		}
		return ""
	}
	other := func(name string) string {
		if name == "OTHER" {
			return "other-value"
		}
		return ""
	}

	if got := p.KeyResolver(set, "http://127.0.0.1:8084/v1")("K"); got != "sk-set" {
		t.Errorf("a set variable should win over the file, got %q", got)
	}
	if got := p.KeyResolver(noEnv, "http://localhost:8888/v1")("K"); got != "sk-stored" {
		t.Errorf("the file should fill an unset variable for a local endpoint, got %q", got)
	}
	if got := p.KeyResolver(noEnv, "")("K"); got != "sk-stored" {
		t.Errorf("an empty base URL counts as local, got %q", got)
	}
	if got := p.KeyResolver(noEnv, "https://studio.example.com/v1")("K"); got != "" {
		t.Errorf("the file must not be read for a remote endpoint, got %q", got)
	}
	r := p.KeyResolver(other, "http://127.0.0.1:8084/v1")
	if got := r("OTHER"); got != "other-value" {
		t.Errorf("other names should pass through, got %q", got)
	}
	if got := r("UNRELATED"); got != "" {
		t.Errorf("an unrelated unset name should stay unset, got %q", got)
	}

	noFile := &Provider{APIKeyEnv: "K"}
	if got := noFile.KeyResolver(noEnv, "")("K"); got != "" {
		t.Errorf("a provider with no key file should leave the key unset, got %q", got)
	}
}

func TestUnslothStoredKeyIsReferencedNotWritten(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	auth := filepath.Join(home, ".unsloth", "studio", "auth")
	if err := os.MkdirAll(auth, 0o700); err != nil {
		t.Fatal(err)
	}
	writeKey(t, auth, ".cli_api_key_cli_0123abcd", "sk-unsloth-stored\n", time.Now())

	cat, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	p := cat.Providers["unsloth"]
	if !strings.HasPrefix(p.APIKeyFile, "$HOME/.unsloth/studio/auth/") {
		t.Fatalf("unsloth apiKeyFile = %q, want Studio's auth directory", p.APIKeyFile)
	}

	block, _, err := BuildProviderBlock("unsloth", p, "m", "http://127.0.0.1:8084/v1", noEnv)
	if err != nil {
		t.Fatal(err)
	}
	opts, _ := block["options"].(map[string]any)
	if opts["apiKey"] != EnvRef("UNSLOTH_API_KEY") {
		t.Fatalf("opencode apiKey = %v, want the env reference", opts["apiKey"])
	}

	pi, _, err := BuildPiProvider("unsloth", p, "m", "http://127.0.0.1:8084/v1", noEnv)
	if err != nil {
		t.Fatal(err)
	}
	if pi.APIKey != "$UNSLOTH_API_KEY" {
		t.Fatalf("Pi apiKey = %q, want $UNSLOTH_API_KEY", pi.APIKey)
	}

	// Remote endpoint: the stored key is not consulted, and opencode still
	// gets a reference, never the secret.
	remote, _, err := BuildProviderBlock("unsloth", p, "m", "https://studio.example.com/v1", noEnv)
	if err != nil {
		t.Fatal(err)
	}
	ropts, _ := remote["options"].(map[string]any)
	if ropts["apiKey"] != EnvRef("UNSLOTH_API_KEY") {
		t.Fatalf("remote opencode apiKey = %v, want the env reference", ropts["apiKey"])
	}
}
