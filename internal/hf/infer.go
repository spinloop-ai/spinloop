package hf

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/spinloop-ai/spinloop/internal/contextsize"
)

// The providers spinloop hf can infer from a repo's contents. They are the
// catalogue's self-hosted engine names, fixed by the inference rules rather
// than read from the catalogue — a leaf package cannot import it.
const (
	ProviderLlamaCpp = "llamacpp"
	ProviderOMLX     = "omlx"
	ProviderVLLM     = "vllm"
)

// HasGGUF reports whether a file list holds at least one .gguf.
func HasGGUF(files []string) bool { return hasExtension(files, ".gguf") }

// HasSafetensors reports whether a file list holds at least one .safetensors.
func HasSafetensors(files []string) bool { return hasExtension(files, ".safetensors") }

func hasExtension(files []string, ext string) bool {
	for _, f := range files {
		if strings.EqualFold(filepath.Ext(f), ext) {
			return true
		}
	}
	return false
}

func hasTag(tags []string, tag string) bool {
	for _, t := range tags {
		if strings.EqualFold(t, tag) {
			return true
		}
	}
	return false
}

// describeFiles says what a file list appears to hold: the extensions it
// carries, or "no files" when it is empty.
func describeFiles(files []string) string {
	if len(files) == 0 {
		return "no files"
	}
	exts := map[string]bool{}
	for _, f := range files {
		if ext := filepath.Ext(f); ext != "" {
			exts[strings.ToLower(ext)] = true
		}
	}
	if len(exts) == 0 {
		return "files of no type spinloop recognises"
	}
	list := make([]string, 0, len(exts))
	for e := range exts {
		list = append(list, e)
	}
	sort.Strings(list)
	return "files of type " + strings.Join(list, ", ")
}

// InferProvider picks the engine a repo's files and library point at: a repo
// that publishes GGUF files is for llama.cpp, one published for MLX is for
// oMLX, and plain safetensors is for vLLM. It returns the provider and the
// reason it was chosen, or an error naming what the repo appears to hold and
// which providers can be inferred. Files come before the library and tags,
// because a repo's tags are freely edited and its files are not.
func InferProvider(files []string, library string, tags []string) (string, string, error) {
	if HasGGUF(files) {
		return ProviderLlamaCpp, "the repo publishes GGUF files", nil
	}
	if library == "mlx" || hasTag(tags, "mlx") {
		return ProviderOMLX, "the repo is published for MLX", nil
	}
	if HasSafetensors(files) {
		return ProviderVLLM, "the repo publishes safetensors weights", nil
	}
	return "", "", fmt.Errorf(
		"the repo appears to hold %s, which no local engine loads — spinloop hf can infer %s, %s or %s from a model's files; name one with -p",
		describeFiles(files), ProviderLlamaCpp, ProviderOMLX, ProviderVLLM)
}

// Quant is one quantisation a repo offers: its name and the files that make
// it up — a sharded quant is several numbered files, one choice.
type Quant struct {
	Name  string
	Files []string
}

// shardSuffix is a GGUF shard marker, -00001-of-00003.
var shardSuffix = regexp.MustCompile(`-\d+-of-\d+$`)

// quantTokenIn matches a quantisation name sitting inside a file or directory
// name, between dashes or at the edges: Q4_K_M, IQ4_XS, UD-Q4_K_XL, MXFP4,
// F16, BF16, F32. A bare Q4 is not a quantisation, so the Q/IQ forms need
// their underscore tail.
var quantTokenIn = regexp.MustCompile(`(?:^|-)((?:UD-)?(?:IQ\d+(?:_[A-Za-z0-9]+)*|Q\d+(?:_[A-Za-z0-9]+)+|MXFP4|F16|BF16|F32))(?:$|-)`)

