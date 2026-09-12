package hf

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// makeSnapshot builds a Hugging Face cache tree under root for repo at
// revision: a refs entry naming sha, a snapshots/{sha} directory whose links
// for present files resolve to real blobs, and whose links for missing files
// dangle. incomplete blobs are written with the .incomplete suffix a
// half-finished download leaves, with no link naming the finished file.
func makeSnapshot(t *testing.T, root, owner, name, revision, sha string, present, missing, incomplete []string) {
	t.Helper()
	repoDir := filepath.Join(root, "models--"+owner+"--"+name)
	refs := filepath.Join(repoDir, "refs")
	if revision != "" {
		if err := os.MkdirAll(refs, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(refs, revision), []byte(sha+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	snapDir := filepath.Join(repoDir, "snapshots", sha)
	blobs := filepath.Join(repoDir, "blobs")
	if err := os.MkdirAll(blobs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeEntry := func(rel string, blobName string, dangling bool) {
		target := filepath.Join(blobs, blobName)
		if !dangling {
			if err := os.WriteFile(target, []byte("weights"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		dest := filepath.Join(snapDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, dest); err != nil {
			t.Fatal(err)
		}
	}
	for i, f := range present {
		writeEntry(f, fmt.Sprintf("blob-%d", i), false)
	}
	for i, f := range missing {
		writeEntry(f, fmt.Sprintf("gone-%d", i), true)
	}
	for i, f := range incomplete {
		// A half-finished download: the blob exists only under its
		// .incomplete name, so the link to the finished file dangles.
		if err := os.WriteFile(filepath.Join(blobs, fmt.Sprintf("half-%d.incomplete", i)), []byte("partial"), 0o644); err != nil {
			t.Fatal(err)
		}
		writeEntry(f, fmt.Sprintf("half-%d", i), true)
	}
}

func TestFindSnapshotFindsADownloadedModel(t *testing.T) {
	root := t.TempDir()
	makeSnapshot(t, root, "unsloth", "Qwen", "main", "abc123",
		[]string{"Qwen-Q4_K_M.gguf", "config.json"}, nil, nil)

	snap, ok := FindSnapshot(root, Repo{Owner: "unsloth", Name: "Qwen"}, "main")
	if !ok {
		t.Fatal("FindSnapshot: the cache holds the repo, not reported")
	}
	if snap.SHA != "abc123" {
		t.Errorf("SHA = %q, want abc123", snap.SHA)
	}
	p, ok := snap.File("Qwen-Q4_K_M.gguf")
	if !ok {
		t.Fatal("File: the snapshot holds the file, not reported")
	}
	if data, err := os.ReadFile(p); err != nil || string(data) != "weights" {
		t.Errorf("the reported path does not read as the file on disk (%v, %q)", err, data)
	}
	files, err := snap.Files()
	if err != nil {
		t.Fatalf("Files: %v", err)
	}
	want := []string{"Qwen-Q4_K_M.gguf", "config.json"}
	if strings.Join(files, ",") != strings.Join(want, ",") {
		t.Errorf("Files = %v, want %v", files, want)
	}
}

func TestFindSnapshotFindsARevisionNamedByItsSHA(t *testing.T) {
	root := t.TempDir()
	// No refs entry: the reference named the commit directly, so the
	// snapshot directory carries the sha.
	makeSnapshot(t, root, "unsloth", "Qwen", "", "e1d7ec4aabbccddeeeffff00011122233344455",
		[]string{"Qwen-Q4_K_M.gguf"}, nil, nil)

	snap, ok := FindSnapshot(root, Repo{Owner: "unsloth", Name: "Qwen"}, "e1d7ec4aabbccddeeeffff00011122233344455")
	if !ok {
		t.Fatal("FindSnapshot: the snapshot is named by the sha, not reported")
	}
	if _, ok := snap.File("Qwen-Q4_K_M.gguf"); !ok {
		t.Error("File: the snapshot holds the file, not reported")
	}
}

func TestADanglingSnapshotIsNotCached(t *testing.T) {
	root := t.TempDir()
	makeSnapshot(t, root, "unsloth", "Qwen", "main", "abc123",
		nil, []string{"Qwen-Q4_K_M.gguf"}, nil)

	snap, ok := FindSnapshot(root, Repo{Owner: "unsloth", Name: "Qwen"}, "main")
	if !ok {
		t.Fatal("FindSnapshot: the snapshot directory exists")
	}
	if _, ok := snap.File("Qwen-Q4_K_M.gguf"); ok {
		t.Error("File: a dangling link was reported as a cached file")
	}
	files, err := snap.Files()
	if err != nil {
		t.Fatalf("Files: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("Files = %v, want none: an interrupted download is not a model", files)
	}
}

func TestAnIncompleteBlobIsNotCached(t *testing.T) {
	root := t.TempDir()
	makeSnapshot(t, root, "unsloth", "Qwen", "main", "abc123",
		nil, nil, []string{"Qwen-Q4_K_M.gguf"})

	snap, ok := FindSnapshot(root, Repo{Owner: "unsloth", Name: "Qwen"}, "main")
	if !ok {
		t.Fatal("FindSnapshot: the snapshot directory exists")
	}
	if _, ok := snap.File("Qwen-Q4_K_M.gguf"); ok {
		t.Error("File: a .incomplete blob was reported as a cached file")
	}
}

func TestFindLLamaGGUFMatchesAllThreeParts(t *testing.T) {
	cache := t.TempDir()
	repo := Repo{Owner: "unsloth", Name: "Qwen3.6-35B"}
	if err := os.WriteFile(filepath.Join(cache, "unsloth-Qwen3.6-35B-Q4_K_M.gguf"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	// The near-misses: each lacks one of the three parts.
	for _, name := range []string{
		"other-org-Qwen3.6-35B-Q4_K_M.gguf",
		"unsloth-OtherModel-Q4_K_M.gguf",
		"unsloth-Qwen3.6-35B-Q8_0.gguf",
		"unsloth-Qwen3.6-35B-Q4_K_M.txt",
	} {
		if err := os.WriteFile(filepath.Join(cache, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	p, ok := FindLLamaGGUF(cache, repo, "Q4_K_M")
	if !ok {
		t.Fatal("FindLLamaGGUF: the matching file is in the cache, not reported")
	}
	if filepath.Base(p) != "unsloth-Qwen3.6-35B-Q4_K_M.gguf" {
		t.Errorf("FindLLamaGGUF = %q, want the matching file", p)
	}

	// The match is case-insensitive.
	p, ok = FindLLamaGGUF(cache, Repo{Owner: "UnslOtH", Name: "qwen3.6-35b"}, "q4_k_m")
	if !ok || filepath.Base(p) != "unsloth-Qwen3.6-35B-Q4_K_M.gguf" {
		t.Errorf("case-insensitive match = %q, %v; want the same file", p, ok)
	}

	if _, ok := FindLLamaGGUF(cache, Repo{Owner: "unsloth", Name: "Qwen3.6-35B"}, "Q5_K_M"); ok {
		t.Error("FindLLamaGGUF matched a quant none of the filenames carry")
	}
}

func TestResolveRootsReadsTheEnvironmentOnce(t *testing.T) {
	t.Setenv("HF_HUB_CACHE", "/tmp/hf-hub-cache")
	t.Setenv("HF_HOME", "/tmp/hf-home")
	t.Setenv("LLAMA_CACHE", "/tmp/llama-cache")
	r := ResolveRoots()
	if r.HFHub != "/tmp/hf-hub-cache" {
		t.Errorf("HFHub = %q, want HF_HUB_CACHE to win", r.HFHub)
	}
	if r.LLama != "/tmp/llama-cache" {
		t.Errorf("LLama = %q, want LLAMA_CACHE to win", r.LLama)
	}

	os.Unsetenv("HF_HUB_CACHE")
	r = ResolveRoots()
	if r.HFHub != filepath.Join("/tmp/hf-home", "hub") {
		t.Errorf("HFHub = %q, want $HF_HOME/hub", r.HFHub)
	}

	os.Unsetenv("HF_HOME")
	home := t.TempDir()
	t.Setenv("HOME", home)
	r = ResolveRoots()
	if r.HFHub != filepath.Join(home, ".cache", "huggingface", "hub") {
		t.Errorf("HFHub = %q, want the conventional location under the home", r.HFHub)
	}
}

func TestAFullyCachedModelResolvesWithoutNetwork(t *testing.T) {
	hubCache := t.TempDir()
	makeSnapshot(t, hubCache, "unsloth", "Qwen", "main", "abc123",
		[]string{"Qwen-Q4_K_M.gguf"}, nil, nil)
	if err := os.WriteFile(
		filepath.Join(hubCache, "models--unsloth--Qwen", "snapshots", "abc123", "config.json"),
		[]byte(`{"max_position_embeddings": 262144}`), 0o644); err != nil {
		t.Fatal(err)
	}
	// The stub answers with the wrong shape on purpose: any request to it
	// means the cache was not enough.
	h := newStubHub(t, `{"wrong":true}`, http.StatusOK, `{"wrong":true}`, http.StatusOK)
	isolateHfEnv(t, h.URL, hubCache, t.TempDir(), "")

	info, err := NewClient(h.URL).Fetch(Repo{Owner: "unsloth", Name: "Qwen"}, "main", Roots{HFHub: hubCache})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if h.requests != 0 {
		t.Errorf("requests = %d, want 0: a fully cached model costs no network", h.requests)
	}
	if info.Window != 262144 {
		t.Errorf("window = %d, want 262144 from the cached config.json", info.Window)
	}
	if !HasGGUF(info.Files) {
		t.Errorf("files = %v, want the cached GGUF", info.Files)
	}
}

func TestACachedModelWithNoGGUFStillReadsTheTags(t *testing.T) {
	hubCache := t.TempDir()
	makeSnapshot(t, hubCache, "mlx-c", "Llama", "main", "abc123",
		[]string{"model.safetensors"}, nil, nil)
	if err := os.WriteFile(
		filepath.Join(hubCache, "models--mlx-c--Llama", "snapshots", "abc123", "config.json"),
		[]byte(`{"max_position_embeddings": 8192}`), 0o644); err != nil {
		t.Fatal(err)
	}
	h := newStubHub(t, `{"siblings":[{"rfilename":"model.safetensors"}],"tags":["mlx"],"library_name":"mlx","sha":"abc123"}`,
		http.StatusOK, `{"max_position_embeddings": 8192}`, http.StatusOK)
	isolateHfEnv(t, h.URL, hubCache, t.TempDir(), "")

	info, err := NewClient(h.URL).Fetch(Repo{Owner: "mlx-c", Name: "Llama"}, "main", Roots{HFHub: hubCache})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if h.requests != 1 {
		t.Errorf("requests = %d, want 1 (the tags the cache does not hold)", h.requests)
	}
	if info.Library != "mlx" {
		t.Errorf("library = %q, want mlx from the Hub", info.Library)
	}
	if info.Window != 8192 {
		t.Errorf("window = %d, want 8192 from the cached config.json", info.Window)
	}
}

func TestAnEmptySnapshotFallsBackToTheHub(t *testing.T) {
	hubCache := t.TempDir()
	makeSnapshot(t, hubCache, "unsloth", "Qwen", "main", "abc123",
		nil, []string{"Qwen-Q4_K_M.gguf"}, nil)
	h := newStubHub(t, `{"siblings":[{"rfilename":"Qwen-Q4_K_M.gguf"}],"library_name":"llama.cpp","sha":"abc123"}`,
		http.StatusOK, `{"max_position_embeddings": 4096}`, http.StatusOK)
	isolateHfEnv(t, h.URL, hubCache, t.TempDir(), "")

	info, err := NewClient(h.URL).Fetch(Repo{Owner: "unsloth", Name: "Qwen"}, "main", Roots{HFHub: hubCache})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if h.requests != 2 {
		t.Errorf("requests = %d, want 2: a snapshot with nothing readable is not cached", h.requests)
	}
	if !HasGGUF(info.Files) {
		t.Errorf("files = %v, want the Hub's file list", info.Files)
	}
}
