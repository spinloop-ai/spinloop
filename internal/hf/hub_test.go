package hf

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// stubHub is a Hugging Face Hub for tests: it answers the metadata endpoint
// and the config.json resolve path with whatever it is told, counts its
// requests, and records the bearer each request carried.
type stubHub struct {
	*httptest.Server
	requests int
	lastAuth string
}

func newStubHub(t *testing.T, meta string, metaStatus int, config string, configStatus int) *stubHub {
	t.Helper()
	h := &stubHub{}
	h.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.requests++
		h.lastAuth = r.Header.Get("Authorization")
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/models/"):
			w.WriteHeader(metaStatus)
			fmt.Fprint(w, meta)
		case strings.Contains(r.URL.Path, "/resolve/") && strings.HasSuffix(r.URL.Path, "/config.json"):
			w.WriteHeader(configStatus)
			fmt.Fprint(w, config)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(h.Close)
	return h
}

// isolateHfEnv points every Hub and cache the package can reach at test-owned
// locations: the endpoint at the stub, the token at the given value ("" for
// none, with the token file kept out of reach), and the caches at temp dirs.
func isolateHfEnv(t *testing.T, endpoint, hubCache, llamaCache, token string) {
	t.Helper()
	t.Setenv("HF_ENDPOINT", endpoint)
	t.Setenv("HF_HUB_CACHE", hubCache)
	t.Setenv("LLAMA_CACHE", llamaCache)
	t.Setenv("HF_HOME", t.TempDir())
	t.Setenv("HF_TOKEN", token)
	t.Setenv("HUGGING_FACE_HUB_TOKEN", "")
}

func TestFetchReadsMetadataAndConfig(t *testing.T) {
	hubCache, llamaCache := t.TempDir(), t.TempDir()
	h := newStubHub(t,
		`{"siblings":[{"rfilename":"b-Q8_0.gguf"},{"rfilename":"a/Q4_K_M.gguf"},{"rfilename":""}],"tags":["gguf","transformers"],"library_name":"llama.cpp","sha":"abc123"}`,
		http.StatusOK,
		`{"max_position_embeddings": 262144}`, http.StatusOK)
	isolateHfEnv(t, h.URL, hubCache, llamaCache, "")

	info, err := NewClient(h.URL).Fetch(Repo{Owner: "unsloth", Name: "Qwen"}, "main", Roots{HFHub: hubCache, LLama: llamaCache})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	want := RepoInfo{
		Files:   []string{"a/Q4_K_M.gguf", "b-Q8_0.gguf"},
		Tags:    []string{"gguf", "transformers"},
		Library: "llama.cpp",
		SHA:     "abc123",
		Window:  262144,
	}
	if fmt.Sprintf("%+v", info) != fmt.Sprintf("%+v", want) {
		t.Errorf("Fetch = %+v, want %+v", info, want)
	}
	if h.requests != 2 {
		t.Errorf("requests = %d, want 2 (metadata and config.json)", h.requests)
	}
}

func TestWindowFallbacks(t *testing.T) {
	cases := []struct {
		name   string
		config string
		status int
		window int
	}{
		{"declared", `{"max_position_embeddings": 262144}`, http.StatusOK, 262144},
		{"declared as a string", `{"max_position_embeddings": "4096"}`, http.StatusOK, 4096},
		{"text_config fallback", `{"architectures": ["M"], "text_config": {"max_position_embeddings": 32768}}`, http.StatusOK, 32768},
		{"text_config fallback as a string", `{"text_config": {"max_position_embeddings": "8192"}}`, http.StatusOK, 8192},
		{"no window field", `{"architectures": ["M"]}`, http.StatusOK, 0},
		{"config.json absent", "", http.StatusNotFound, 0},
		{"config.json not a config", "not json", http.StatusOK, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hubCache, llamaCache := t.TempDir(), t.TempDir()
			h := newStubHub(t, `{"siblings":[]}`, http.StatusOK, tc.config, tc.status)
			isolateHfEnv(t, h.URL, hubCache, llamaCache, "")

			info, err := NewClient(h.URL).Fetch(Repo{Owner: "o", Name: "m"}, "main", Roots{HFHub: hubCache, LLama: llamaCache})
			if err != nil {
				t.Fatalf("Fetch: %v", err)
			}
			if info.Window != tc.window {
				t.Errorf("window = %d, want %d", info.Window, tc.window)
			}
		})
	}
}

