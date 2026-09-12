package hf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInferProvider(t *testing.T) {
	cases := []struct {
		name    string
		files   []string
		library string
		tags    []string
		want    string
		wantErr string
	}{
		{
			name:  "a GGUF repo is for llama.cpp",
			files: []string{"model-Q4_K_M.gguf", "README.md"},
			want:  ProviderLlamaCpp,
		},
		{
			name:    "files beat tags: GGUF and safetensors is for llama.cpp",
			files:   []string{"model-Q4_K_M.gguf", "model.safetensors"},
			library: "transformers",
			want:    ProviderLlamaCpp,
		},
		{
			name:    "a repo published for MLX by library is for oMLX",
			files:   []string{"model.safetensors"},
			library: "mlx",
			want:    ProviderOMLX,
		},
		{
			name:  "a repo tagged mlx is for oMLX",
			files: []string{"model.safetensors"},
			tags:  []string{"transformers", "MLX"},
			want:  ProviderOMLX,
		},
		{
			name:  "plain safetensors is for vLLM",
			files: []string{"model-00001.safetensors", "model-00002.safetensors", "config.json"},
			want:  ProviderVLLM,
		},
		{
			name:    "a GGUF repo is for llama.cpp even when also tagged mlx",
			files:   []string{"model-Q4_K_M.gguf"},
			tags:    []string{"mlx"},
			library: "mlx",
			want:    ProviderLlamaCpp,
		},
		{
			name:    "a repo that is not a model fails saying what it holds",
			files:   []string{"README.md", "data.txt"},
			library: "transformers",
			wantErr: "no local engine loads",
		},
		{
			name:    "an empty repo fails saying it holds no files",
			wantErr: "no files",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, why, err := InferProvider(tc.files, tc.library, tc.tags)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("InferProvider = %q (%q), want an error", got, why)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %q, want it to say %q", err, tc.wantErr)
				}
				for _, p := range []string{ProviderLlamaCpp, ProviderOMLX, ProviderVLLM} {
					if !strings.Contains(err.Error(), p) {
						t.Errorf("error = %q does not name the inferable provider %s", err, p)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("InferProvider: %v", err)
			}
			if got != tc.want {
				t.Errorf("provider = %q, want %q", got, tc.want)
			}
			if why == "" {
				t.Error("no reason given for the provider")
			}
		})
	}
}

