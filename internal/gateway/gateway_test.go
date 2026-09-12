package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spinloop-ai/spinloop/internal/daemon"
	"github.com/spinloop-ai/spinloop/internal/fleet"
	"github.com/spinloop-ai/spinloop/internal/remote"
)

// fakeNode is one machine: its daemon's control API and, once running, its
// engine's real HTTP endpoint on the port the daemon reports — so the
// gateway's proxy path is exercised end to end against a listener, not a
// stub. The engine's port is reserved at construction and occupied only when
// the engine is meant to be up, which is what readiness means here.
type fakeNode struct {
	mu sync.Mutex
	// state and what the daemon reports serving.
	state      string
	model      string
	servedName string
	// ready is what the daemon reports for `ready`: daemon.ReadyYes,
	// daemon.ReadyNo, or empty for a daemon whose reading has not landed —
	// an older build, or a runner with no known health-check convention.
	ready string
	// engineAuth, when set, is what the engine requires as its key.
	engineAuth string
	// startErr and startStatus are a refused start's reply.
	startErr    string
	startStatus int
	// engineDelay is how long after a start the engine listens.
	engineDelay time.Duration
	// noEngine keeps the engine down even after an accepted start:
	// readiness can only come from the daemon's own reading.
	noEngine bool
	// loopbackOnly is what the daemon reports for its engine's bind.
	loopbackOnly bool
	// started counts accepted starts; pushed is the last config and key.
	started   int
	pushed    *remote.DeployConfig
	pushedKey string
	// engineGotAuth is the last authorisation the engine itself saw.
	engineGotAuth string
	// statusHits counts status calls, so a burst's fan-out is countable.
	statusHits int

	daemonSrv  *httptest.Server
	engine     *http.Server
	engineLn   net.Listener
	enginePort int
}

func newFakeNode(t *testing.T, state, model string) *fakeNode {
	t.Helper()
	f := &fakeNode{state: state, model: model}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f.enginePort = ln.Addr().(*net.TCPAddr).Port
	ln.Close() // free it until the engine is meant to be up
	f.engine = &http.Server{Handler: f.engineHandler()}
	// A node already running has its engine up: its port answers.
	if state == string(daemon.StateRunning) {
		f.upAsEngine()
	}
	f.daemonSrv = httptest.NewServer(f.daemonMux())
	t.Cleanup(func() {
		f.daemonSrv.Close()
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.engineLn != nil {
			f.engineLn.Close()
		}
	})
	return f
}

// engineHandler is the engine itself: it checks its key, streams when asked,
// and fails when told to.
func (f *fakeNode) engineHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.engineGotAuth = r.Header.Get("Authorization")
		auth := f.engineAuth
		f.mu.Unlock()
		if auth != "" && f.engineGotAuth != "Bearer "+auth {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"error":"bad key"}`)
			return
		}
		body, _ := io.ReadAll(r.Body)
		switch {
		case strings.Contains(string(body), `"stream":true`):
			w.Header().Set("Content-Type", "text/event-stream")
			fl := w.(http.Flusher)
			for _, chunk := range []string{"data: one\n\n", "data: two\n\n", "data: [DONE]\n\n"} {
				io.WriteString(w, chunk)
				fl.Flush()
			}
		case strings.Contains(string(body), `"fail":true`):
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, `{"error":"the engine is on fire"}`)
		default:
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"id":"cmpl-1","object":"chat.completion","choices":[{"message":{"role":"assistant","content":"hello"}}]}`)
		}
	})
}

func (f *fakeNode) daemonMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/status", func(w http.ResponseWriter, _ *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.statusHits++
		resp := daemon.StatusResponse{State: f.state, Model: f.model, ServedName: f.servedName}
		if f.state == string(daemon.StateRunning) {
			resp.Engine = &daemon.EngineEndpoint{
				Port:         f.enginePort,
				LoopbackOnly: f.loopbackOnly,
				RequiresKey:  f.engineAuth != "",
			}
			resp.Ready = f.ready
		}
		json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("/v1/start", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		if f.started > 0 && f.state == string(daemon.StateRunning) && f.startErr == "" {
			// The daemon's own conflict: another start while one runs.
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(daemon.Error{Error: "an engine is already running"})
			f.mu.Unlock()
			return
		}
		if f.startErr != "" {
			status := f.startStatus
			if status == 0 {
				status = http.StatusBadRequest
			}
			w.WriteHeader(status)
			json.NewEncoder(w).Encode(daemon.Error{Error: f.startErr})
			f.mu.Unlock()
			return
		}
		var req daemon.StartRequest
		json.NewDecoder(r.Body).Decode(&req)
		dc := req.DeployConfig
		f.pushed = &dc
		f.pushedKey = req.EngineAPIKey
		f.started++
		f.state = string(daemon.StateRunning)
		if dc.ModelID != "" {
			f.model = dc.ModelID
		}
		f.servedName = dc.ServedModelName
		delay := f.engineDelay
		noEngine := f.noEngine
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(daemon.StatusResponse{State: f.state, Model: f.model})
		f.mu.Unlock()
		if !noEngine {
			go func() {
				time.Sleep(delay)
				f.upAsEngine()
			}()
		}
	})
	return mux
}

