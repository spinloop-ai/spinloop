package hf

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Roots are the local cache directories a lookup consults, resolved once by
// the caller and passed to every lookup as an argument. Nothing in this
// package reads the environment or the home directory at the point of use: a
// test points the roots at a temp tree, and the machine's real caches stay
// untouched.
type Roots struct {
	// HFHub is the Hugging Face cache: HF_HUB_CACHE, else $HF_HOME/hub, else
	// the conventional ~/.cache/huggingface/hub — the layout the Hugging Face
	// libraries write.
	HFHub string
	// LLama is llama.cpp's own model cache: LLAMA_CACHE, else the platform
	// cache directory's llama.cpp subdirectory — where llama-server puts what
	// it downloads itself.
	LLama string
}

// ResolveRoots reads the cache roots from the environment once.
func ResolveRoots() Roots {
	r := Roots{}
	switch {
	case os.Getenv("HF_HUB_CACHE") != "":
		r.HFHub = os.Getenv("HF_HUB_CACHE")
	case os.Getenv("HF_HOME") != "":
		r.HFHub = filepath.Join(os.Getenv("HF_HOME"), "hub")
	default:
		if home, err := os.UserHomeDir(); err == nil {
			r.HFHub = filepath.Join(home, ".cache", "huggingface", "hub")
		}
	}
	switch {
	case os.Getenv("LLAMA_CACHE") != "":
		r.LLama = os.Getenv("LLAMA_CACHE")
	default:
		if cache, err := os.UserCacheDir(); err == nil {
			r.LLama = filepath.Join(cache, "llama.cpp")
		}
	}
	return r
}

// String is the repo id, owner/name — the form an engine downloads it by.
func (r Repo) String() string { return r.Owner + "/" + r.Name }

// CacheDir is the repo's directory name in the Hugging Face cache layout.
func (r Repo) CacheDir() string {
	return "models--" + strings.ReplaceAll(r.Owner, "/", "--") + "--" + strings.ReplaceAll(r.Name, "/", "--")
}

// Snapshot is a repo's on-disk state in the Hugging Face cache layout:
// models--{owner}--{name}/refs/{revision} holds the commit sha, and
// snapshots/{sha}/ holds one entry per file, each a link into blobs/.
type Snapshot struct {
	Repo     Repo
	Revision string
	SHA      string
	Dir      string
}

// FindSnapshot looks repo at revision up in the Hugging Face cache under
// hubCache. It reports the snapshot when the cache names a sha for the
// revision (or the revision is itself a sha whose snapshot directory exists)
// and that directory holds at least one entry. Whether the entries are
// genuinely present files is Files' and File's job: a snapshot whose links
// all dangle is not a cached model.
func FindSnapshot(hubCache string, repo Repo, revision string) (Snapshot, bool) {
	if hubCache == "" || repo.Owner == "" || repo.Name == "" || revision == "" {
		return Snapshot{}, false
	}
	repoDir := filepath.Join(hubCache, repo.CacheDir())
	sha := ""
	if data, err := os.ReadFile(filepath.Join(repoDir, "refs", revision)); err == nil {
		sha = strings.TrimSpace(string(data))
	}
	if sha == "" && isSHA(revision) {
		if info, err := os.Stat(filepath.Join(repoDir, "snapshots", revision)); err == nil && info.IsDir() {
			sha = revision
		}
	}
	if sha == "" {
		return Snapshot{}, false
	}
	dir := filepath.Join(repoDir, "snapshots", sha)
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		return Snapshot{}, false
	}
	return Snapshot{Repo: repo, Revision: revision, SHA: sha, Dir: dir}, true
}

// isSHA reports whether s reads as a commit sha: hex, at least seven long. A
// reference that names a sha directly has no refs entry, so its snapshot
// directory is looked up under the sha itself.
func isSHA(s string) bool {
	if len(s) < 7 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

// File resolves one repo-relative file of the snapshot to its path on disk.
// The entry counts only when the link resolves to a readable regular file —
// a dangling link, which an interrupted download leaves behind, reads as not
// cached.
func (s Snapshot) File(rel string) (string, bool) {
	p := filepath.Join(s.Dir, filepath.FromSlash(rel))
	if !isReadableRegular(p) {
		return "", false
	}
	return p, true
}

// Files lists the repo-relative paths of every file in the snapshot that is
// genuinely present and readable, in stable order. Broken entries — dangling
// links, anything not a regular file once followed — are skipped.
func (s Snapshot) Files() ([]string, error) {
	var out []string
	err := filepath.WalkDir(s.Dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		t := d.Type()
		if t.IsDir() || (!t.IsRegular() && t != fs.ModeSymlink) {
			return nil
		}
		if !isReadableRegular(path) {
			return nil
		}
		rel, err := filepath.Rel(s.Dir, path)
		if err != nil {
			return nil
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("reading the Hugging Face cache: %w", err)
	}
	sort.Strings(out)
	return out, nil
}

// isReadableRegular reports whether path resolves (following a link) to a
// regular file that opens for reading.
func isReadableRegular(p string) bool {
	info, err := os.Stat(p)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	f, err := os.Open(p)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

// FindLLamaGGUF scans llama.cpp's cache for a .gguf whose name carries the
// repo's owner, the repo's name and the quant — all three, case-insensitively.
// The filename convention llama-server writes is not a documented contract, so
// a near-match must never produce a wrong path; a miss costs nothing, since
// the engine finds its own cached copy.
func FindLLamaGGUF(llamaCache string, repo Repo, quant string) (string, bool) {
	if llamaCache == "" || repo.Owner == "" || repo.Name == "" || quant == "" {
		return "", false
	}
	entries, err := os.ReadDir(llamaCache)
	if err != nil {
		return "", false
	}
	wantOwner := strings.ToLower(repo.Owner)
	wantName := strings.ToLower(repo.Name)
	wantQuant := strings.ToLower(quant)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		lower := strings.ToLower(name)
		if !strings.HasSuffix(lower, ".gguf") {
			continue
		}
		if strings.Contains(lower, wantOwner) && strings.Contains(lower, wantName) && strings.Contains(lower, wantQuant) {
			return filepath.Join(llamaCache, name), true
		}
	}
	return "", false
}
