package main

import (
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
)

// undeployRecorder answers the plane's control calls and remembers the last
// one that asked to undeploy, so a test can tell a refusal from a release —
// and prove that a refusal never reached the destroy path at all.
type undeployRecorder struct {
	mu     sync.Mutex
	calls  int
	query  url.Values
	body   string
	replay string
}

func (u *undeployRecorder) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.Method == http.MethodPost && q.Get("action") == "undeploy" {
			u.mu.Lock()
			u.calls++
			u.query = q
			u.mu.Unlock()
		}
		w.Header().Set("Content-Type", "application/json")
		if u.replay != "" {
			w.Write([]byte(u.replay))
			return
		}
		w.Write([]byte(`{"node":"dev","released":["instance","address","security-group","secret","schedules","deploy-config"]}`))
	})
}

func (u *undeployRecorder) undeployCalls() (int, url.Values) {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.calls, u.query
}

// The release is the plane's to perform and the report's to give: the command
// posts the undeploy action for the named cloud node and says what came back
// released.
func TestCloudUndeploy_ReleasesAndReports(t *testing.T) {
	isolateConfig(t)
	rec := &undeployRecorder{}
	stubPlane(t, rec.handler(), "dev")

	out := captureStdout(t, func() {
		if err := cmdCloudUndeploy([]string{"--node", "dev", "--yes"}); err != nil {
			t.Fatalf("undeploy: %v", err)
		}
	})
	calls, query := rec.undeployCalls()
	if calls != 1 || query.Get("env") != "dev" || query.Get("dryRun") != "" {
		t.Errorf("the plane was not asked to undeploy dev once: calls=%d query=%v", calls, query)
	}
	for _, want := range []string{"undeployed dev", "instance", "security-group", "deploy-config"} {
		if !strings.Contains(out, want) {
			t.Errorf("undeploy output missing %q:\n%s", want, out)
		}
	}
}

// A dry run asks the plane what it would release and releases nothing: the
// request carries the dry-run marker the plane honours by destroying nothing.
func TestCloudUndeploy_DryRunReleasesNothing(t *testing.T) {
	isolateConfig(t)
	rec := &undeployRecorder{}
	stubPlane(t, rec.handler(), "dev")

	out := captureStdout(t, func() {
		if err := cmdCloudUndeploy([]string{"--node", "dev", "--dry-run"}); err != nil {
			t.Fatalf("undeploy --dry-run: %v", err)
		}
	})
	calls, query := rec.undeployCalls()
	if calls != 1 || query.Get("dryRun") != "true" {
		t.Errorf("the dry run did not reach the plane marked as one: calls=%d query=%v", calls, query)
	}
	for _, want := range []string{"would release", "instance", "nothing was destroyed"} {
		if !strings.Contains(out, want) {
			t.Errorf("dry-run output missing %q:\n%s", want, out)
		}
	}
}

// Consent is required: with no --yes and nothing to answer the typed-name
// prompt (the test's stdin is no terminal), the command stops — naming the
// cloud node it did not destroy and the flag that would have confirmed it —
// and the plane is never asked to destroy.
func TestCloudUndeploy_ConsentIsRequired(t *testing.T) {
	isolateConfig(t)
	rec := &undeployRecorder{}
	stubPlane(t, rec.handler(), "dev")

	err := cmdCloudUndeploy([]string{"--node", "dev"})
	if err == nil {
		t.Fatal("an unconfirmed undeploy was accepted")
	}
	for _, want := range []string{"dev", "--yes"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q should name %q — the cloud node and the confirming flag", err, want)
		}
	}
	if calls, _ := rec.undeployCalls(); calls != 0 {
		t.Errorf("an aborted undeploy still reached the destroy path %d times", calls)
	}
}

// No name destroys nothing: there is no default cloud node, and the refusal
// names the flag that would have said which.
func TestCloudUndeploy_NoNameDestroysNothing(t *testing.T) {
	isolateConfig(t)
	rec := &undeployRecorder{}
	stubPlane(t, rec.handler(), "dev", "prod")

	err := cmdCloudUndeploy(nil)
	if err == nil {
		t.Fatal("an undeploy with no cloud node named was accepted")
	}
	for _, want := range []string{"--node", "dev", "prod"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q should name %q (the flag, or what could have been typed)", err, want)
		}
	}
	if calls, _ := rec.undeployCalls(); calls != 0 {
		t.Errorf("the refusal still reached the destroy path %d times", calls)
	}
}

// A name the plane does not hold is refused from its inventory — before the
// prompt and before any call — because existence belongs to the plane.
func TestCloudUndeploy_ACloudNodeThePlaneDoesNotHold(t *testing.T) {
	isolateConfig(t)
	rec := &undeployRecorder{}
	stubPlane(t, rec.handler(), "prod")

	err := cmdCloudUndeploy([]string{"--node", "ghost", "--yes"})
	if err == nil {
		t.Fatal("an undeploy of a name the plane does not hold was accepted")
	}
	for _, want := range []string{"ghost", "has no cloud node"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q should name %q", err, want)
		}
	}
	if calls, _ := rec.undeployCalls(); calls != 0 {
		t.Errorf("existence is settled by the inventory, yet a destroy call went out %d times", calls)
	}
}

// The plane's report of an empty release is said as such rather than leaving
// a trailing "released nothing" to read as a bug.
func TestCloudUndeploy_ReportsAnEmptyReleasePlainly(t *testing.T) {
	isolateConfig(t)
	rec := &undeployRecorder{replay: `{"node":"dev","released":[]}`}
	stubPlane(t, rec.handler(), "dev")

	out := captureStdout(t, func() {
		if err := cmdCloudUndeploy([]string{"--node", "dev", "--yes"}); err != nil {
			t.Fatalf("undeploy: %v", err)
		}
	})
	if !strings.Contains(out, "no allocated resources") {
		t.Errorf("an empty release should be said plainly:\n%s", out)
	}
}

// An inventory the plane itself reported as incomplete cannot certify that a
// name is absent — the absent name may be exactly what the plane could not
// read — and this verb destroys, so it must refuse rather than guess, without
// ever reaching the destroy request.
func TestCloudUndeploy_IncompleteInventoryIsNotAbsence(t *testing.T) {
	isolateConfig(t)
	rec := &undeployRecorder{}
	stubPlane(t, rec.handler(), "other")
	planeInventoryErrors = []string{"list failed: AccessDeniedException"}
	t.Cleanup(func() { planeInventoryErrors = nil })

	err := cmdCloudUndeploy([]string{"--node", "dev", "--yes"})
	if err == nil ||
		!strings.Contains(err.Error(), "cannot check whether cloud node") ||
		!strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("err = %v, want a refusal to conclude absence from a half-read inventory", err)
	}
	if calls, _ := rec.undeployCalls(); calls != 0 {
		t.Errorf("a refused undeploy must never reach the plane: %d calls", calls)
	}
}