// upAsEngine occupies the reserved port, which readiness probes and the proxy
// both dial.
func (f *fakeNode) upAsEngine() {
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(f.enginePort)))
	if err != nil {
		return
	}
	f.mu.Lock()
	f.engineLn = ln
	f.mu.Unlock()
	go f.engine.Serve(ln)
}

// nodeConfig is the fleet-file entry pointing at this fake.
func (f *fakeNode) nodeConfig(name string) fleet.NodeConfig {
	host, port, _ := net.SplitHostPort(strings.TrimPrefix(f.daemonSrv.URL, "http://"))
	p, _ := strconv.Atoi(port)
	return fleet.NodeConfig{Name: name, Host: host, Port: p, Kind: fleet.KindDaemon}
}

// fleetOf builds a Config over the fakes, in the order given.
func fleetOf(t *testing.T, names []string, nodes ...*fakeNode) *fleet.Config {
	t.Helper()
	cfg := &fleet.Config{Path: "fleet.yaml", Dir: t.TempDir()}
	for i, n := range nodes {
		cfg.Nodes = append(cfg.Nodes, n.nodeConfig(names[i]))
	}
	return cfg
}

// cfgForOf is a per-node source resolver from a table: what each node's own
// Spinloop source would resolve to, or its refusal.
func cfgForOf(t *testing.T, table map[string]remote.DeployConfig, refused map[string]string) fleet.ConfigFor {
	return func(entry fleet.NodeConfig) (remote.DeployConfig, error) {
		if msg, ok := refused[entry.Name]; ok {
			return remote.DeployConfig{}, fmt.Errorf("%s", msg)
		}
		dc, ok := table[entry.Name]
		if !ok {
			return remote.DeployConfig{}, fmt.Errorf("node %q names no Spinloop source: no `file` field, no alias, no subdirectory", entry.Name)
		}
		return dc, nil
	}
}

// post sends one completion request to a handler and returns the reply.
func post(t *testing.T, h http.Handler, token string, body string) (*http.Response, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "http://gw/v1/chat/completions", strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	data, _ := io.ReadAll(rec.Result().Body)
	return rec.Result(), string(data)
}

// --- the surface ----------------------------------------------------------

func TestListenRefusesTokenlessNonLoopback(t *testing.T) {
	_, err := Listen("0.0.0.0:0", "")
	if err == nil {
		t.Fatal("a tokenless non-loopback listen should be refused")
	}
	for _, want := range []string{"--api-token-file", "SPINLOOP_API_TOKEN", "--api-token"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should name %q, got: %v", want, err)
		}
	}
	ln, err := Listen("127.0.0.1:0", "")
	if err != nil {
		t.Fatalf("a tokenless loopback listen is allowed: %v", err)
	}
	ln.Close()
}

func TestCallerAuthentication(t *testing.T) {
	cfg := fleetOf(t, []string{"box"}, newFakeNode(t, string(daemon.StateIdle), ""))
	h := New(cfg, "secret", Options{})

	// Missing and wrong tokens are 401, and no node is contacted.
	for _, tok := range []string{"", "wrong"} {
		req := httptest.NewRequest(http.MethodGet, "http://gw/health", nil)
		if tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("token %q: HTTP %d, want 401", tok, rec.Code)
		}
	}

	// The right one is through.
	req := httptest.NewRequest(http.MethodGet, "http://gw/health", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("right token: HTTP %d, want 200", rec.Code)
	}
}

func TestTokenlessLoopbackServesWithoutAuth(t *testing.T) {
	cfg := fleetOf(t, []string{"box"}, newFakeNode(t, string(daemon.StateIdle), ""))
	h := New(cfg, "", Options{})
	req := httptest.NewRequest(http.MethodGet, "http://gw/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("HTTP %d, want 200 with no token", rec.Code)
	}
}

func TestHealthTouchesNoNode(t *testing.T) {
	// A node on a dead port: unreachable, but health must not care.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	cfg := &fleet.Config{Path: "fleet.yaml", Dir: t.TempDir(), Nodes: []fleet.NodeConfig{
		{Name: "down", Host: "127.0.0.1", Port: port, Kind: fleet.KindDaemon},
	}}
	h := New(cfg, "", Options{})
	req := httptest.NewRequest(http.MethodGet, "http://gw/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("health with every node down: HTTP %d, want 200", rec.Code)
	}
}