// quantTokenWhole matches a name that is exactly a quantisation, for the
// directory-level form (Q4_K_M/model.gguf).
var quantTokenWhole = regexp.MustCompile(`^(?:UD-)?(?:IQ\d+(?:_[A-Za-z0-9]+)*|Q\d+(?:_[A-Za-z0-9]+)+|MXFP4|F16|BF16|F32)$`)

// QuantOf names the quantisation a GGUF path carries, or "" when it carries
// none of the recognised forms. A directory level that is itself a quant name
// wins over the file's own name: a repo that puts each quant in its own
// directory (Q4_K_M/model.gguf) uses it.
func QuantOf(relPath string) string {
	slash := filepath.ToSlash(relPath)
	base := slash
	if i := strings.LastIndexByte(slash, '/'); i >= 0 {
		base = slash[i+1:]
		dirs := strings.Split(slash[:i], "/")
		for j := len(dirs) - 1; j >= 0; j-- {
			if quantTokenWhole.MatchString(dirs[j]) {
				return dirs[j]
			}
		}
	}
	if strings.HasSuffix(strings.ToLower(base), ".gguf") {
		base = base[:len(base)-len(".gguf")]
	}
	base = shardSuffix.ReplaceAllString(base, "")
	if m := quantTokenIn.FindString(base); m != "" {
		return strings.TrimPrefix(m, "-")
	}
	return ""
}

// ParseQuants groups a file list into the quantisations it offers: every
// .gguf whose path carries a quant name joins that quant's group, a sharded
// quant landing in one choice. The groups come back sorted by name, their
// files sorted.
func ParseQuants(files []string) []Quant {
	groups := map[string][]string{}
	for _, f := range files {
		if !strings.EqualFold(filepath.Ext(f), ".gguf") {
			continue
		}
		q := QuantOf(f)
		if q == "" {
			continue
		}
		groups[q] = append(groups[q], f)
	}
	names := make([]string, 0, len(groups))
	for n := range groups {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]Quant, 0, len(names))
	for _, n := range names {
		fs := groups[n]
		sort.Strings(fs)
		out = append(out, Quant{Name: n, Files: fs})
	}
	return out
}

// quantPreference is the documented default order for a repo that offers
// several quantisations and none was named: roughly 4-bit is the size that
// fits the machines people run local models on.
var quantPreference = []string{"Q4_K_M", "UD-Q4_K_XL", "Q4_K_S", "Q5_K_M", "Q6_K", "Q8_0"}

// SelectQuant chooses among the quantisations a repo offers. want, when it
// names one, wins — matched case-insensitively, so q4_k_m finds Q4_K_M.
// Otherwise the documented preference order is tried, then the smallest
// non-full-precision group, the name as tie-break. A want the repo does not
// offer fails, listing the quantisations it does.
func SelectQuant(offered []Quant, want string) (Quant, error) {
	if len(offered) == 0 {
		return Quant{}, fmt.Errorf("the repo offers no quantisation to choose")
	}
	if want != "" {
		for _, q := range offered {
			if strings.EqualFold(q.Name, want) {
				return q, nil
			}
		}
		return Quant{}, fmt.Errorf("the repo offers no %s quantisation — it offers: %s", want, joinQuantNames(offered))
	}
	for _, pref := range quantPreference {
		for _, q := range offered {
			if strings.EqualFold(q.Name, pref) {
				return q, nil
			}
		}
	}
	candidates := make([]Quant, 0, len(offered))
	for _, q := range offered {
		if p := quantPrecision(q.Name); p > 0 && p < 16 {
			candidates = append(candidates, q)
		}
	}
	if len(candidates) == 0 {
		candidates = offered
	}
	sort.Slice(candidates, func(i, j int) bool {
		pi, pj := quantPrecision(candidates[i].Name), quantPrecision(candidates[j].Name)
		if pi != pj {
			return pi < pj
		}
		return candidates[i].Name < candidates[j].Name
	})
	return candidates[0], nil
}

