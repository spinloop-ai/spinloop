// The fleet gateway: one OpenAI-compatible endpoint in front of a fleet. It
// answers agent requests with the fleet's own selector, holds each node's
// engine key, and wakes a node when nothing is serving what a request asks
// for — so a machine running an agent needs nothing but a URL and one token.
//
// It is a foreground process, the way `spinloop serve` is: it holds the fleet
// file it serves, and a machine that hosts agents points its Spinloop's FLEET
// at the address it prints.

package gateway

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/spinloop-ai/spinloop/internal/daemon"
	"github.com/spinloop-ai/spinloop/internal/fleet"
	"github.com/spinloop-ai/spinloop/internal/remote"
)

// DefaultListen is where the gateway answers when --listen is not given: a
// fixed port on every interface, so a Spinloop's FLEET can name one address
// without knowing the machine it lands on.
const DefaultListen = ":4000"

// LoopbackListen is where `--loopback` binds the gateway: the default port on
// loopback, the safe bind a local-only gateway wants — one that Listen's token
// check accepts without a token.
const LoopbackListen = "127.0.0.1:4000"

// cacheTTL is how long a fan-out reading stays fresh for routing. A burst of
// requests pays one fan-out, not one each; a request older than this sees the
// fleet as it is now. A variable so tests do not wait.
var cacheTTL = 2 * time.Second

// sourcesTTL is how long the reading of what each node's own source describes
// stays fresh for the models list. A source changes on a deploy, not on a
// request, so this is far longer than the status reading's: the cost is
// reading each node's Spinloop and its preset, which a burst of models
// requests must not pay per request. A variable so tests do not wait.
var sourcesTTL = 30 * time.Second

// pathsServed is the surface the gateway answers, for the 404 that names it.
var pathsServed = []string{"/v1/models", "/v1/chat/completions", "/v1/completions", "/health"}

// Options shapes a handler beyond the fleet it serves.
type Options struct {
	// ConfigFor resolves the deploy config a candidate node would be started
	// with — each node's own Spinloop source. nil disables waking: nothing can
	// be started, and a request nothing is serving fails saying so.
	ConfigFor fleet.ConfigFor
	// Log receives the gateway's log lines; nil discards them.
	Log *slog.Logger
	// Now is the clock the reading cache ages against; nil uses time.Now.
	Now func() time.Time
}

// Handler is the gateway: the fleet it serves, the token its callers present,
// and the state a burst of requests shares — the last reading of the fleet,
// and the reading's age.
type Handler struct {
	cfg    *fleet.Config
	token  string
	cfgFor fleet.ConfigFor
	log    *slog.Logger
	now    func() time.Time

	mu         sync.Mutex
	results    []fleet.NodeResult
	at         time.Time
	wakeable   map[string]string
	wakeableAt time.Time
}

// New builds a gateway handler over a resolved fleet file. The token is the
// one callers must present — empty on loopback, where none is needed, as the
// daemon's control API allows.
func New(cfg *fleet.Config, token string, opts Options) *Handler {
	log := opts.Log
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	return &Handler{cfg: cfg, token: token, cfgFor: opts.ConfigFor, log: log, now: now}
}

// Listen opens the gateway's listener, applying the daemon's exposure rule:
// a non-loopback address is refused without a token, because it would put an
// engine's full output on the network for anyone to read.
func Listen(addr, token string) (net.Listener, error) {
	if token == "" && !loopbackAddr(addr) {
		return nil, fmt.Errorf(
			"refusing to listen on non-loopback %q without a token: "+
				"pass --api-token-file <path>, set %s, or pass --api-token — "+
				"or bind loopback, e.g. --listen 127.0.0.1:4000, which needs none",
			addr, daemon.TokenEnvVar)
	}
	return net.Listen("tcp", addr)
}

