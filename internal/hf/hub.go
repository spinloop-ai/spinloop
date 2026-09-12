package hf

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// DefaultEndpoint is the Hugging Face Hub's public host, used when
// HF_ENDPOINT names no other.
const DefaultEndpoint = "https://huggingface.co"

// requestTimeout bounds one Hub request. Reading a repo is a couple of small
// JSON fetches: a reachable Hub answers in well under this, and an
// unreachable one fails fast enough not to strand the command.
const requestTimeout = 10 * time.Second

// maxBody caps one Hub response body. The two endpoints read are small JSON
// documents; anything larger is not an answer spinloop can use.
const maxBody = 16 << 20

// The failure kinds, for errors.Is: each names what went wrong, and every
// error a read returns carries its repo (and, for a transport failure, the
// endpoint too), so none surfaces as a bare status or transport error.
var (
	ErrNotFound    = errors.New("no such model repo on the Hub")
	ErrGated       = errors.New("the model is gated or private")
	ErrRateLimited = errors.New("the Hub is rate limiting requests")
	ErrUnreachable = errors.New("the Hub could not be reached")
	ErrUnparseable = errors.New("the Hub's response could not be understood")
)

// Endpoint is the Hub's base URL: HF_ENDPOINT when it names one — a mirror or
// a private deployment — else the public host.
func Endpoint() string {
	if e := strings.TrimRight(os.Getenv("HF_ENDPOINT"), "/"); e != "" {
		return e
	}
	return DefaultEndpoint
}

// tokenVars are the variables a Hugging Face token may sit in, in precedence
// order.
var tokenVars = []string{"HF_TOKEN", "HUGGING_FACE_HUB_TOKEN"}

// Home is the Hugging Face home directory: $HF_HOME, else
// ~/.cache/huggingface. It holds the token file the Hugging Face CLI writes.
func Home() string {
	if h := os.Getenv("HF_HOME"); h != "" {
		return h
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".cache", "huggingface")
}

// ResolveToken looks for a Hugging Face token: HF_TOKEN, then
// HUGGING_FACE_HUB_TOKEN, then the token file the Hugging Face CLI writes
// under its home. It returns "" when no token exists, in which case requests
// go out unauthenticated and public repos work as before. A resolved token is
// sent only as the request's bearer credential — never printed, and never
// written to any file spinloop produces.
func ResolveToken() string {
	for _, v := range tokenVars {
		if t := os.Getenv(v); t != "" {
			return t
		}
	}
	h := Home()
	if h == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(h, "token"))
	if err != nil {
		return ""
	}
	// The file holds one token, usually with a trailing newline.
	for _, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return ""
}

// Client reads one Hub: its base URL, the token it sends (when one was
// resolved), and a bounded-timeout HTTP client with no retries. It holds no
// state between requests, so one serves a whole command.
type Client struct {
	endpoint string
	token    string
	http     *http.Client
}

// NewClient builds a Client for endpoint with the conventional timeout.
// Endpoint() gives the conventional host; a test points it at a stub.
func NewClient(endpoint string) *Client {
	return NewClientWithTimeout(endpoint, requestTimeout)
}

// NewClientWithTimeout builds a Client for endpoint whose requests give up
// after timeout.
func NewClientWithTimeout(endpoint string, timeout time.Duration) *Client {
	return &Client{
		endpoint: strings.TrimRight(endpoint, "/"),
		token:    ResolveToken(),
		http:     &http.Client{Timeout: timeout},
	}
}

// DefaultClient is a Client for the conventional endpoint — HF_ENDPOINT or
// https://huggingface.co — with whatever token the machine has.
func DefaultClient() *Client {
	return NewClient(Endpoint())
}

// RepoInfo is what a model repo publishes at one revision: its files
// (repo-relative paths, sorted), its tags, the library it was published with,
// the commit sha the revision names, and the window its config.json declares
// (0 when it declares none). Nothing in it is a weights file: reading a repo
// is metadata only.
type RepoInfo struct {
	Files   []string
	Tags    []string
	Library string
	SHA     string
	Window  int
}

// Fetch reads repo at revision, preferring the local caches: when the repo's
// snapshot is complete in the Hugging Face cache, its file list and its
// config.json come from disk and no request is made. The tags — which no
// cache holds — are read from the Hub only when the files do not already name
// a GGUF repo, since they matter only for spotting an MLX one.
func (c *Client) Fetch(repo Repo, revision string, roots Roots) (RepoInfo, error) {
	info := RepoInfo{}
	if snap, ok := FindSnapshot(roots.HFHub, repo, revision); ok {
		files, err := snap.Files()
		if err != nil {
			return RepoInfo{}, err
		}
		info.Files = files
		if path, ok := snap.File("config.json"); ok {
			if data, err := os.ReadFile(path); err == nil {
				info.Window = WindowFromConfig(data)
			}
		}
	}
	if len(info.Files) == 0 {
		meta, err := c.meta(repo, revision)
		if err != nil {
			return RepoInfo{}, err
		}
		info.Files, info.Tags, info.Library, info.SHA = meta.Files, meta.Tags, meta.Library, meta.SHA
	} else if !HasGGUF(info.Files) {
		meta, err := c.meta(repo, revision)
		if err != nil {
			return RepoInfo{}, err
		}
		info.Tags, info.Library, info.SHA = meta.Tags, meta.Library, meta.SHA
	}
	if info.Window == 0 {
		w, err := c.window(repo, revision)
		if err != nil {
			return RepoInfo{}, err
		}
		info.Window = w
	}
	return info, nil
}