// quantPrecision is a quant's bits per weight, 0 when it cannot be read: the
// number after Q, IQ, UD-Q or in MXFPn, F16/BF16 as 16 and F32 as 32.
func quantPrecision(name string) int {
	upper := strings.ToUpper(name)
	switch {
	case strings.HasPrefix(upper, "F32"):
		return 32
	case strings.HasPrefix(upper, "F16"), strings.HasPrefix(upper, "BF16"):
		return 16
	}
	m := precisionRe.FindStringSubmatch(upper)
	if m == nil {
		return 0
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0
	}
	return n
}

var precisionRe = regexp.MustCompile(`(?:UD-)?(?:IQ|Q|MXFP)(\d+)`)

// joinQuantNames lists a group of quantisations for an error message.
func joinQuantNames(quants []Quant) string {
	names := make([]string, 0, len(quants))
	for _, q := range quants {
		names = append(names, q.Name)
	}
	return strings.Join(names, ", ")
}

// packagingSuffixes mark a repo name's packaging rather than its model, and
// are dropped when an alias is derived from the name.
var packagingSuffixes = []string{"-GGUF", "-MLX"}

// DeriveAlias gives a repo a short name: the repo's name lower-cased, with a
// packaging suffix such as -GGUF or -MLX dropped.
func DeriveAlias(repo Repo) string {
	name := repo.Name
	for _, suf := range packagingSuffixes {
		if strings.HasSuffix(strings.ToLower(name), strings.ToLower(suf)) {
			name = name[:len(name)-len(suf)]
			break
		}
	}
	return strings.ToLower(name)
}

// Options are the overrides the command hands the resolver: each inference
// can be beaten by its flag.
type Options struct {
	// Provider is the -p value: it beats the inference outright.
	Provider string
	// Quant is the -q value: it beats the reference's :QUANT suffix and the
	// default preference.
	Quant string
	// Context is the -c value, parsed by the same lenient size format every
	// other command uses; it beats the window the model declares.
	Context string
	// Alias is the -a value: it beats the name derived from the repo.
	Alias string
	// NoCache keeps the MODEL a repo reference even when a copy is cached,
	// so the Spinloop stays portable.
	NoCache bool
}

// Resolved is a repo turned into the values a Spinloop states, each with the
// reasoning for it, so the command can render the Spinloop and explain it
// separately.
type Resolved struct {
	Ref  Ref
	Repo Repo

	// Provider and why it was chosen.
	Provider    string
	ProviderWhy string

	// Quant is the chosen quantisation, "" when the provider takes no
	// quantisation; Quants is every name the repo offers, for the
	// alternatives beside it.
	Quant  string
	Quants []string

	// Model is the MODEL to write. ModelSource is where it came from when it
	// is a file on disk ("the Hugging Face cache" or "llama.cpp's cache"),
	// "" when the engine will download it from the Hub.
	Model       string
	ModelSource string

	// CachedPath is a copy of the chosen model found in a local cache, even
	// when --no-cache keeps the MODEL a repo reference; CachedIn names the
	// cache it came from.
	CachedPath string
	CachedIn   string

	// Alias is the ALIAS to write.
	Alias string

	// Context is the CONTEXT to write, "" when the model declares no window
	// and none was given; ContextSource is where it came from.
	Context       string
	ContextSource string
}