// loopbackAddr reports whether a listen address binds only loopback. An empty
// or wildcard host binds every interface, so it is not loopback.
func loopbackAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	if host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// ServeHTTP is the gateway's whole surface: the three paths it serves, a
// health check that touches no node, and a 404 that names the rest.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var handler http.HandlerFunc
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/health":
		handler = h.handleHealth
	case r.Method == http.MethodGet && r.URL.Path == "/v1/models":
		handler = h.handleModels
	case r.Method == http.MethodPost && (r.URL.Path == "/v1/chat/completions" || r.URL.Path == "/v1/completions"):
		handler = h.handleCompletion
	default:
		handler = func(w http.ResponseWriter, r *http.Request) {
			writeError(w, http.StatusNotFound, fmt.Errorf(
				"the gateway serves %s, not %s %s", strings.Join(pathsServed, ", "), r.Method, r.URL.Path))
		}
	}
	h.authenticate(handler)(w, r)
}

// authenticate gates every request behind the bearer token, on the daemon's
// terms: an empty token means no auth, which Listen permits on loopback only.
func (h *Handler) authenticate(next http.HandlerFunc) http.HandlerFunc {
	if h.token == "" {
		return next
	}
	want := []byte("Bearer " + h.token)
	return func(w http.ResponseWriter, r *http.Request) {
		got := []byte(r.Header.Get("Authorization"))
		if subtle.ConstantTimeCompare(got, want) != 1 {
			writeError(w, http.StatusUnauthorized, fmt.Errorf("missing or invalid bearer token"))
			return
		}
		next(w, r)
	}
}

// handleHealth answers that the gateway is up. It contacts no node on
// purpose: it is how an operator tells the gateway down from the fleet down.
func (h *Handler) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleModels lists what a request can reach, in the OpenAI list shape: what
// the running nodes report, and — when the fleet allows waking — what a
// stopped node's own source describes, so a client sees the models it can ask
// for. Served name first, duplicates once. Nothing reachable is an empty
// list, not an error.
func (h *Handler) handleModels(w http.ResponseWriter, r *http.Request) {
	results := h.reading(r.Context())
	wakeable := h.wakeableModels()
	seen := map[string]bool{}
	data := []map[string]any{}
	add := func(name string) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		data = append(data, map[string]any{"id": name, "object": "model"})
	}
	for _, res := range results {
		if !res.OK() || res.Status.State != string(daemon.StateRunning) {
			continue
		}
		name := res.Status.ServedName
		if name == "" {
			name = res.Status.Model
		}
		add(name)
	}
	for _, res := range results {
		if !res.OK() || res.Status.State == string(daemon.StateRunning) {
			// A running engine is never displaced to make room, so a running
			// node's source's model is not one this gateway would answer; a
			// node that does not answer its status cannot be started either.
			continue
		}
		add(wakeable[res.Name])
	}
	writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": data})
}

// wakeableModels is what a request could start: for each node, the model its
// own source describes, under the served-name-first naming a running node
// reports. It is resolved at most once per sourcesTTL, shared by every models
// request. The gateway cannot start a remote environment — one the fleet
// names by environment and a request never wakes — so a remote node
// contributes nothing here, and neither does any node when the fleet's wake is
// off or the gateway holds no way to resolve a source.
func (h *Handler) wakeableModels() map[string]string {
	if h.cfgFor == nil || !h.cfg.Wakes() {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.wakeable != nil && h.now().Sub(h.wakeableAt) < sourcesTTL {
		return h.wakeable
	}
	m := map[string]string{}
	for _, entry := range h.cfg.Nodes {
		if entry.Kind != fleet.KindDaemon {
			continue
		}
		dc, err := h.cfgFor(entry)
		if err != nil {
			continue
		}
		name := dc.ServedModelName
		if name == "" {
			name = dc.ModelID
		}
		if name != "" {
			m[entry.Name] = name
		}
	}
	h.wakeable = m
	h.wakeableAt = h.now()
	return m
}

// reading returns the fleet's last fan-out, taking one when the last is stale.
// It is the whole freshness story: a burst of requests shares one reading,
// and a request older than cacheTTL sees the fleet as it is now. The mutex is
// held for the fan-out, so requests that arrive mid-fan-out wait for it and
// take its result rather than fanning out again.
func (h *Handler) reading(ctx context.Context) []fleet.NodeResult {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.results != nil && h.now().Sub(h.at) < cacheTTL {
		return h.results
	}
	h.results = h.cfg.FanOut(ctx, fleet.StatusCall)
	h.at = h.now()
	return h.results
}

// handleCompletion routes a completion request to the node serving its model,
// waking one when nothing is and the fleet file allows it.
func (h *Handler) handleCompletion(w http.ResponseWriter, r *http.Request) {
	model, body, err := requestModel(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if model == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf(
			"the request names no model: completion requests need a `model` field"))
		return
	}
	ctx := r.Context()
	prefer, err := h.cfg.Preference("")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	want := fleet.Want{Model: model, Prefer: prefer}

	results := h.reading(ctx)
	choice, err := h.cfg.Choose(h.reachable(results), want)
	if err != nil {
		var none *fleet.ErrNoneServing
		if !errors.As(err, &none) {
			writeError(w, http.StatusBadGateway, err)
			return
		}
		choice, err = h.routeNothingServing(ctx, want, none)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, err)
			return
		}
	}

	h.log.Info("routed",
		slog.String("model", model),
		slog.String("node", choice.Node.Name),
		slog.Bool("woken", choice.Woken))
	h.proxy(w, r, body, choice)
}

