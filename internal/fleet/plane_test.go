package fleet

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/spinloop-ai/spinloop/internal/cloud"
)

// A fleet's cloud nodes live on a control plane resolved from the caller's
// credentials and region, and the plane's own inventory is what says whether a
// name is a cloud node at all. There is nothing on disk to seed for a test
// any more — that is the whole change — so these helpers stand a plane in the
// same way a machine without permission to read a stack states one outright:
// the endpoints by environment variable, and one test server answering them.

// stubPlane stands the control plane in for a test: it holds the cloud nodes
// named, and every node-named control read is answered by handler (nil
// for one that says the instance is running). It returns the URL the plane's
// endpoints resolve to, so a test can assert against the same server.
//
// The one request the plane is asked for itself — the stats read that names no
// node, which is the inventory — is answered here rather than by
// handler, so the two never need separate servers.
func stubPlane(t *testing.T, handler http.Handler, names ...string) string {
	t.Helper()
	nodes := make([]cloud.Node, 0, len(names))
	for _, n := range names {
		nodes = append(nodes, cloud.Node{Name: n, State: "running"})
	}
	return stubPlaneNodes(t, handler, nodes)
}

// stubPlaneNodes is stubPlane for a test that cares what the plane says about an
// node beyond that it holds the name — who deployed it, what it serves.
func stubPlaneNodes(t *testing.T, handler http.Handler, nodes []cloud.Node) string {
	t.Helper()
	if handler == nil {
		handler = controlAnswers(`{"state":"running","healthy":true}`, http.StatusOK)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("env") == "" {
			json.NewEncoder(w).Encode(map[string]any{"nodes": nodes})
			return
		}
		handler.ServeHTTP(w, r)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	stubPlaneEndpoints(t, srv.URL)
	return srv.URL
}

// stubPlaneCounting is stubPlane for a test that wants to know how many times
// the plane was asked for its inventory — the env-less stats read — which is
// how a test tells one read per fleet from one per node.
func stubPlaneCounting(t *testing.T, nodes []cloud.Node, reads *int) string {
	t.Helper()
	stubAWSCreds(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("env") == "" {
			*reads++
			json.NewEncoder(w).Encode(map[string]any{"nodes": nodes})
			return
		}
		w.Write([]byte(`{"state":"running","healthy":true}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	stubPlaneEndpoints(t, srv.URL)
	return srv.URL
}

// stubPlaneEndpoints states a plane's endpoints outright, which resolves it
// without a stack read, and remembers which nodes it holds.
func stubPlaneEndpoints(t *testing.T, url string, nodes ...cloud.Node) {
	t.Helper()
	stubAWSCreds(t)
	// Every endpoint the plane has, stated outright: a server standing in for a
	// plane answers all of them, and a node that reads an optional one (its
	// stats, its schedules) must find it where the plane says it is.
	for _, key := range []string{
		"SPINLOOP_CLOUD_START_URL", "SPINLOOP_CLOUD_STOP_URL", "SPINLOOP_CLOUD_DEPLOY_URL",
		"SPINLOOP_CLOUD_STATS_URL", "SPINLOOP_CLOUD_ENV_URL", "SPINLOOP_CLOUD_SEED_URL",
		"SPINLOOP_CLOUD_UPDATE_URL", "SPINLOOP_CLOUD_SCHEDULE_URL",
	} {
		t.Setenv(key, url)
	}
	t.Setenv("SPINLOOP_CLOUD_REGION", "us-east-1")
	if nodes != nil {
		servePlaneInventory(t, nodes)
	}
	// A complete set of endpoints resolves without a stack read, so the memo
	// cannot carry another test's plane into this one; clearing it both ways is
	// cheap and keeps the suite honest where a test resolves partially.
	cloud.ForgetPlanes()
	t.Cleanup(cloud.ForgetPlanes)
}

// stubPlaneAt points the resolved plane at a control server the test already
// built, and answers its inventory from a server of the stub's own holding the
// nodes named. It is for a test whose control replies are stateful — a
// wake fake that starts an instance — where one server answering everything
// would swallow the env-less inventory read.
func stubPlaneAt(t *testing.T, controlURL string, names ...string) {
	t.Helper()
	nodes := make([]cloud.Node, 0, len(names))
	for _, n := range names {
		nodes = append(nodes, cloud.Node{Name: n, State: "stopped"})
	}
	stubPlaneEndpoints(t, controlURL, nodes...)
}

// servePlaneInventory points the plane's stats endpoint at a small proxy that
// answers the env-less read — the inventory — itself and forwards every
// node-named read on to the plane's real stats endpoint. Both live at one
// URL on a real plane, separated only by whether the request names an
// node, so a stub that replaced the URL outright would swallow the node's
// own stats reads.
func servePlaneInventory(t *testing.T, nodes []cloud.Node) {
	t.Helper()
	target, err := url.Parse(os.Getenv("SPINLOOP_CLOUD_STATS_URL"))
	if err != nil {
		t.Fatalf("stats endpoint %q is not a URL: %v", target, err)
	}
	forward := httputil.NewSingleHostReverseProxy(target)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("env") == "" {
			json.NewEncoder(w).Encode(map[string]any{"nodes": nodes})
			return
		}
		forward.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("SPINLOOP_CLOUD_STATS_URL", srv.URL)
}

// stubPlaneEmpty is a plane that resolves and holds nothing at all: the shape a
// name that was never deployed meets.
func stubPlaneEmpty(t *testing.T) string {
	return stubPlaneNodes(t, nil, []cloud.Node{})
}

// stubPlaneRefused is a plane that resolves but cannot answer for itself — the
// case that must never be reported as an empty one. Its stats endpoint refuses
// the inventory the way a plane deployed before it existed does.
func stubPlaneRefused(t *testing.T, body string) string {
	t.Helper()
	srv := httptest.NewServer(controlAnswers(body, http.StatusBadRequest))
	t.Cleanup(srv.Close)
	stubPlaneEndpoints(t, srv.URL)
	return srv.URL
}

// stubPlaneUnresolvable leaves no plane to resolve: no endpoints stated, no
// region named, and an AWS chain that yields neither.
func stubPlaneUnresolvable(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"SPINLOOP_CLOUD_START_URL", "SPINLOOP_CLOUD_STOP_URL", "SPINLOOP_CLOUD_DEPLOY_URL",
		"SPINLOOP_CLOUD_STATS_URL", "SPINLOOP_CLOUD_ENV_URL", "SPINLOOP_CLOUD_SEED_URL",
		"SPINLOOP_CLOUD_UPDATE_URL", "SPINLOOP_CLOUD_SCHEDULE_URL",
		"SPINLOOP_CLOUD_REGION", "AWS_REGION", "AWS_DEFAULT_REGION",
		"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_SESSION_TOKEN", "AWS_PROFILE",
	} {
		t.Setenv(key, "")
	}
	// Neither an empty profile nor an empty credentials file can supply a
	// region, which is the only thing left for the chain to look in.
	t.Setenv("AWS_CONFIG_FILE", filepath.Join(t.TempDir(), "no-such-config"))
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(t.TempDir(), "no-such-credentials"))
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	cloud.ForgetPlanes()
	t.Cleanup(cloud.ForgetPlanes)
}

// controlAnswers is control-plane behaviour: one JSON body, one status, for
// every read it is given.
func controlAnswers(body string, code int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if code != http.StatusOK {
			w.WriteHeader(code)
		}
		w.Write([]byte(body))
	})
}

// testCtx is the context a test resolves a plane with: these commands are
// one-shot here, so there is nothing to cancel.
func testCtx() context.Context { return context.Background() }

// stubPlaneHalfRead is a plane that answers the inventory while admitting it
// could not read part of it — the answer from which absence proves nothing.
func stubPlaneHalfRead(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("env") == "" {
			io.WriteString(w, `{"nodes":[],"errors":["list failed: AccessDeniedException"]}`)
			return
		}
		io.WriteString(w, `{"state":"running","healthy":true}`)
	}))
	t.Cleanup(srv.Close)
	stubPlaneEndpoints(t, srv.URL)
	return srv.URL
}

// An abandoned read is not the plane's answer. A dashboard tick whose context
// is cancelled mid-flight must not leave every node of the next tick inheriting
// the cancellation for the whole TTL against a plane that was healthy.
func TestPlaneCacheDoesNotRecordACancelledRead(t *testing.T) {
	stubPlane(t, nil, "dev")
	cfg := &Config{Nodes: []NodeConfig{{Name: "dev", Kind: KindCloud}}, plane: &planeInventory{}}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := cfg.planeNode(ctx, "dev"); err == nil {
		t.Fatal("a cancelled read should fail")
	}

	// The next tick asks with a live context: the healthy plane must answer,
	// not the previous tick's cancellation.
	if _, found, err := cfg.planeNode(context.Background(), "dev"); err != nil || !found {
		t.Errorf("after a cancelled read, a live read = found %v, err %v; want the plane's answer", found, err)
	}
}