func TestUnknownPathNamesTheSurface(t *testing.T) {
	cfg := fleetOf(t, []string{"box"}, newFakeNode(t, string(daemon.StateIdle), ""))
	h := New(cfg, "", Options{})
	req := httptest.NewRequest(http.MethodGet, "http://gw/v1/embeddings", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("HTTP %d, want 404", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"/v1/models", "/v1/chat/completions", "/v1/completions", "/health"} {
		if !strings.Contains(body, want) {
			t.Errorf("404 should name %s, got: %s", want, body)
		}
	}
}

// --- models ----------------------------------------------------------------

// modelsList makes one models request and returns the ids it lists.
func modelsList(t *testing.T, h http.Handler) []string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "http://gw/v1/models", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("HTTP %d, want 200", rec.Code)
	}
	var list struct {
		Data []map[string]any `json:"data"`
	}
	json.NewDecoder(rec.Body).Decode(&list)
	var ids []string
	for _, m := range list.Data {
		ids = append(ids, m["id"].(string))
	}
	return ids
}

func TestModelsListsWhatIsRunning(t *testing.T) {
	aliased := newFakeNode(t, string(daemon.StateRunning), "org/model")
	aliased.servedName = "the-alias"
	plain := newFakeNode(t, string(daemon.StateRunning), "org/other")
	stopped := newFakeNode(t, string(daemon.StateStopped), "org/stale")
	cfg := fleetOf(t, []string{"aliased", "plain", "stopped"}, aliased, plain, stopped)
	h := New(cfg, "", Options{})

	ids := modelsList(t, h)
	if !slices.Contains(ids, "the-alias") || !slices.Contains(ids, "org/other") {
		t.Errorf("models should list the running names, got %v", ids)
	}
	if slices.Contains(ids, "org/stale") {
		t.Error("a stopped node with no resolved source contributes nothing")
	}
}

func TestModelsEmptyWhenNothingIsReachable(t *testing.T) {
	cfg := fleetOf(t, []string{"box"}, newFakeNode(t, string(daemon.StateIdle), ""))
	h := New(cfg, "", Options{})
	if ids := modelsList(t, h); len(ids) != 0 {
		t.Errorf("nothing reachable is an empty list, not %v", ids)
	}
}

// With waking allowed, a node that is not running contributes the model its
// own source describes — the served name when the source gives one — so a
// client sees the models it can ask for, not only the ones answering now.
func TestModelsListsWhatAWakeCanStart(t *testing.T) {
	live := newFakeNode(t, string(daemon.StateRunning), "org/live")
	cold := newFakeNode(t, string(daemon.StateStopped), "")
	cfg := fleetOf(t, []string{"live", "cold"}, live, cold)
	h := New(cfg, "", Options{ConfigFor: cfgForOf(t,
		map[string]remote.DeployConfig{
			"live": {Runner: "llamacpp", ModelID: "org/live"},
			"cold": {Runner: "llamacpp", ModelID: "org/cold", ServedModelName: "cold"},
		},
		nil)})

	ids := modelsList(t, h)
	if !slices.Contains(ids, "org/live") || !slices.Contains(ids, "cold") {
		t.Errorf("models should list what runs and what a wake can start, got %v", ids)
	}
	if slices.Contains(ids, "org/cold") {
		t.Error("the source's served name, not its model id, is what a request names")
	}
}

// A running engine is never displaced to make room, so a running node lists
// what it reports and nothing its source describes.
func TestRunningNodeListsOnlyWhatItReports(t *testing.T) {
	node := newFakeNode(t, string(daemon.StateRunning), "org/live")
	cfg := fleetOf(t, []string{"box"}, node)
	h := New(cfg, "", Options{ConfigFor: cfgForOf(t,
		map[string]remote.DeployConfig{"box": {Runner: "llamacpp", ModelID: "org/other"}},
		nil)})
	if ids := modelsList(t, h); len(ids) != 1 || ids[0] != "org/live" {
		t.Errorf("a running node lists what it reports, got %v", ids)
	}
}

// With the fleet's wake off, nothing is started, so nothing is listed beyond
// what answers now.
func TestModelsListsOnlyWhatRunsWhenWakeIsOff(t *testing.T) {
	cold := newFakeNode(t, string(daemon.StateStopped), "")
	cfg := fleetOf(t, []string{"cold"}, cold)
	cfg.WakePolicy = fleet.WakeOff
	h := New(cfg, "", Options{ConfigFor: cfgForOf(t,
		map[string]remote.DeployConfig{"cold": {Runner: "llamacpp", ModelID: "org/cold"}},
		nil)})
	if ids := modelsList(t, h); len(ids) != 0 {
		t.Errorf("with wake off a stopped node's model is not listed, got %v", ids)
	}
}