func TestFetchFailureMappings(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		hubCache, llamaCache := t.TempDir(), t.TempDir()
		h := newStubHub(t, `{"error":"Not Found"}`, http.StatusNotFound, "{}", http.StatusOK)
		isolateHfEnv(t, h.URL, hubCache, llamaCache, "")
		_, err := NewClient(h.URL).Fetch(Repo{Owner: "unsloth", Name: "Qwen"}, "main", Roots{})
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound", err)
		}
		if !strings.Contains(err.Error(), "unsloth/Qwen") {
			t.Errorf("err %q does not name the repo", err)
		}
	})
	t.Run("gated without a token says how to authenticate", func(t *testing.T) {
		hubCache, llamaCache := t.TempDir(), t.TempDir()
		h := newStubHub(t, `{"error":"restricted"}`, http.StatusUnauthorized, "{}", http.StatusOK)
		isolateHfEnv(t, h.URL, hubCache, llamaCache, "")
		_, err := NewClient(h.URL).Fetch(Repo{Owner: "o", Name: "m"}, "main", Roots{})
		if !errors.Is(err, ErrGated) {
			t.Fatalf("err = %v, want ErrGated", err)
		}
		if !strings.Contains(err.Error(), "gated or private") || !strings.Contains(err.Error(), "HF_TOKEN") {
			t.Errorf("err %q does not say gated or how to authenticate", err)
		}
	})
	t.Run("gated with a token does not repeat the hint", func(t *testing.T) {
		hubCache, llamaCache := t.TempDir(), t.TempDir()
		h := newStubHub(t, `{"error":"restricted"}`, http.StatusForbidden, "{}", http.StatusOK)
		isolateHfEnv(t, h.URL, hubCache, llamaCache, "secret-token")
		_, err := NewClient(h.URL).Fetch(Repo{Owner: "o", Name: "m"}, "main", Roots{})
		if !errors.Is(err, ErrGated) {
			t.Fatalf("err = %v, want ErrGated", err)
		}
		if strings.Contains(err.Error(), "HF_TOKEN") {
			t.Errorf("err %q repeats the authenticate hint although a token was resolved", err)
		}
		if strings.Contains(err.Error(), "secret-token") {
			t.Errorf("err %q contains the token", err)
		}
	})
	t.Run("rate limited", func(t *testing.T) {
		hubCache, llamaCache := t.TempDir(), t.TempDir()
		h := newStubHub(t, "", http.StatusTooManyRequests, "{}", http.StatusOK)
		isolateHfEnv(t, h.URL, hubCache, llamaCache, "")
		_, err := NewClient(h.URL).Fetch(Repo{Owner: "o", Name: "m"}, "main", Roots{})
		if !errors.Is(err, ErrRateLimited) {
			t.Fatalf("err = %v, want ErrRateLimited", err)
		}
	})
	t.Run("unreachable names the endpoint", func(t *testing.T) {
		hubCache, llamaCache := t.TempDir(), t.TempDir()
		slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(300 * time.Millisecond)
		}))
		t.Cleanup(slow.Close)
		isolateHfEnv(t, slow.URL, hubCache, llamaCache, "")
		_, err := NewClientWithTimeout(slow.URL, 50*time.Millisecond).Fetch(Repo{Owner: "o", Name: "m"}, "main", Roots{})
		if !errors.Is(err, ErrUnreachable) {
			t.Fatalf("err = %v, want ErrUnreachable", err)
		}
		if !strings.Contains(err.Error(), slow.URL) {
			t.Errorf("err %q does not name the endpoint", err)
		}
	})
	t.Run("an unexpected status is named, not bare", func(t *testing.T) {
		hubCache, llamaCache := t.TempDir(), t.TempDir()
		h := newStubHub(t, "", http.StatusInternalServerError, "{}", http.StatusOK)
		isolateHfEnv(t, h.URL, hubCache, llamaCache, "")
		_, err := NewClient(h.URL).Fetch(Repo{Owner: "o", Name: "m"}, "main", Roots{})
		if !errors.Is(err, ErrUnparseable) {
			t.Fatalf("err = %v, want ErrUnparseable", err)
		}
		if !strings.Contains(err.Error(), "o/m") {
			t.Errorf("err %q does not name the repo", err)
		}
	})
	t.Run("metadata that is not JSON", func(t *testing.T) {
		hubCache, llamaCache := t.TempDir(), t.TempDir()
		h := newStubHub(t, "not json", http.StatusOK, "{}", http.StatusOK)
		isolateHfEnv(t, h.URL, hubCache, llamaCache, "")
		_, err := NewClient(h.URL).Fetch(Repo{Owner: "o", Name: "m"}, "main", Roots{})
		if !errors.Is(err, ErrUnparseable) {
			t.Fatalf("err = %v, want ErrUnparseable", err)
		}
	})
}