// requestModel pulls the model field out of a completion request and returns
// it with the full body, which the proxy must forward unmodified. A body that
// is not a JSON object fails saying so, rather than being routed at a guess.
func requestModel(r *http.Request) (string, []byte, error) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return "", nil, fmt.Errorf("reading the request: %w", err)
	}
	var req struct {
		Model string `json:"model"`
	}
	if len(bytes.TrimSpace(body)) > 0 {
		if err := json.Unmarshal(body, &req); err != nil {
			return "", nil, fmt.Errorf("the request is not a JSON body: %v", err)
		}
	}
	return req.Model, body, nil
}

// reachable marks the running engines the gateway cannot reach as not
// candidates: a node whose engine is bound to loopback, without an override
// taking responsibility for reachability, answers only on its own machine, so
// selecting it would hold a request for the wake timeout at best. The mark
// carries the daemon's own explanation, which names the bind and the fix, so
// a fleet where that is the only match fails saying so.
func (h *Handler) reachable(results []fleet.NodeResult) []fleet.NodeResult {
	out := make([]fleet.NodeResult, len(results))
	for i, res := range results {
		out[i] = res
		if !res.OK() || res.Status.State != string(daemon.StateRunning) {
			continue
		}
		entry, ok := h.cfg.Node(res.Name)
		if !ok {
			continue
		}
		if _, err := h.cfg.EngineBaseURL(entry, res.Status); err != nil {
			out[i] = fleet.NodeResult{
				Name:    res.Name,
				Outcome: fleet.OutcomeUnreachable,
				Err:     err,
				Status:  res.Status,
				At:      res.At,
			}
		}
	}
	return out
}

// routeNothingServing answers a request that nothing in the fleet is serving.
// A node already running the wanted model whose engine has not answered yet is
// waited for rather than passed over: it is loading the weights this request
// needs, so it serves sooner than a node started from cold, and waking a
// second node would leave two engines up for one request.
func (h *Handler) routeNothingServing(ctx context.Context, want fleet.Want, none *fleet.ErrNoneServing) (*fleet.Choice, error) {
	if starting := h.cfg.Loading(none.Results, want); len(starting) > 0 {
		h.log.Info("waiting for a starting engine",
			slog.String("model", want.Model),
			slog.String("node", starting[0].Name))
		return h.cfg.WaitLoading(ctx, want, none.Results, h.progress())
	}
	return h.wakeFor(ctx, want, none.Results)
}

// progress adapts the fleet's progress reporting to the gateway's log, which
// is where a held request's waiting has to be visible: the caller is given
// nothing until the request completes.
func (h *Handler) progress() fleet.Waker {
	return func(format string, args ...any) {
		h.log.Info(strings.TrimSuffix(fmt.Sprintf(format, args...), "\n"))
	}
}

// wakeFor starts a node for a request nothing is serving, when the fleet file
// allows it, and holds the request until the engine answers. A concurrent
// request waking the same node loses its start to the daemon's 409 and takes
// the node the other one started — the same engine, the same wait.
func (h *Handler) wakeFor(ctx context.Context, want fleet.Want, results []fleet.NodeResult) (*fleet.Choice, error) {
	none := &fleet.ErrNoneServing{Results: results, Want: want, Path: h.cfg.Path}
	if h.cfgFor == nil {
		return nil, fmt.Errorf("%s\nthis gateway can wake no node: it has no way to resolve a node's Spinloop source", none)
	}
	if !h.cfg.Wakes() {
		return nil, h.refuseWake(want, none)
	}
	return h.cfg.Wake(ctx, want, h.matchingConfigFor(want.Model), results, h.progress())
}

