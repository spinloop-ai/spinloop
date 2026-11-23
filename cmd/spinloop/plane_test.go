package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/spinloop-ai/spinloop/internal/cloud"
)

// What the CLI reads about the cloud comes from the control plane the caller's
// credentials and region resolve, and from nowhere on disk — so a test cannot
// seed a registry file any more than a user can write one. These helpers stand
// the plane in the way a machine with no permission to read a stack states one:
// the endpoints by environment variable, which resolve without AWS, and one test
// server answering them.
//
// stubPlane is the common shape: one server for the whole plane, splitting the
// one request the plane is asked for itself — the env-less stats read, which is
// the inventory — from the node-named control calls a node makes.

// stubPlane stands the CLI's control plane in: it holds the cloud nodes named,
// and every node-named control call is answered by handler (nil for one
// that says the instance is running). It returns the server, so a test can
// assert against it.
func stubPlane(t *testing.T, handler http.Handler, names ...string) *httptest.Server {
	t.Helper()
	nodes := make([]cloud.Node, 0, len(names))
	for _, n := range names {
		nodes = append(nodes, cloud.Node{Name: n, State: "running"})
	}
	return stubPlaneNodes(t, handler, nodes)
}

// stubPlaneNodes is stubPlane for a test that cares what the plane says about a
// cloud node beyond that it holds the name — who deployed it, what it serves,
// the address it answers on.
func stubPlaneNodes(t *testing.T, handler http.Handler, nodes []cloud.Node) *httptest.Server {
	t.Helper()
	if handler == nil {
		handler = planeControlAnswers(`{"state":"running","healthy":true}`, http.StatusOK)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("env") == "" {
			json.NewEncoder(w).Encode(map[string]any{"nodes": nodes})
			return
		}
		handler.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	stubPlaneEndpoints(t, srv.URL, nodes...)
	return srv
}

// stubPlaneAt points the resolved plane at a control server the test already
// built and answers its inventory from a server of the stub's own, holding the
// cloud nodes named. It is for a test whose control replies are stateful — the
// deploy fake that reports a fresh instance — where one server answering
// everything would swallow the env-less inventory read.
func stubPlaneAt(t *testing.T, controlURL string, names ...string) {
	t.Helper()
	nodes := make([]cloud.Node, 0, len(names))
	for _, n := range names {
		nodes = append(nodes, cloud.Node{Name: n, State: "stopped"})
	}
	stubPlaneEndpoints(t, controlURL, nodes...)
}

// stubPlaneEndpoints states a plane's endpoints outright — which resolves it
// without a stack read — and, when given an inventory, serves it from a server of
// the stub's own. With none, the plane's stats endpoint stays where the control
// endpoints are: a command that never lists the plane need not care what it
// answers.
func stubPlaneEndpoints(t *testing.T, controlURL string, nodes ...cloud.Node) {
	t.Helper()
	stubAWSEnv(t)
	// Every endpoint the plane has, stated outright: a test server standing in
	// for a plane answers all of them, and a command that reads an optional one
	// (schedules, seeds, the env Lambda) must find it the way it would on a
	// plane that has them.
	for _, key := range []string{
		"SPINLOOP_CLOUD_START_URL", "SPINLOOP_CLOUD_STOP_URL", "SPINLOOP_CLOUD_DEPLOY_URL",
		"SPINLOOP_CLOUD_STATS_URL", "SPINLOOP_CLOUD_ENV_URL", "SPINLOOP_CLOUD_SEED_URL",
		"SPINLOOP_CLOUD_UPDATE_URL", "SPINLOOP_CLOUD_SCHEDULE_URL",
	} {
		t.Setenv(key, controlURL)
	}
	t.Setenv("SPINLOOP_CLOUD_REGION", "us-east-1")
	if nodes != nil {
		servePlaneInventory(t, nodes)
	} else {
		planeInventory = nil
	}
	// The resolution is remembered per process, so a test that resolved
	// partially must leave nothing for the next one to inherit.
	cloud.ForgetPlanes()
	t.Cleanup(cloud.ForgetPlanes)
}

// servePlaneInventory points the plane's stats endpoint at a small proxy that
// answers the env-less read — the inventory — itself and forwards every
// node-named read on to the plane's real stats endpoint.
//
// The proxy is what lets a test whose control server also serves stats keep
// both: the inventory and a version string live at the same URL on a real
// plane, separated only by whether the request names a node, and a
// stub that simply replaced the URL would swallow one of them.
func servePlaneInventory(t *testing.T, nodes []cloud.Node) {
	t.Helper()
	upstream := os.Getenv("SPINLOOP_CLOUD_STATS_URL")
	target, err := url.Parse(upstream)
	if err != nil {
		t.Fatalf("stats endpoint %q is not a URL: %v", upstream, err)
	}
	forward := httputil.NewSingleHostReverseProxy(target)
	inventory := func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"nodes": nodes})
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("env") == "" {
			inventory(w)
			return
		}
		forward.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("SPINLOOP_CLOUD_STATS_URL", srv.URL)
	planeInventory = nodes
}

// planeInventory remembers what the running test's plane holds, so a fixture
// that moves the stats endpoint afterwards re-installs the inventory proxy over
// the new address instead of losing the inventory with it.
var planeInventory []cloud.Node

// planeStatsAt points the plane's stats endpoint at a test server — which
// answers both the node's own stats read and, through the proxy, the env-less
// inventory.
func planeStatsAt(t *testing.T, statsURL string) {
	t.Helper()
	t.Setenv("SPINLOOP_CLOUD_STATS_URL", statsURL)
	if planeInventory != nil {
		servePlaneInventory(t, planeInventory)
	}
}

// stubPlaneEmpty is a plane that resolves and holds nothing: the shape a name
// that was never deployed meets, and an honest empty `cloud ls`.
func stubPlaneEmpty(t *testing.T) *httptest.Server {
	return stubPlaneNodes(t, nil, []cloud.Node{})
}

// stubPlaneRefused is a plane that resolves but cannot answer for itself — the
// case that must never be reported as an empty one. Its stats endpoint refuses
// the inventory the way a plane deployed before the inventory existed does.
func stubPlaneRefused(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(planeControlAnswers(body, http.StatusBadRequest))
	t.Cleanup(srv.Close)
	stubPlaneEndpoints(t, srv.URL)
	return srv
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
	t.Setenv("AWS_CONFIG_FILE", filepath.Join(t.TempDir(), "no-such-config"))
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(t.TempDir(), "no-such-credentials"))
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	cloud.ForgetPlanes()
	t.Cleanup(cloud.ForgetPlanes)
}

// planeControlAnswers is control-plane behaviour: one JSON body, one status, for
// every call it is given.
func planeControlAnswers(body string, code int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if code != http.StatusOK {
			w.WriteHeader(code)
		}
		w.Write([]byte(body))
	})
}