// Resolve turns a repo's metadata into the values a Spinloop states, applying
// the options' overrides in their documented order. It consults the caches
// named by roots to prefer a copy already on disk; it never downloads
// anything — fetching weights stays the engine's job.
func Resolve(info RepoInfo, ref Ref, roots Roots, opts Options) (Resolved, error) {
	repo := ref.Repo()
	r := Resolved{Ref: ref, Repo: repo}

	if opts.Provider != "" {
		r.Provider, r.ProviderWhy = opts.Provider, "named with -p"
	} else {
		provider, why, err := InferProvider(info.Files, info.Library, info.Tags)
		if err != nil {
			return Resolved{}, err
		}
		r.Provider, r.ProviderWhy = provider, why
	}

	if r.Provider == ProviderLlamaCpp {
		quants := ParseQuants(info.Files)
		if len(quants) == 0 {
			return Resolved{}, fmt.Errorf(
				"the repo publishes no GGUF files for %s to load — it appears to hold %s; name a provider that can load it with -p",
				ProviderLlamaCpp, describeFiles(info.Files))
		}
		r.Quants = make([]string, len(quants))
		for i, q := range quants {
			r.Quants[i] = q.Name
		}

		// The choice, in order: a file the reference names, then the
		// reference's :QUANT suffix, then -q, then the default.
		want := ""
		switch {
		case ref.File != "":
			found := false
			for _, q := range quants {
				if contains(q.Files, ref.File) {
					want, found = q.Name, true
					break
				}
			}
			if !found {
				return Resolved{}, fmt.Errorf("the repo has no file %q at revision %s", ref.File, ref.Revision)
			}
			if opts.Quant != "" && !strings.EqualFold(opts.Quant, want) {
				return Resolved{}, fmt.Errorf("the named file belongs to %s, not the %s -q asks for", want, opts.Quant)
			}
		case ref.Quant != "":
			want = ref.Quant
		case opts.Quant != "":
			want = opts.Quant
		}
		chosen, err := SelectQuant(quants, want)
		if err != nil {
			return Resolved{}, err
		}
		r.Quant = chosen.Name

		if path, cache := cachedLlamaFile(roots, ref, chosen); path != "" {
			r.CachedPath, r.CachedIn = path, cache
			if opts.NoCache {
				r.Model = repo.String() + ":" + chosen.Name
			} else {
				r.Model, r.ModelSource = path, cache
			}
		} else {
			r.Model = repo.String() + ":" + chosen.Name
		}
	} else {
		if ref.Quant != "" || opts.Quant != "" {
			return Resolved{}, fmt.Errorf(
				"%s loads a whole repo, not one quantisation — drop the %s", r.Provider,
				orSuffix(ref.Quant, opts.Quant))
		}
		// A repo or directory is what oMLX and vLLM load: the MODEL stays the
		// repo reference whether or not a copy is cached.
		r.Model = repo.String()
	}

	if opts.Alias != "" {
		r.Alias = opts.Alias
	} else {
		r.Alias = DeriveAlias(repo)
	}

	switch {
	case opts.Context != "":
		n, err := contextsize.Parse(opts.Context)
		if err != nil {
			return Resolved{}, fmt.Errorf("--context: %w", err)
		}
		r.Context, r.ContextSource = strconv.Itoa(n), "the --context flag"
	case info.Window > 0:
		r.Context, r.ContextSource = strconv.Itoa(info.Window), "the model's config.json"
	default:
		// No window declared and none given: no CONTEXT line, and the
		// narration says so rather than inventing a number.
	}
	return r, nil
}

// cachedLlamaFile looks a chosen quant's first file up in the local caches:
// the Hugging Face cache first — its snapshot names the file — then
// llama.cpp's own. It returns the path on disk and which cache it came from.
// The first file is what the engine needs: given the first shard of a split
// GGUF, llama-server loads the rest itself.
func cachedLlamaFile(roots Roots, ref Ref, q Quant) (string, string) {
	if snap, ok := FindSnapshot(roots.HFHub, ref.Repo(), ref.Revision); ok {
		if p, ok := snap.File(q.Files[0]); ok {
			return p, "the Hugging Face cache"
		}
	}
	if p, ok := FindLLamaGGUF(roots.LLama, ref.Repo(), q.Name); ok {
		return p, "llama.cpp's cache"
	}
	return "", ""
}

// orSuffix names the spelling of a quantisation that was given: the
// reference's suffix when it was in the reference, else the -q flag.
func orSuffix(suffix, flag string) string {
	if suffix != "" {
		return ":" + suffix + " / -q"
	}
	return "-q"
}

func contains(files []string, f string) bool {
	for _, x := range files {
		if x == f {
			return true
		}
	}
	return false
}