// refuseWake is the wake-off answer: nothing is started, and the failure names
// the node whose source describes the model and the command that would start
// it — or, when no source describes it, that there is nothing to start.
func (h *Handler) refuseWake(want fleet.Want, none error) error {
	cfgFor := h.matchingConfigFor(want.Model)
	for _, entry := range h.cfg.Nodes {
		if _, err := cfgFor(entry); err == nil {
			return fmt.Errorf("%s\nwake is off in %s: %q's source describes %s; start it with `spinloop fleet start %s`",
				none, h.cfg.Path, entry.Name, want.Model, entry.Name)
		}
	}
	return fmt.Errorf("%s\nwake is off in %s, and no node's source describes %s", none, h.cfg.Path, want.Model)
}

// matchingConfigFor wraps the per-node source resolver with the one condition
// a wake has to meet: the source's config is the model the request asks for.
// A node whose source describes a different model is not a candidate — it
// would be started with the wrong engine — and its refusal says so.
func (h *Handler) matchingConfigFor(model string) fleet.ConfigFor {
	base := h.cfgFor
	return func(entry fleet.NodeConfig) (remote.DeployConfig, error) {
		dc, err := base(entry)
		if err != nil {
			return dc, err
		}
		if dc.ModelID != model && dc.ServedModelName != model {
			described := dc.ServedModelName
			if described == "" {
				described = dc.ModelID
			}
			return dc, fmt.Errorf("its source describes %s, not %s", described, model)
		}
		return dc, nil
	}
}

// proxy forwards the request to the chosen engine and the engine's reply back
// to the caller, unmodified: the body goes out as it came in, a streamed
// reply passes through as it is produced, and the reply the engine gives is
// the reply the caller gets — the gateway never retries another node.
func (h *Handler) proxy(w http.ResponseWriter, r *http.Request, body []byte, choice *fleet.Choice) {
	target, err := url.Parse(choice.BaseURL)
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Errorf("node %q's engine address is not a URL: %v", choice.Node.Name, err))
		return
	}
	p := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			// The caller asks the gateway for /v1/<rest>; the engine's base
			// URL already carries its own prefix, so /v1 comes off the
			// request and the rest goes on the base.
			pr.Out.URL.Path = joinPath(target.Path, strings.TrimPrefix(pr.In.URL.Path, "/v1"))
			pr.Out.URL.RawQuery = pr.In.URL.RawQuery
		},
		// -1 flushes after every write: a streamed reply reaches the caller
		// as the engine produces it, not when a buffer fills.
		FlushInterval: -1,
		// The default transport, with its connection pooling and no overall
		// request timeout: a completion may take as long as the model takes.
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			h.log.Error("route failed",
				slog.String("node", choice.Node.Name),
				slog.String("error", err.Error()))
			writeError(w, http.StatusBadGateway, fmt.Errorf(
				"the engine on %s failed to answer: %v", choice.Node.Name, err))
		},
	}

	out := r.Clone(r.Context())
	out.Body = io.NopCloser(bytes.NewReader(body))
	out.ContentLength = int64(len(body))
	if choice.APIKey != "" {
		// The caller's authoriser never travels past the gateway: the engine
		// is reached with the key its fleet entry names, resolved the way
		// every other fleet client resolves it.
		out.Header.Set("Authorization", "Bearer "+choice.APIKey)
	} else {
		out.Header.Del("Authorization")
	}
	p.ServeHTTP(w, out)
}

// joinPath joins a base path and a request path with at most one slash
// between them, either piece absent.
func joinPath(base, rest string) string {
	base = strings.TrimRight(base, "/")
	rest = "/" + strings.TrimLeft(rest, "/")
	return base + rest
}

// writeJSON sends a JSON reply.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeError sends a JSON error in the shape the OpenAI surface uses.
func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]any{"error": map[string]any{"message": err.Error(), "type": "gateway_error"}})
}