func TestQuantOf(t *testing.T) {
	cases := map[string]string{
		"Qwen3.6-35B-A3B-Q4_K_M.gguf":      "Q4_K_M",
		"model-UD-Q4_K_XL.gguf":            "UD-Q4_K_XL",
		"model-IQ2_XXS.gguf":               "IQ2_XXS",
		"model-MXFP4.gguf":                 "MXFP4",
		"model-F16.gguf":                   "F16",
		"model-BF16.gguf":                  "BF16",
		"model-F32.gguf":                   "F32",
		"model-Q8_0-00001-of-00003.gguf":   "Q8_0",
		"Q4_K_M/model-00001-of-00003.gguf": "Q4_K_M",
		"Q4_K_M/model.gguf":                "Q4_K_M",
		"sub/Q4_K_S/00001-of-00002.gguf":   "Q4_K_S",
		"model.gguf":                       "",
		"model-Q4.gguf":                    "",
		"gpt2-35B-Q4K.gguf":                "",
		"Qwen3.6-35B-A3B-gguf/q4_k_m.gguf": "",
	}
	for in, want := range cases {
		if got := QuantOf(in); got != want {
			t.Errorf("QuantOf(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseQuantsGroupsShardsIntoOneChoice(t *testing.T) {
	files := []string{
		"model-Q8_0.gguf",
		"model-Q4_K_M-00001-of-00003.gguf",
		"model-Q4_K_M-00003-of-00003.gguf",
		"model-Q4_K_M-00002-of-00003.gguf",
		"config.json",
	}
	quants := ParseQuants(files)
	if len(quants) != 2 {
		t.Fatalf("ParseQuants = %v, want two choices", quants)
	}
	if quants[0].Name != "Q4_K_M" || len(quants[0].Files) != 3 {
		t.Errorf("first choice = %+v, want the sharded Q4_K_M as one choice", quants[0])
	}
	if quants[1].Name != "Q8_0" || len(quants[1].Files) != 1 {
		t.Errorf("second choice = %+v, want Q8_0", quants[1])
	}
}

func TestSelectQuant(t *testing.T) {
	offered := []Quant{
		{Name: "Q8_0", Files: []string{"model-Q8_0.gguf"}},
		{Name: "Q4_K_M", Files: []string{"model-Q4_K_M.gguf"}},
	}
	t.Run("a named quant wins, case-insensitively", func(t *testing.T) {
		q, err := SelectQuant(offered, "q8_0")
		if err != nil || q.Name != "Q8_0" {
			t.Errorf("SelectQuant = %+v, %v; want Q8_0", q, err)
		}
	})
	t.Run("the default prefers Q4_K_M", func(t *testing.T) {
		q, err := SelectQuant(offered, "")
		if err != nil || q.Name != "Q4_K_M" {
			t.Errorf("SelectQuant = %+v, %v; want Q4_K_M", q, err)
		}
	})
	t.Run("a quant the repo does not have fails listing what it does", func(t *testing.T) {
		_, err := SelectQuant(offered, "Q3_K_XXL")
		if err == nil {
			t.Fatal("SelectQuant: a missing quant was accepted")
		}
		for _, n := range []string{"Q3_K_XXL", "Q4_K_M", "Q8_0"} {
			if !strings.Contains(err.Error(), n) {
				t.Errorf("error = %q does not name %s", err, n)
			}
		}
	})
	t.Run("outside the preference order, the smallest non-full-precision wins", func(t *testing.T) {
		only := []Quant{
			{Name: "F16", Files: []string{"model-F16.gguf"}},
			{Name: "Q2_K", Files: []string{"model-Q2_K.gguf"}},
		}
		q, err := SelectQuant(only, "")
		if err != nil || q.Name != "Q2_K" {
			t.Errorf("SelectQuant = %+v, %v; want Q2_K over F16", q, err)
		}
	})
	t.Run("a size tie breaks on the name", func(t *testing.T) {
		tied := []Quant{
			{Name: "Q4_K_XL", Files: []string{"a.gguf"}},
			{Name: "Q4_0", Files: []string{"b.gguf"}},
		}
		q, err := SelectQuant(tied, "")
		if err != nil || q.Name != "Q4_0" {
			t.Errorf("SelectQuant = %+v, %v; want Q4_0 by name", q, err)
		}
	})
}

func TestDeriveAliasDropsThePackagingSuffix(t *testing.T) {
	cases := map[Repo]string{
		{Owner: "unsloth", Name: "Qwen3.6-35B-A3B-GGUF"}: "qwen3.6-35b-a3b",
		{Owner: "mlx-c", Name: "Llama-3.2-1B-MLX"}:       "llama-3.2-1b",
		{Owner: "org", Name: "PlainModel"}:               "plainmodel",
	}
	for repo, want := range cases {
		if got := DeriveAlias(repo); got != want {
			t.Errorf("DeriveAlias(%+v) = %q, want %q", repo, got, want)
		}
	}
}

// resolveRepo runs Resolve over a fixed repo, the caches at temp dirs.
func resolveRepo(t *testing.T, info RepoInfo, ref Ref, opts Options) (Resolved, error) {
	t.Helper()
	return Resolve(info, ref, Roots{}, opts)
}

func TestResolve(t *testing.T) {
	t.Run("a GGUF repo becomes a Spinloop's values", func(t *testing.T) {
		info := RepoInfo{
			Files:   []string{"model-Q4_K_M.gguf", "model-Q8_0.gguf", "config.json"},
			Library: "llama.cpp",
			Window:  262144,
		}
		ref, err := ParseRef("unsloth/Qwen3.6-35B-A3B-GGUF")
		if err != nil {
			t.Fatal(err)
		}
		res, err := resolveRepo(t, info, ref, Options{})
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if res.Provider != ProviderLlamaCpp {
			t.Errorf("provider = %q, want llamacpp", res.Provider)
		}
		if res.Quant != "Q4_K_M" {
			t.Errorf("quant = %q, want the default Q4_K_M", res.Quant)
		}
		if strings.Join(res.Quants, ",") != "Q4_K_M,Q8_0" {
			t.Errorf("quants = %v, want the alternatives beside it", res.Quants)
		}
		if res.Model != "unsloth/Qwen3.6-35B-A3B-GGUF:Q4_K_M" {
			t.Errorf("model = %q, want the repo reference with the quant", res.Model)
		}
		if res.Alias != "qwen3.6-35b-a3b" {
			t.Errorf("alias = %q, want the derived name", res.Alias)
		}
		if res.Context != "262144" || res.ContextSource == "" {
			t.Errorf("context = %q (%q), want the declared window with its source", res.Context, res.ContextSource)
		}
	})
	t.Run("a repo carrying both GGUF and safetensors can point at either engine", func(t *testing.T) {
		info := RepoInfo{
			Files:   []string{"model-Q4_K_M.gguf", "model.safetensors"},
			Library: "transformers",
		}
		ref, err := ParseRef("org/both")
		if err != nil {
			t.Fatal(err)
		}
		res, err := resolveRepo(t, info, ref, Options{Provider: ProviderVLLM})
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if res.Provider != ProviderVLLM {
			t.Errorf("provider = %q, want the -p override honoured", res.Provider)
		}
		if res.Model != "org/both" {
			t.Errorf("model = %q, want the bare repo reference", res.Model)
		}
		if res.Quant != "" {
			t.Errorf("quant = %q, want none: vLLM loads the whole repo", res.Quant)
		}
	})
	t.Run("the reference's suffix beats -q, and -q beats the default", func(t *testing.T) {
		info := RepoInfo{Files: []string{"model-Q4_K_M.gguf", "model-Q8_0.gguf"}}
		ref, err := ParseRef("org/model:Q8_0")
		if err != nil {
			t.Fatal(err)
		}
		res, err := resolveRepo(t, info, ref, Options{Quant: "Q4_K_M"})
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if res.Quant != "Q8_0" {
			t.Errorf("quant = %q, want the reference's Q8_0", res.Quant)
		}
		res, err = resolveRepo(t, info, Ref{Owner: "org", Name: "model", Revision: "main"}, Options{Quant: "Q8_0"})
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if res.Quant != "Q8_0" {
			t.Errorf("quant = %q, want -q's Q8_0 over the default", res.Quant)
		}
	})
	t.Run("a sharded quant's MODEL loads the whole set", func(t *testing.T) {
		cache := t.TempDir()
		info := RepoInfo{Files: []string{
			"model-Q4_K_M-00001-of-00003.gguf",
			"model-Q4_K_M-00002-of-00003.gguf",
			"model-Q4_K_M-00003-of-00003.gguf",
		}}
		ref := Ref{Owner: "org", Name: "model", Revision: "main"}
		res, err := resolveRepo(t, info, ref, Options{})
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if res.Model != "org/model:Q4_K_M" {
			t.Errorf("model = %q, want the repo reference with the sharded quant", res.Model)
		}
		_ = cache
	})
	t.Run("a repo declaring no window writes no context", func(t *testing.T) {
		info := RepoInfo{Files: []string{"model-Q4_K_M.gguf"}}
		ref := Ref{Owner: "org", Name: "model", Revision: "main"}
		res, err := resolveRepo(t, info, ref, Options{})
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if res.Context != "" {
			t.Errorf("context = %q, want none: a window is never invented", res.Context)
		}
	})
	t.Run("a repo matching no provider fails", func(t *testing.T) {
		info := RepoInfo{Files: []string{"README.md", "data.csv"}}
		ref := Ref{Owner: "org", Name: "not-a-model", Revision: "main"}
		if _, err := resolveRepo(t, info, ref, Options{}); err == nil {
			t.Fatal("Resolve: a non-model repo was accepted")
		}
	})
	t.Run("-c overrides the declared window with the lenient format", func(t *testing.T) {
		info := RepoInfo{Files: []string{"model-Q4_K_M.gguf"}, Window: 262144}
		ref := Ref{Owner: "org", Name: "model", Revision: "main"}
		res, err := resolveRepo(t, info, ref, Options{Context: "32k"})
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if res.Context != "32000" {
			t.Errorf("context = %q, want 32000 from 32k", res.Context)
		}
	})
	t.Run("an unparseable -c fails", func(t *testing.T) {
		info := RepoInfo{Files: []string{"model-Q4_K_M.gguf"}}
		ref := Ref{Owner: "org", Name: "model", Revision: "main"}
		if _, err := resolveRepo(t, info, ref, Options{Context: "wide"}); err == nil {
			t.Fatal("Resolve: an unparseable -c was accepted")
		}
	})
	t.Run("-q on a repo without quantisations fails", func(t *testing.T) {
		info := RepoInfo{Files: []string{"model.safetensors"}, Library: "mlx"}
		ref := Ref{Owner: "org", Name: "mlx-model", Revision: "main"}
		if _, err := resolveRepo(t, info, ref, Options{Quant: "Q4_K_M"}); err == nil {
			t.Fatal("Resolve: -q on a repo without quantisations was accepted")
		}
	})
	t.Run("a file named in the reference selects its quantisation", func(t *testing.T) {
		info := RepoInfo{Files: []string{"model-Q4_K_M.gguf", "model-Q8_0.gguf"}}
		ref := Ref{Owner: "org", Name: "model", Revision: "main", File: "model-Q8_0.gguf"}
		res, err := resolveRepo(t, info, ref, Options{})
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if res.Quant != "Q8_0" {
			t.Errorf("quant = %q, want the named file's Q8_0", res.Quant)
		}
	})
	t.Run("a file the repo does not have fails", func(t *testing.T) {
		info := RepoInfo{Files: []string{"model-Q4_K_M.gguf"}}
		ref := Ref{Owner: "org", Name: "model", Revision: "main", File: "model-Q8_0.gguf"}
		if _, err := resolveRepo(t, info, ref, Options{}); err == nil {
			t.Fatal("Resolve: a missing file was accepted")
		}
	})
}

// cachedGGUF builds a Hugging Face cache holding repo's first Q4_K_M file.
func cachedGGUF(t *testing.T, root, owner, name string) {
	t.Helper()
	makeSnapshot(t, root, owner, name, "main", "abc123",
		[]string{owner + "-" + name + "-Q4_K_M.gguf"}, nil, nil)
}

func TestResolvePrefersACopyOnDisk(t *testing.T) {
	info := RepoInfo{Files: []string{"unsloth-Qwen-Q4_K_M.gguf"}}
	ref := Ref{Owner: "unsloth", Name: "Qwen", Revision: "main"}

	t.Run("a cached GGUF is named directly", func(t *testing.T) {
		hubCache, llama := t.TempDir(), t.TempDir()
		cachedGGUF(t, hubCache, "unsloth", "Qwen")
		res, err := Resolve(info, ref, Roots{HFHub: hubCache, LLama: llama}, Options{})
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if filepath.Base(res.Model) != "unsloth-Qwen-Q4_K_M.gguf" {
			t.Errorf("model = %q, want the path on disk", res.Model)
		}
		if res.ModelSource != "the Hugging Face cache" {
			t.Errorf("source = %q, want the Hugging Face cache named", res.ModelSource)
		}
		if !filepath.IsAbs(res.Model) {
			t.Errorf("model = %q, want an absolute path on disk", res.Model)
		}
	})
	t.Run("llama.cpp's own cache counts too", func(t *testing.T) {
		hubCache, llama := t.TempDir(), t.TempDir()
		if err := os.WriteFile(filepath.Join(llama, "unsloth-Qwen-Q4_K_M.gguf"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
		res, err := Resolve(info, ref, Roots{HFHub: hubCache, LLama: llama}, Options{})
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if filepath.Base(res.Model) != "unsloth-Qwen-Q4_K_M.gguf" {
			t.Errorf("model = %q, want the path in llama.cpp's cache", res.Model)
		}
		if res.ModelSource != "llama.cpp's cache" {
			t.Errorf("source = %q, want llama.cpp's cache named", res.ModelSource)
		}
	})
	t.Run("--no-cache keeps the repo reference, noting the copy exists", func(t *testing.T) {
		hubCache, llama := t.TempDir(), t.TempDir()
		cachedGGUF(t, hubCache, "unsloth", "Qwen")
		res, err := Resolve(info, ref, Roots{HFHub: hubCache, LLama: llama}, Options{NoCache: true})
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if res.Model != "unsloth/Qwen:Q4_K_M" {
			t.Errorf("model = %q, want the portable repo reference", res.Model)
		}
		if res.ModelSource != "" {
			t.Errorf("source = %q, want none: no path was written", res.ModelSource)
		}
		if res.CachedPath == "" || res.CachedIn == "" {
			t.Errorf("the cached copy is not reported for the narration: %+v", res)
		}
	})
	t.Run("an uncached model keeps the repo reference", func(t *testing.T) {
		res, err := Resolve(info, ref, Roots{HFHub: t.TempDir(), LLama: t.TempDir()}, Options{})
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if res.Model != "unsloth/Qwen:Q4_K_M" {
			t.Errorf("model = %q, want the repo reference", res.Model)
		}
		if res.CachedPath != "" {
			t.Errorf("cached path = %q, want none", res.CachedPath)
		}
	})
	t.Run("oMLX and vLLM keep the repo reference even when a copy could exist", func(t *testing.T) {
		info := RepoInfo{Files: []string{"model.safetensors"}, Library: "mlx"}
		ref := Ref{Owner: "mlx-c", Name: "Llama", Revision: "main"}
		res, err := Resolve(info, ref, Roots{HFHub: t.TempDir(), LLama: t.TempDir()}, Options{})
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if res.Model != "mlx-c/Llama" {
			t.Errorf("model = %q, want the repo reference: these engines load a repo, not a file", res.Model)
		}
	})
}