// The gateway cannot start a remote environment — a request never wakes one —
// so its source's model is not wakeable and not listed.
func TestModelsLeavesOutARemoteEnvironment(t *testing.T) {
	cfg := &fleet.Config{Path: "fleet.yaml", Dir: t.TempDir(), Nodes: []fleet.NodeConfig{
		{Name: "env", Kind: fleet.KindRemote},
	}}
	h := New(cfg, "", Options{ConfigFor: cfgForOf(t,
		map[string]remote.DeployConfig{"env": {Runner: "llamacpp", ModelID: "org/cold"}},
		nil)})
	if m := h.wakeableModels(); len(m) != 0 {
		t.Errorf("a remote environment's source is not a request's, got %v", m)
	}
}

// Two sources describing one model list it once.
func TestModelsListsASharedSourceModelOnce(t *testing.T) {
	a := newFakeNode(t, string(daemon.StateStopped), "")
	b := newFakeNode(t, string(daemon.StateStopped), "")
	cfg := fleetOf(t, []string{"a", "b"}, a, b)
	h := New(cfg, "", Options{ConfigFor: cfgForOf(t,
		map[string]remote.DeployConfig{
			"a": {Runner: "llamacpp", ModelID: "org/same"},
			"b": {Runner: "llamacpp", ModelID: "org/same"},
		},
		nil)})
	ids := modelsList(t, h)
	if len(ids) != 1 || ids[0] != "org/same" {
		t.Errorf("a model two sources describe is listed once, got %v", ids)
	}
}

// A burst of models requests resolves each node's source once; a request
// after the reading ages resolves it again.
func TestModelsResolvesEachSourceOnce(t *testing.T) {
	cold := newFakeNode(t, string(daemon.StateStopped), "")
	cfg := fleetOf(t, []string{"cold"}, cold)
	now := time.Now()
	calls := 0
	h := New(cfg, "", Options{
		Now: func() time.Time { return now },
		ConfigFor: func(entry fleet.NodeConfig) (remote.DeployConfig, error) {
			calls++
			return remote.DeployConfig{Runner: "llamacpp", ModelID: "org/cold"}, nil
		},
	})
	for range 3 {
		modelsList(t, h)
	}
	if calls != 1 {
		t.Fatalf("a burst of three resolved the source %d times, want once", calls)
	}
	now = now.Add(sourcesTTL + time.Second)
	modelsList(t, h)
	if calls != 2 {
		t.Fatalf("the aged reading should have resolved the source once more, got %d total", calls)
	}
}

// --- routing ----------------------------------------------------------------

func TestRequestGoesToTheNodeServingItsModel(t *testing.T) {
	t.Setenv("RIGHT_ENGINE_KEY", "engine-key")
	right := newFakeNode(t, string(daemon.StateRunning), "org/wanted")
	wrong := newFakeNode(t, string(daemon.StateRunning), "org/other")
	cfg := fleetOf(t, []string{"right", "wrong"}, right, wrong)
	cfg.Nodes[0].EngineTokenEnv = "RIGHT_ENGINE_KEY"
	right.engineAuth = "engine-key"
	h := New(cfg, "caller-token", Options{})

	resp, body := post(t, h, "caller-token", `{"model":"org/wanted","messages":[]}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("HTTP %d, body %s", resp.StatusCode, body)
	}
	if !strings.Contains(body, "hello") {
		t.Errorf("the engine's reply did not reach the caller: %s", body)
	}
	right.mu.Lock()
	defer right.mu.Unlock()
	// The engine got its own key, not the caller's token.
	if right.engineGotAuth != "Bearer engine-key" {
		t.Errorf("engine saw %q, want the fleet's key", right.engineGotAuth)
	}
	if strings.Contains(right.engineGotAuth, "caller-token") {
		t.Error("the caller's token travelled past the gateway")
	}
}

func TestRequestNamesNoModel(t *testing.T) {
	node := newFakeNode(t, string(daemon.StateRunning), "org/wanted")
	cfg := fleetOf(t, []string{"box"}, node)
	h := New(cfg, "", Options{})
	resp, body := post(t, h, "", `{"messages":[]}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("HTTP %d, want 400", resp.StatusCode)
	}
	if !strings.Contains(body, "no model") {
		t.Errorf("the refusal should say the request names no model: %s", body)
	}
	node.mu.Lock()
	defer node.mu.Unlock()
	if node.statusHits != 0 {
		t.Error("a refused request contacted a node")
	}
}