// meta reads the repo metadata from the Hub's API: the file list from
// siblings, plus tags, library_name and sha.
func (c *Client) meta(repo Repo, revision string) (RepoInfo, error) {
	u := c.endpoint + "/api/models/" + url.PathEscape(repo.Owner) + "/" + url.PathEscape(repo.Name) +
		"/revision/" + url.PathEscape(revision)
	body, err := c.get(u, repo.String())
	if err != nil {
		return RepoInfo{}, err
	}
	var payload struct {
		Siblings []struct {
			RFilename string `json:"rfilename"`
		} `json:"siblings"`
		Tags        []string `json:"tags"`
		LibraryName string   `json:"library_name"`
		SHA         string   `json:"sha"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return RepoInfo{}, fmt.Errorf("%w: the Hub's metadata for %s is not what it should be (from %s)", ErrUnparseable, repo, c.endpoint)
	}
	info := RepoInfo{Tags: payload.Tags, Library: payload.LibraryName, SHA: payload.SHA}
	for _, s := range payload.Siblings {
		if s.RFilename != "" {
			info.Files = append(info.Files, s.RFilename)
		}
	}
	sort.Strings(info.Files)
	return info, nil
}

// window reads the repo's config.json and returns the window it declares. A
// missing file is no declared window, not an error.
func (c *Client) window(repo Repo, revision string) (int, error) {
	u := c.endpoint + "/" + url.PathEscape(repo.Owner) + "/" + url.PathEscape(repo.Name) +
		"/resolve/" + url.PathEscape(revision) + "/config.json"
	body, err := c.get(u, repo.String())
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return 0, nil
		}
		return 0, err
	}
	return WindowFromConfig(body), nil
}

// WindowFromConfig reads the window a model config.json declares:
// max_position_embeddings, falling back to text_config.max_position_embeddings
// for a multimodal config. It returns 0 when the file states no window or
// does not parse as one.
func WindowFromConfig(data []byte) int {
	var cfg struct {
		MaxPositionEmbeddings any `json:"max_position_embeddings"`
		TextConfig            *struct {
			MaxPositionEmbeddings any `json:"max_position_embeddings"`
		} `json:"text_config"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return 0
	}
	if n, ok := configInt(cfg.MaxPositionEmbeddings); ok {
		return n
	}
	if cfg.TextConfig != nil {
		if n, ok := configInt(cfg.TextConfig.MaxPositionEmbeddings); ok {
			return n
		}
	}
	return 0
}

// configInt reads a config value written as a number or a string of digits.
func configInt(v any) (int, bool) {
	switch x := v.(type) {
	case float64:
		if x == float64(int64(x)) && x > 0 {
			return int(x), true
		}
	case string:
		if n, err := strconv.Atoi(x); err == nil && n > 0 {
			return n, true
		}
	}
	return 0, false
}

// get performs one GET of the Hub and returns the body, mapping each failure
// to a named error that says what went wrong and names the repo. A token is
// sent as the request's bearer only when one was resolved.
func (c *Client) get(rawURL, repo string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: naming the request for %s: %v", ErrUnparseable, repo, err)
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return nil, fmt.Errorf("%w: the Hub at %s did not answer in time for %s", ErrUnreachable, c.endpoint, repo)
		}
		return nil, fmt.Errorf("%w: the Hub at %s could not be reached for %s", ErrUnreachable, c.endpoint, repo)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, fmt.Errorf("%w: reading the Hub's response for %s (from %s)", ErrUnparseable, repo, c.endpoint)
	}
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return body, nil
	case resp.StatusCode == http.StatusNotFound:
		return nil, fmt.Errorf("%w: no model repo %s on the Hub (from %s)", ErrNotFound, repo, c.endpoint)
	case resp.StatusCode == http.StatusUnauthorized, resp.StatusCode == http.StatusForbidden:
		msg := fmt.Sprintf("model %s is gated or private", repo)
		if c.token == "" {
			msg += " — set HF_TOKEN, or log in with the Hugging Face CLI, to read it"
		}
		return nil, fmt.Errorf("%w: %s", ErrGated, msg)
	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, fmt.Errorf("%w: the Hub is rate limiting requests for %s — wait a moment and try again", ErrRateLimited, repo)
	default:
		return nil, fmt.Errorf("%w: the Hub's response for %s could not be understood (status %d from %s)",
			ErrUnparseable, repo, resp.StatusCode, c.endpoint)
	}
}
