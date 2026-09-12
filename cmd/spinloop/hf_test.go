package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spinloop-ai/spinloop/internal/spinloop"
)

// hfStubHub is a Hugging Face Hub good enough for the command: repo
// metadata plus the config.json the window is read from.
type hfStubHub struct {
	server *httptest.Server
}

func newHfStubHub(t *testing.T) *hfStubHub {
	t.Helper()
	hub := &hfStubHub{}
	hub.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/models/"):
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"siblings":[{"rfilename":"Qwen3.6-35B-A3B-Q4_K_M.gguf"},`+
				`{"rfilename":"Qwen3.6-35B-A3B-Q8_0.gguf"},`+
				`{"rfilename":"config.json"}],"tags":["gguf"],"library_name":"llama.cpp","sha":"abc123"}`)
		case strings.HasSuffix(r.URL.Path, "/config.json"):
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"max_position_embeddings": 262144}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(hub.server.Close)
	return hub
}

// hfEnv isolates the home, the caches and the token the command reads.
func hfEnv(t *testing.T) (home, hubCache, llamaCache string) {
	t.Helper()
	home = isolateConfig(t)
	hubCache, llamaCache = t.TempDir(), t.TempDir()
	t.Setenv("HF_HOME", t.TempDir())
	t.Setenv("HF_HUB_CACHE", hubCache)
	t.Setenv("LLAMA_CACHE", llamaCache)
	t.Setenv("HF_TOKEN", "")
	t.Setenv("HUGGING_FACE_HUB_TOKEN", "")
	return home, hubCache, llamaCache
}

// makeCachedGGUF plants a finished download of owner/name in the Hugging
// Face cache: a main ref and a snapshot holding the one gguf.
func makeCachedGGUF(t *testing.T, hubCache, owner, name, file string) {
	t.Helper()
	repoDir := filepath.Join(hubCache, "models--"+owner+"--"+name)
	if err := os.MkdirAll(filepath.Join(repoDir, "refs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "refs", "main"), []byte("abc123\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repoDir, "snapshots", "abc123"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "snapshots", "abc123", file), nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestHfPrintsTheSpinloopAndSaysWhatItInferred(t *testing.T) {
	hfEnv(t)
	hub := newHfStubHub(t)
	t.Setenv("HF_ENDPOINT", hub.server.URL)

	out := captureStdout(t, func() {
		if err := cmdHf([]string{"unsloth/Qwen3.6-35B-A3B-GGUF"}); err != nil {
			t.Fatal(err)
		}
	})
	sel, err := spinloop.Parse([]byte(out))
	if err != nil {
		t.Fatalf("stdout did not parse as a Spinloop: %v\n%s", err, out)
	}
	if sel.Provider != "llamacpp" {
		t.Errorf("PROVIDER = %q, want llamacpp", sel.Provider)
	}
	if sel.Model != "unsloth/Qwen3.6-35B-A3B-GGUF:Q4_K_M" {
		t.Errorf("MODEL = %q, want the repo reference with the default quant", sel.Model)
	}
	if sel.Alias != "qwen3.6-35b-a3b" {
		t.Errorf("ALIAS = %q, want the derived alias", sel.Alias)
	}
	if sel.Context != "262144" {
		t.Errorf("CONTEXT = %q, want the declared window", sel.Context)
	}
	if sel.Output != "" {
		t.Errorf("OUTPUT = %q, want none: a hf Spinloop writes no output line", sel.Output)
	}
	if strings.Contains(out, "provider  ") {
		t.Errorf("stdout carries narration:\n%s", out)
	}

	errOut := captureStderr(t, func() {
		if err := cmdHf([]string{"unsloth/Qwen3.6-35B-A3B-GGUF"}); err != nil {
			t.Fatal(err)
		}
	})
	for _, want := range []string{"llamacpp", "Q4_K_M", "Q8_0", "262144"} {
		if !strings.Contains(errOut, want) {
			t.Errorf("stderr does not explain %q:\n%s", want, errOut)
		}
	}
}

func TestHfAcceptsAModelPageURL(t *testing.T) {
	hfEnv(t)
	hub := newHfStubHub(t)
	t.Setenv("HF_ENDPOINT", hub.server.URL)

	out := captureStdout(t, func() {
		if err := cmdHf([]string{"https://huggingface.co/unsloth/Qwen3.6-35B-A3B-GGUF"}); err != nil {
			t.Fatal(err)
		}
	})
	sel, err := spinloop.Parse([]byte(out))
	if err != nil {
		t.Fatalf("stdout did not parse as a Spinloop: %v\n%s", err, out)
	}
	if sel.Model != "unsloth/Qwen3.6-35B-A3B-GGUF:Q4_K_M" {
		t.Errorf("MODEL = %q, want the repo reference with the default quant", sel.Model)
	}
}

func TestHfWritesToFileInsteadOfStdout(t *testing.T) {
	hfEnv(t)
	hub := newHfStubHub(t)
	t.Setenv("HF_ENDPOINT", hub.server.URL)

	path := filepath.Join(t.TempDir(), "Spinloop")
	out := captureStdout(t, func() {
		if err := cmdHf([]string{"-o", path, "unsloth/Qwen3.6-35B-A3B-GGUF"}); err != nil {
			t.Fatal(err)
		}
	})
	if out != "" {
		t.Errorf("stdout = %q, want nothing: the file took the Spinloop", out)
	}
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the file was not written: %v", err)
	}
	sel, err := spinloop.Parse(written)
	if err != nil {
		t.Fatalf("the file did not parse as a Spinloop: %v\n%s", err, written)
	}
	if sel.Provider != "llamacpp" {
		t.Errorf("PROVIDER = %q, want llamacpp", sel.Provider)
	}

	// A second run without --force must fail and leave the file alone.
	errOut := captureStderr(t, func() {
		err := cmdHf([]string{"-o", path, "unsloth/Qwen3.6-35B-A3B-GGUF"})
		if err == nil {
			t.Fatal("a second -o run was accepted without --force")
		}
		if !strings.Contains(err.Error(), "--force") {
			t.Errorf("error = %q, want it to name --force", err)
		}
	})
	if strings.Contains(errOut, "written to") {
		t.Errorf("the file was reported written on a refused run:\n%s", errOut)
	}
	if again, _ := os.ReadFile(path); string(again) != string(written) {
		t.Error("the existing file changed on a refused run")
	}

	// --force overwrites it.
	if err := cmdHf([]string{"-o", path, "--force", "unsloth/Qwen3.6-35B-A3B-GGUF"}); err != nil {
		t.Fatalf("--force run: %v", err)
	}
	if again, _ := os.ReadFile(path); !bytes.Equal(again, written) {
		t.Error("--force did not write the same Spinloop")
	}
}

func TestHfApplyConfiguresTheHarness(t *testing.T) {
	home, _, _ := hfEnv(t)
	hub := newHfStubHub(t)
	t.Setenv("HF_ENDPOINT", hub.server.URL)

	if err := cmdHf([]string{"--apply", "unsloth/Qwen3.6-35B-A3B-GGUF"}); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	cfg := readConfigMap(t, cfgPath)
	prov, _ := cfg["provider"].(map[string]any)
	llamacpp, _ := prov["llamacpp"].(map[string]any)
	if llamacpp == nil {
		t.Fatalf("the llamacpp provider is missing from %s:\n%v", cfgPath, cfg)
	}
	models, _ := llamacpp["models"].(map[string]any)
	if models == nil || models["qwen3.6-35b-a3b"] == nil {
		t.Fatalf("the derived alias is not a model key: %v", models)
	}
}

func TestHfPrefersACachedCopyAndNoCacheKeepsTheReference(t *testing.T) {
	_, hubCache, _ := hfEnv(t)
	hub := newHfStubHub(t)
	t.Setenv("HF_ENDPOINT", hub.server.URL)
	makeCachedGGUF(t, hubCache, "unsloth", "Qwen3.6-35B-A3B-GGUF", "Qwen3.6-35B-A3B-Q4_K_M.gguf")

	out := captureStdout(t, func() {
		if err := cmdHf([]string{"unsloth/Qwen3.6-35B-A3B-GGUF"}); err != nil {
			t.Fatal(err)
		}
	})
	sel, err := spinloop.Parse([]byte(out))
	if err != nil {
		t.Fatalf("stdout did not parse as a Spinloop: %v\n%s", err, out)
	}
	if filepath.Base(sel.Model) != "Qwen3.6-35B-A3B-Q4_K_M.gguf" {
		t.Errorf("MODEL = %q, want the path of the cached copy", sel.Model)
	}
	if !filepath.IsAbs(sel.Model) {
		t.Errorf("MODEL = %q, want an absolute path", sel.Model)
	}

	errOut := captureStderr(t, func() {
		if err := cmdHf([]string{"--no-cache", "unsloth/Qwen3.6-35B-A3B-GGUF"}); err != nil {
			t.Fatal(err)
		}
	})
	out = captureStdout(t, func() {
		if err := cmdHf([]string{"--no-cache", "unsloth/Qwen3.6-35B-A3B-GGUF"}); err != nil {
			t.Fatal(err)
		}
	})
	sel, err = spinloop.Parse([]byte(out))
	if err != nil {
		t.Fatalf("stdout did not parse as a Spinloop: %v\n%s", err, out)
	}
	if sel.Model != "unsloth/Qwen3.6-35B-A3B-GGUF:Q4_K_M" {
		t.Errorf("MODEL = %q, want the repo reference under --no-cache", sel.Model)
	}
	if !strings.Contains(errOut, "stays on this machine") {
		t.Errorf("stderr does not say the cached copy stays:\n%s", errOut)
	}
}

func TestHfRequiresAReference(t *testing.T) {
	hfEnv(t)
	err := cmdHf(nil)
	if err == nil {
		t.Fatal("hf with no reference succeeded")
	}
	if !strings.Contains(err.Error(), "required") {
		t.Errorf("error = %q, want it to say a reference is required", err)
	}
}