func TestTokenResolution(t *testing.T) {
	hubCache, llamaCache := t.TempDir(), t.TempDir()
	h := newStubHub(t, `{"siblings":[]}`, http.StatusOK, `{"max_position_embeddings": 8}`, http.StatusOK)
	repo := Repo{Owner: "o", Name: "m"}

	t.Run("the bearer is sent when a variable holds a token", func(t *testing.T) {
		isolateHfEnv(t, h.URL, hubCache, llamaCache, "var-token")
		c := NewClient(h.URL)
		if _, err := c.Fetch(repo, "main", Roots{}); err != nil {
			t.Fatalf("Fetch: %v", err)
		}
		if h.lastAuth != "Bearer var-token" {
			t.Errorf("Authorization = %q, want %q", h.lastAuth, "Bearer var-token")
		}
	})
	t.Run("the second variable ranks below the first", func(t *testing.T) {
		isolateHfEnv(t, h.URL, hubCache, llamaCache, "first")
		t.Setenv("HUGGING_FACE_HUB_TOKEN", "second")
		c := NewClient(h.URL)
		if _, err := c.Fetch(repo, "main", Roots{}); err != nil {
			t.Fatalf("Fetch: %v", err)
		}
		if h.lastAuth != "Bearer first" {
			t.Errorf("Authorization = %q, want %q", h.lastAuth, "Bearer first")
		}
	})
	t.Run("the CLI's token file ranks below the variables", func(t *testing.T) {
		home := t.TempDir()
		isolateHfEnv(t, h.URL, hubCache, llamaCache, "")
		t.Setenv("HF_HOME", home)
		if err := os.WriteFile(filepath.Join(home, "token"), []byte("file-token\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		c := NewClient(h.URL)
		if _, err := c.Fetch(repo, "main", Roots{}); err != nil {
			t.Fatalf("Fetch: %v", err)
		}
		if h.lastAuth != "Bearer file-token" {
			t.Errorf("Authorization = %q, want %q", h.lastAuth, "Bearer file-token")
		}
	})
	t.Run("a public repo goes out unauthenticated", func(t *testing.T) {
		isolateHfEnv(t, h.URL, hubCache, llamaCache, "")
		c := NewClient(h.URL)
		info, err := c.Fetch(repo, "main", Roots{})
		if err != nil {
			t.Fatalf("Fetch: %v", err)
		}
		if h.lastAuth != "" {
			t.Errorf("Authorization = %q, want none", h.lastAuth)
		}
		got := fmt.Sprintf("%+v", info)
		if strings.Contains(got, "token") {
			t.Errorf("a produced value carries the word token: %s", got)
		}
	})
	t.Run("the token is absent from every produced value", func(t *testing.T) {
		isolateHfEnv(t, h.URL, hubCache, llamaCache, "top-secret-token")
		c := NewClient(h.URL)
		info, err := c.Fetch(repo, "main", Roots{})
		if err != nil {
			t.Fatalf("Fetch: %v", err)
		}
		for _, s := range []string{fmt.Sprintf("%+v", info), c.endpoint} {
			if strings.Contains(s, "top-secret-token") {
				t.Errorf("a produced value contains the token: %s", s)
			}
		}
	})
}

func TestEndpointResolvesFromEnvironment(t *testing.T) {
	t.Setenv("HF_ENDPOINT", "")
	if got := Endpoint(); got != DefaultEndpoint {
		t.Errorf("Endpoint() = %q, want the default %q", got, DefaultEndpoint)
	}
	t.Setenv("HF_ENDPOINT", "https://mirror.example.com/")
	if got := Endpoint(); got != "https://mirror.example.com" {
		t.Errorf("Endpoint() = %q, want the mirror without a trailing slash", got)
	}
}