func TestStreamedReplyPassesThrough(t *testing.T) {
	node := newFakeNode(t, string(daemon.StateRunning), "org/wanted")
	cfg := fleetOf(t, []string{"box"}, node)
	h := New(cfg, "", Options{})
	resp, body := post(t, h, "", `{"model":"org/wanted","stream":true}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("HTTP %d, body %s", resp.StatusCode, body)
	}
	if resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Errorf("content type = %q, want the engine's stream type", resp.Header.Get("Content-Type"))
	}
	for _, want := range []string{"data: one", "data: two", "data: [DONE]"} {
		if !strings.Contains(body, want) {
			t.Errorf("stream lost %q: %s", want, body)
		}
	}
}

func TestEngineRefusalIsTheCallersError(t *testing.T) {
	t.Setenv("KEY", "k")
	failing := newFakeNode(t, string(daemon.StateRunning), "org/wanted")
	other := newFakeNode(t, string(daemon.StateRunning), "org/wanted")
	cfg := fleetOf(t, []string{"failing", "other"}, failing, other)
	h := New(cfg, "", Options{})

	resp, body := post(t, h, "", `{"model":"org/wanted","fail":true}`)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("the engine's 500 is the caller's: HTTP %d, body %s", resp.StatusCode, body)
	}
	if !strings.Contains(body, "on fire") {
		t.Errorf("the engine's own refusal should reach the caller: %s", body)
	}
	other.mu.Lock()
	defer other.mu.Unlock()
	// The other node was never tried: its engine saw no request at all.
	if other.engineGotAuth != "" {
		t.Error("a failed request was retried at another node")
	}
}

func TestEngineDownFailsNamingTheNode(t *testing.T) {
	node := newFakeNode(t, string(daemon.StateRunning), "org/wanted")
	cfg := fleetOf(t, []string{"box"}, node)
	h := New(cfg, "", Options{})
	node.mu.Lock()
	node.engineLn.Close() // the engine dies while the daemon still reports it
	node.mu.Unlock()

	resp, body := post(t, h, "", `{"model":"org/wanted"}`)
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("HTTP %d, want 502: %s", resp.StatusCode, body)
	}
	if !strings.Contains(body, "box") {
		t.Errorf("the failure should name the node: %s", body)
	}
}

// A node reached by a non-loopback name with a loopback-bound engine, without
// an override, is not a candidate; the mark carries the daemon's explanation,
// which names the bind and the fix.
func TestReachableMarksLoopbackBoundEngines(t *testing.T) {
	cfg := &fleet.Config{Path: "fleet.yaml", Dir: t.TempDir(), Nodes: []fleet.NodeConfig{
		{Name: "bound", Host: "remote-box", Kind: fleet.KindDaemon, Port: 14242},
		{Name: "overridden", Host: "remote-box", Kind: fleet.KindDaemon, Port: 14243,
			Engine: &fleet.EngineOverride{Host: "proxy.local", Port: 9000, Path: "/v1"}},
	}}
	h := New(cfg, "", Options{})
	results := []fleet.NodeResult{
		{Name: "bound", Outcome: fleet.OutcomeOK, Status: daemon.StatusResponse{
			State: string(daemon.StateRunning), Model: "m",
			Engine: &daemon.EngineEndpoint{Port: 8080, LoopbackOnly: true},
		}},
		{Name: "overridden", Outcome: fleet.OutcomeOK, Status: daemon.StatusResponse{
			State: string(daemon.StateRunning), Model: "m",
			Engine: &daemon.EngineEndpoint{Port: 8080, LoopbackOnly: true},
		}},
	}
	marked := h.reachable(results)
	if marked[0].OK() {
		t.Fatal("a loopback-bound engine without an override is not a candidate")
	}
	if !strings.Contains(marked[0].Err.Error(), "loopback") {
		t.Errorf("the mark should carry the explanation naming the bind: %v", marked[0].Err)
	}
	if !marked[1].OK() {
		t.Errorf("an override takes responsibility for reachability: %v", marked[1].Err)
	}
}

// With nothing else matching, the failure the gateway gives names the bind
// and the fix rather than a bare "no node serving".
func TestLoopbackBoundEngineFailsNamingTheFix(t *testing.T) {
	cfg := &fleet.Config{Path: "fleet.yaml", Dir: t.TempDir(), Nodes: []fleet.NodeConfig{
		{Name: "bound", Host: "remote-box", Kind: fleet.KindDaemon, Port: 14242},
	}}
	h := New(cfg, "", Options{})
	results := []fleet.NodeResult{
		{Name: "bound", Outcome: fleet.OutcomeOK, Status: daemon.StatusResponse{
			State: string(daemon.StateRunning), Model: "org/wanted",
			Engine: &daemon.EngineEndpoint{Port: 8080, LoopbackOnly: true},
		}},
	}
	_, err := cfg.Choose(h.reachable(results), fleet.Want{Model: "org/wanted"})
	if err == nil {
		t.Fatal("nothing reachable serves the model, so the request must fail")
	}
	for _, want := range []string{"bound", "loopback"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("failure should name %q: %v", want, err)
		}
	}
}

// A remote environment's engine address arrives as its status's reported host —
// the control plane's published endpoint — so it is a candidate, not marked
// unreachable the way a loopback-bound engine is. Without this the gateway
// could list a remote node's model and then refuse to route a request to it.
func TestReachableRoutesToARemoteNode(t *testing.T) {
	t.Setenv("TEST_ENGINE_KEY", "secret")
	cfg := &fleet.Config{Path: "fleet.yaml", Dir: t.TempDir(), APIKeyEnv: "TEST_ENGINE_KEY", Nodes: []fleet.NodeConfig{
		{Name: "env", Kind: fleet.KindRemote},
	}}
	h := New(cfg, "", Options{})
	results := []fleet.NodeResult{
		{Name: "env", Outcome: fleet.OutcomeOK, Status: daemon.StatusResponse{
			State:      string(daemon.StateRunning),
			Model:      "org/wanted",
			ServedName: "wanted",
			Engine:     &daemon.EngineEndpoint{Host: "1.2.3.4", Port: 8000, Path: "/v1"},
		}},
	}
	choice, err := cfg.Choose(h.reachable(results), fleet.Want{Model: "wanted"})
	if err != nil {
		t.Fatalf("a request for the remote node's model should route to it: %v", err)
	}
	if choice.Node.Name != "env" {
		t.Errorf("choice = %q, want env", choice.Node.Name)
	}
	if choice.BaseURL != "http://1.2.3.4:8000/v1" {
		t.Errorf("base URL = %q, want the control plane's published address", choice.BaseURL)
	}
	if choice.APIKey != "secret" {
		t.Errorf("API key = %q, want the resolved engine key", choice.APIKey)
	}
}

func TestBurstFansOutOnce(t *testing.T) {
	node := newFakeNode(t, string(daemon.StateRunning), "org/wanted")
	cfg := fleetOf(t, []string{"box"}, node)
	now := time.Now()
	h := New(cfg, "", Options{Now: func() time.Time { return now }})

	for i := range 3 {
		resp, _ := post(t, h, "", `{"model":"org/wanted"}`)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("request %d: HTTP %d", i, resp.StatusCode)
		}
	}
	node.mu.Lock()
	hits := node.statusHits
	node.mu.Unlock()
	if hits != 1 {
		t.Fatalf("a burst of three fanned out %d times, want once", hits)
	}

	// A request after the reading ages fans out again.
	now = now.Add(3 * time.Second)
	resp, _ := post(t, h, "", `{"model":"org/wanted"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("stale-reading request: HTTP %d", resp.StatusCode)
	}
	node.mu.Lock()
	hits = node.statusHits
	node.mu.Unlock()
	if hits != 2 {
		t.Fatalf("the aged reading should have fanned out once more, got %d total", hits)
	}
}

// --- waking -----------------------------------------------------------------

func TestColdRequestWakesANode(t *testing.T) {
	t.Setenv("BOX_ENGINE_KEY", "engine-key")
	node := newFakeNode(t, string(daemon.StateIdle), "")
	node.engineDelay = 100 * time.Millisecond
	cfg := fleetOf(t, []string{"box"}, node)
	cfg.Nodes[0].EngineTokenEnv = "BOX_ENGINE_KEY"
	h := New(cfg, "", Options{
		ConfigFor: cfgForOf(t,
			map[string]remote.DeployConfig{"box": {Runner: "llamacpp", ModelID: "org/wanted", ServedModelName: "org/wanted"}},
			nil),
	})

	start := time.Now()
	resp, body := post(t, h, "", `{"model":"org/wanted"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("HTTP %d, body %s", resp.StatusCode, body)
	}
	if time.Since(start) < 100*time.Millisecond {
		t.Error("answered before the engine was up")
	}
	node.mu.Lock()
	defer node.mu.Unlock()
	if node.started != 1 {
		t.Fatalf("node started %d times, want once", node.started)
	}
	// Started with its own source's config, gated with the fleet's key.
	if node.pushed == nil || node.pushed.ModelID != "org/wanted" {
		t.Errorf("the node's own config did not reach it: %+v", node.pushed)
	}
	if node.pushedKey != "engine-key" {
		t.Errorf("the engine was gated with %q, want the fleet's key", node.pushedKey)
	}
}

func TestConcurrentColdRequestsShareOneWake(t *testing.T) {
	node := newFakeNode(t, string(daemon.StateIdle), "")
	node.engineDelay = 150 * time.Millisecond
	cfg := fleetOf(t, []string{"box"}, node)
	h := New(cfg, "", Options{
		ConfigFor: cfgForOf(t,
			map[string]remote.DeployConfig{"box": {Runner: "llamacpp", ModelID: "org/wanted"}},
			nil),
	})

	var wg sync.WaitGroup
	codes := make([]int, 2)
	for i := range codes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			resp, _ := post(t, h, "", `{"model":"org/wanted"}`)
			codes[i] = resp.StatusCode
		}(i)
	}
	wg.Wait()
	for i, code := range codes {
		if code != http.StatusOK {
			t.Errorf("request %d: HTTP %d, want 200 from the shared wake", i, code)
		}
	}
	node.mu.Lock()
	defer node.mu.Unlock()
	if node.started != 1 {
		t.Fatalf("the node was started %d times, want once", node.started)
	}
}

func TestWakeTimeoutFailsTheRequestAndLeavesTheEngine(t *testing.T) {
	old := fleet.WakeTimeout
	fleet.WakeTimeout = 300 * time.Millisecond
	t.Cleanup(func() { fleet.WakeTimeout = old })

	node := newFakeNode(t, string(daemon.StateIdle), "")
	node.noEngine = true // the engine never answers
	cfg := fleetOf(t, []string{"box"}, node)
	h := New(cfg, "", Options{
		ConfigFor: cfgForOf(t,
			map[string]remote.DeployConfig{"box": {Runner: "llamacpp", ModelID: "org/wanted"}},
			nil),
	})

	resp, body := post(t, h, "", `{"model":"org/wanted"}`)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("HTTP %d, want 503: %s", resp.StatusCode, body)
	}
	if !strings.Contains(body, "box") {
		t.Errorf("the timeout should name the node: %s", body)
	}
	node.mu.Lock()
	defer node.mu.Unlock()
	if node.started != 1 || node.state != string(daemon.StateRunning) {
		t.Error("the started engine is left running on timeout, not stopped")
	}
}

func TestWakeOffRefusesNamingTheNodeAndCommand(t *testing.T) {
	node := newFakeNode(t, string(daemon.StateIdle), "")
	cfg := fleetOf(t, []string{"box"}, node)
	cfg.WakePolicy = fleet.WakeOff
	h := New(cfg, "", Options{
		ConfigFor: cfgForOf(t,
			map[string]remote.DeployConfig{"box": {Runner: "llamacpp", ModelID: "org/wanted"}},
			nil),
	})

	resp, body := post(t, h, "", `{"model":"org/wanted"}`)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("HTTP %d, want 503", resp.StatusCode)
	}
	for _, want := range []string{"box", "spinloop fleet start box"} {
		if !strings.Contains(body, want) {
			t.Errorf("refusal should name %q: %s", want, body)
		}
	}
	node.mu.Lock()
	defer node.mu.Unlock()
	if node.started != 0 {
		t.Error("wake off starts nothing")
	}
}

func TestWakeOffWithNoMatchingSource(t *testing.T) {
	node := newFakeNode(t, string(daemon.StateIdle), "")
	cfg := fleetOf(t, []string{"box"}, node)
	cfg.WakePolicy = fleet.WakeOff
	h := New(cfg, "", Options{
		ConfigFor: cfgForOf(t,
			map[string]remote.DeployConfig{"box": {Runner: "llamacpp", ModelID: "org/something-else"}},
			nil),
	})
	resp, body := post(t, h, "", `{"model":"org/wanted"}`)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("HTTP %d, want 503", resp.StatusCode)
	}
	if !strings.Contains(body, "no node's source describes") {
		t.Errorf("the refusal should say nothing describes the model: %s", body)
	}
}

func TestNothingCanServeNamesEveryRefusal(t *testing.T) {
	a := newFakeNode(t, string(daemon.StateIdle), "")
	b := newFakeNode(t, string(daemon.StateIdle), "")
	cfg := fleetOf(t, []string{"a", "b"}, a, b)
	h := New(cfg, "", Options{
		ConfigFor: cfgForOf(t,
			map[string]remote.DeployConfig{"a": {Runner: "llamacpp", ModelID: "org/other"}},
			map[string]string{"b": "node \"b\" names no Spinloop source: no `file` field, no alias, no subdirectory"},
		),
	})

	resp, body := post(t, h, "", `{"model":"org/wanted"}`)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("HTTP %d, want 503", resp.StatusCode)
	}
	for _, want := range []string{"a", "b", "org/other", "Spinloop source"} {
		if !strings.Contains(body, want) {
			t.Errorf("the failure should name %q: %s", want, body)
		}
	}
	a.mu.Lock()
	b.mu.Lock()
	defer func() { a.mu.Unlock(); b.mu.Unlock() }()
	if a.started != 0 || b.started != 0 {
		t.Error("nothing is started when nothing can serve")
	}
}

// --- logging ------------------------------------------------------------------

func TestRoutedRequestLeavesOneLogLine(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, nil))
	node := newFakeNode(t, string(daemon.StateRunning), "org/wanted")
	cfg := fleetOf(t, []string{"box"}, node)
	h := New(cfg, "", Options{Log: log})

	resp, _ := post(t, h, "", `{"model":"org/wanted","messages":[{"content":"a secret prompt"}]}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("HTTP %d", resp.StatusCode)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("one routed request leaves one log line, got %d: %q", len(lines), buf.String())
	}
	for _, want := range []string{"org/wanted", "box"} {
		if !strings.Contains(lines[0], want) {
			t.Errorf("the log line should name %q: %s", want, lines[0])
		}
	}
	if strings.Contains(buf.String(), "secret prompt") {
		t.Error("the log carries the request body")
	}
}

// --- a node that is running but has not answered yet -----------------------

// startingNode is a machine whose daemon reports running with its engine's
// port closed and readiness not-ready: llama.cpp fetching or loading weights.
// It is the state that made a request fail with a dial error before the
// gateway consulted readiness at all.
func startingNode(t *testing.T, model string) *fakeNode {
	t.Helper()
	f := newFakeNode(t, string(daemon.StateIdle), model)
	f.mu.Lock()
	f.state = string(daemon.StateRunning)
	f.ready = daemon.ReadyNo
	f.mu.Unlock()
	return f
}

// answersIn flips the node to ready and opens its engine port after d.
func (f *fakeNode) answersIn(d time.Duration) {
	go func() {
		time.Sleep(d)
		f.upAsEngine()
		f.mu.Lock()
		f.ready = daemon.ReadyYes
		f.mu.Unlock()
	}()
}

func TestRequestWaitsForAStartingEngine(t *testing.T) {
	node := startingNode(t, "org/wanted")
	node.answersIn(150 * time.Millisecond)
	cfg := fleetOf(t, []string{"box"}, node)
	h := New(cfg, "", Options{
		ConfigFor: cfgForOf(t,
			map[string]remote.DeployConfig{"box": {Runner: "llamacpp", ModelID: "org/wanted"}},
			nil),
	})

	start := time.Now()
	resp, body := post(t, h, "", `{"model":"org/wanted"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("HTTP %d, body %s", resp.StatusCode, body)
	}
	if time.Since(start) < 150*time.Millisecond {
		t.Error("answered before the engine was up")
	}
	node.mu.Lock()
	defer node.mu.Unlock()
	if node.started != 0 {
		t.Errorf("the node was started %d times; it was already running", node.started)
	}
}

// The node loading the model is the one to wait for, not a reason to start a
// second engine somewhere else — which on a fleet of cloud nodes costs money
// for a model that is already minutes from being served.
func TestAStartingEngineIsWaitedForRatherThanWakingAnother(t *testing.T) {
	starting := startingNode(t, "org/wanted")
	starting.answersIn(100 * time.Millisecond)
	spare := newFakeNode(t, string(daemon.StateIdle), "")
	cfg := fleetOf(t, []string{"loading", "spare"}, starting, spare)
	h := New(cfg, "", Options{
		ConfigFor: cfgForOf(t, map[string]remote.DeployConfig{
			"loading": {Runner: "llamacpp", ModelID: "org/wanted"},
			"spare":   {Runner: "llamacpp", ModelID: "org/wanted"},
		}, nil),
	})

	resp, body := post(t, h, "", `{"model":"org/wanted"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("HTTP %d, body %s", resp.StatusCode, body)
	}
	spare.mu.Lock()
	defer spare.mu.Unlock()
	if spare.started != 0 {
		t.Errorf("the spare node was started %d times, want none", spare.started)
	}
}

// An engine that never answers fails the request with a message naming the
// node, not the dial error the caller used to get on every attempt.
func TestStartingEngineThatNeverAnswersFailsNamingTheNode(t *testing.T) {
	old := fleet.WakeTimeout
	fleet.WakeTimeout = 200 * time.Millisecond
	t.Cleanup(func() { fleet.WakeTimeout = old })

	node := startingNode(t, "org/wanted")
	cfg := fleetOf(t, []string{"box"}, node)
	h := New(cfg, "", Options{
		ConfigFor: cfgForOf(t,
			map[string]remote.DeployConfig{"box": {Runner: "llamacpp", ModelID: "org/wanted"}},
			nil),
	})

	resp, body := post(t, h, "", `{"model":"org/wanted"}`)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("HTTP %d, body %s", resp.StatusCode, body)
	}
	for _, want := range []string{"box", "still be loading"} {
		if !strings.Contains(body, want) {
			t.Errorf("body should mention %q, got: %s", want, body)
		}
	}
	if strings.Contains(body, "connect: connection refused") {
		t.Errorf("the caller should not be given a dial error: %s", body)
	}
}
