//go:build !windows

package daemon

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"

	"github.com/spinloop-ai/spinloop/internal/metrics"
	"github.com/spinloop-ai/spinloop/internal/remote"
)

// fakeHealth is a stand-in engine health endpoint whose status the test
// controls, mirroring fakeEngine's shape for /metrics.
type fakeHealth struct {
	mu     sync.Mutex
	status int
}

func (f *fakeHealth) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/health" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	f.mu.Lock()
	status := f.status
	f.mu.Unlock()
	w.WriteHeader(status)
}

// startRunningDaemon builds a Daemon with its engine already running under
// the given runner name, without going through Push/ValidateConfig — the
// readiness checks care about the runner name and running state, not what
// deploy config produced them.
func startRunningDaemon(t *testing.T, runner string) *Daemon {
	t.Helper()
	dir := t.TempDir()
	engine := stubEngine(t, `trap 'exit 0' TERM
while true; do sleep 0.05; done`)
	d := &Daemon{Sup: NewSupervisor(filepath.Join(dir, "engine.log")), Dir: dir}
	if err := d.Sup.Start([]string{engine}); err != nil {
		t.Fatal(err)
	}
	waitForState(t, d.Sup, StateRunning)
	d.SetServed(runner, "model")
	return d
}

func TestCheckReadyOnceReady(t *testing.T) {
	health := &fakeHealth{status: http.StatusOK}
	srv := httptest.NewServer(health)
	defer srv.Close()

	d := startRunningDaemon(t, "llamacpp")
	d.SetScrape(metrics.ScrapeTarget{BaseURL: srv.URL, Engine: "llamacpp"})

	d.checkReadyOnce(context.Background())
	if got := d.readinessField(); got != "ready" {
		t.Fatalf("readinessField = %q, want ready", got)
	}
	if got := d.Status().Ready; got != "ready" {
		t.Errorf("Status().Ready = %q, want ready", got)
	}
	if got := d.Metrics(context.Background()).Ready; got != "ready" {
		t.Errorf("Metrics().Ready = %q, want ready", got)
	}
}

// TestCheckReadyOnceGatedEngineStillReady covers the 401 case directly, since
// CheckEngineReady's own tests (internal/metrics) already cover the HTTP
// semantics — this only confirms the daemon plumbs a 401 through as ready.
func TestCheckReadyOnceGatedEngineStillReady(t *testing.T) {
	health := &fakeHealth{status: http.StatusUnauthorized}
	srv := httptest.NewServer(health)
	defer srv.Close()

	d := startRunningDaemon(t, "llamacpp")
	d.SetScrape(metrics.ScrapeTarget{BaseURL: srv.URL, Engine: "llamacpp"})

	d.checkReadyOnce(context.Background())
	if got := d.readinessField(); got != "ready" {
		t.Fatalf("readinessField for a gated engine = %q, want ready", got)
	}
}

func TestCheckReadyOnceNotReady(t *testing.T) {
	health := &fakeHealth{status: http.StatusServiceUnavailable}
	srv := httptest.NewServer(health)
	defer srv.Close()

	d := startRunningDaemon(t, "vllm")
	d.SetScrape(metrics.ScrapeTarget{BaseURL: srv.URL, Engine: "vllm"})

	d.checkReadyOnce(context.Background())
	if got := d.readinessField(); got != "not-ready" {
		t.Fatalf("readinessField = %q, want not-ready", got)
	}
	if got := d.Status().Ready; got != "not-ready" {
		t.Errorf("Status().Ready = %q, want not-ready", got)
	}
	if got := d.Metrics(context.Background()).Ready; got != "not-ready" {
		t.Errorf("Metrics().Ready = %q, want not-ready", got)
	}
}

// TestCheckReadyOnceUnknownRunnerSkipped covers omlx, and any other runner
// with no established health-check convention: the check is never attempted,
// so the field stays absent rather than reporting a guess.
func TestCheckReadyOnceUnknownRunnerSkipped(t *testing.T) {
	health := &fakeHealth{status: http.StatusOK}
	srv := httptest.NewServer(health)
	defer srv.Close()

	d := startRunningDaemon(t, "omlx")
	d.SetScrape(metrics.ScrapeTarget{BaseURL: srv.URL, Engine: "omlx"})

	d.checkReadyOnce(context.Background())
	if got := d.readinessField(); got != "" {
		t.Fatalf("readinessField for an unchecked runner = %q, want empty", got)
	}
	if got := d.Status().Ready; got != "" {
		t.Errorf("Status().Ready for an unchecked runner = %q, want empty", got)
	}
}

// TestReadinessAbsentWhenNotRunning covers idle, stopped, and crashed alike:
// Status and Metrics only consult readiness inside their running branch, so
// a stale reading never leaks out once the engine is not running.
func TestReadinessAbsentWhenNotRunning(t *testing.T) {
	dir := t.TempDir()
	d := &Daemon{Sup: NewSupervisor(filepath.Join(dir, "engine.log")), Dir: dir}
	d.ready.record(true)

	if got := d.Status().Ready; got != "" {
		t.Errorf("idle Status().Ready = %q, want empty", got)
	}
	if got := d.Metrics(context.Background()).Ready; got != "" {
		t.Errorf("idle Metrics().Ready = %q, want empty", got)
	}
}

func TestStartEngineForgetsReadiness(t *testing.T) {
	health := &fakeHealth{status: http.StatusOK}
	srv := httptest.NewServer(health)
	defer srv.Close()

	d := testDaemon(t, `trap 'exit 0' TERM
while true; do sleep 0.05; done`)
	if err := d.Push(remote.DeployConfig{Runner: "llamacpp", ModelID: "m"}); err != nil {
		t.Fatal(err)
	}
	if err := d.StartEngine(); err != nil {
		t.Fatal(err)
	}
	waitForState(t, d.Sup, StateRunning)
	d.SetScrape(metrics.ScrapeTarget{BaseURL: srv.URL, Engine: "llamacpp"})
	d.checkReadyOnce(context.Background())
	if got := d.readinessField(); got != "ready" {
		t.Fatalf("readinessField before restart = %q, want ready", got)
	}

	if err := d.Sup.Stop(); err != nil {
		t.Fatal(err)
	}
	waitForState(t, d.Sup, StateStopped)
	if err := d.StartEngine(); err != nil {
		t.Fatal(err)
	}
	waitForState(t, d.Sup, StateRunning)
	if got := d.readinessField(); got != "" {
		t.Fatalf("readinessField right after restart, before a new check = %q, want empty", got)
	}
}
