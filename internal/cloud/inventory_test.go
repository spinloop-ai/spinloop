package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The plane answers the env-less stats read with its inventory, and the
// stop endpoint's undeploy action with what it released. These are the two
// questions the per-user registry used to answer from memory; the tests pin
// the wire shape in both directions.

func TestInventory_ReadsThePlanesRecord(t *testing.T) {
	stubAWSEnv(t)
	var calledWithEnv bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledWithEnv = r.URL.Query().Get("env") != ""
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"nodes":[{
			"node":"dev","deployedBy":"pete","deployedAt":"2026-11-01T09:00:00.000Z",
			"runner":"llamacpp","modelId":"org/qwen","servedName":"qwen",
			"baseUrl":"http://198.51.100.9:8000/v1","state":"running",
			"instanceType":"g6e.xlarge","instanceId":"i-abc123","partial":false
		}],"errors":["one instance could not be read"]}`)
	}))
	defer server.Close()

	cfg := Config{StatsURL: server.URL, Node: "ignored", Region: "eu-west-1"}
	inv, err := ReadInventory(context.Background(), cfg)
	if err != nil {
		t.Fatalf("inventory: %v", err)
	}
	if calledWithEnv {
		t.Error("the inventory must be asked without naming a node")
	}
	if len(inv.Nodes) != 1 {
		t.Fatalf("nodes = %v", inv.Nodes)
	}
	e := inv.Nodes[0]
	if e.Name != "dev" || e.DeployedBy != "pete" || e.Served() != "qwen" ||
		e.BaseURL != "http://198.51.100.9:8000/v1" || e.State != "running" {
		t.Errorf("node parsed wrong: %+v", e)
	}
	if !strings.Contains(e.Since(), "2026-11-01") {
		t.Errorf("Since() = %q, want the deploy date in it", e.Since())
	}
	if inv.Complete() || len(inv.Errors) != 1 {
		t.Errorf("the plane's own read failures must travel with the answer: %+v", inv.Errors)
	}
	if inv.Uncertain() == nil {
		t.Error("an answer carrying errors cannot settle what is deployed")
	}
	if _, found := inv.Lookup("dev"); !found {
		t.Error("Lookup should find the node the plane did read")
	}
}

// An answer that admits its own gaps must say so: absence from a short list
// is not the plane's "no such node", and callers deciding existence (or a
// deploy deciding a name is free) have to be able to tell the two apart.
func TestInventory_IncompleteAnswerIsUncertain(t *testing.T) {
	stubAWSEnv(t)
	server := httptest.NewServer(planeReply(`{"nodes":[],"errors":["list failed: AccessDeniedException"]}`))
	defer server.Close()

	inv, err := ReadInventory(context.Background(), Config{StatsURL: server.URL, Region: "eu-west-1"})
	if err != nil {
		t.Fatalf("inventory: %v", err)
	}
	if _, found := inv.Lookup("dev"); found {
		t.Fatal("the plane listed no dev")
	}
	if inv.Complete() {
		t.Fatal("the plane reported an error")
	}
	err = inv.Uncertain()
	if err == nil || !strings.Contains(err.Error(), "incomplete") || !strings.Contains(err.Error(), "AccessDeniedException") {
		t.Errorf("Uncertain = %v, want it to name the plane's own failure", err)
	}
}

// A plane deployed before the inventory existed answers the env-less read the
// way it always has. Reporting that beats reporting an empty plane — an
// confidently wrong answer.
func TestInventory_PlaneWithoutTheField(t *testing.T) {
	stubAWSEnv(t)
	server := httptest.NewServer(planeReply(`{"state":"stopped","version":"1.2.0"}`))
	defer server.Close()

	_, err := ReadInventory(context.Background(), Config{StatsURL: server.URL, Region: "eu-west-1"})
	if err == nil || !strings.Contains(err.Error(), "does not report its nodes") {
		t.Errorf("a pre-inventory plane should say so, got %v", err)
	}
	if err != nil && !strings.Contains(err.Error(), "spinloop cloud bootstrap") {
		t.Errorf("the failure should name the fix: %v", err)
	}
}

// An empty inventory is an answer, not a failure: a bootstrapped plane with
// nothing deployed lists nothing.
func TestInventory_EmptyPlaneIsEmptyAnswer(t *testing.T) {
	stubAWSEnv(t)
	server := httptest.NewServer(planeReply(`{"nodes":[]}`))
	defer server.Close()

	inv, err := ReadInventory(context.Background(), Config{StatsURL: server.URL, Region: "eu-west-1"})
	if err != nil || len(inv.Nodes) != 0 || !inv.Complete() {
		t.Errorf("inv = %+v, err = %v; want a complete empty answer", inv, err)
	}
}

// A plane with no stats endpoint to read cannot be listed, and the failure
// names the variable that supplies one.
func TestInventory_NoStatsEndpointNamesTheFix(t *testing.T) {
	_, err := ReadInventory(context.Background(), Config{Region: "eu-west-1"})
	if err == nil || !strings.Contains(err.Error(), "SPINLOOP_CLOUD_STATS_URL") {
		t.Errorf("err = %v, want it to name the stats variable", err)
	}
}

// An unreadable inventory is reported as unreadable, not as empty.
func TestInventory_UnreadableReply(t *testing.T) {
	stubAWSEnv(t)
	server := httptest.NewServer(planeReply("{not json"))
	defer server.Close()

	_, err := ReadInventory(context.Background(), Config{StatsURL: server.URL, Region: "eu-west-1"})
	if err == nil || !strings.Contains(err.Error(), "parsing") {
		t.Errorf("err = %v, want a parsing failure", err)
	}
}

func TestUndeploy_PostsTheActionAndReportsTheRelease(t *testing.T) {
	stubAWSEnv(t)
	var method, action, env, dryRun string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, action, env, dryRun = r.Method,
			r.URL.Query().Get("action"), r.URL.Query().Get("env"), r.URL.Query().Get("dryRun")
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"node":"dev","released":["instance","address","security-group","secret","schedules","deploy-config"]}`)
	}))
	defer server.Close()

	cfg := Config{StopURL: server.URL, Node: "dev", Region: "eu-west-1"}
	res, err := Undeploy(context.Background(), cfg, false)
	if err != nil {
		t.Fatalf("undeploy: %v", err)
	}
	if method != http.MethodPost || action != "undeploy" || env != "dev" {
		t.Errorf("request = %s action=%q env=%q, want POST undeploy for dev", method, action, env)
	}
	if dryRun != "" {
		t.Errorf("a real undeploy must not be marked as a dry run: %q", dryRun)
	}
	if res.Node != "dev" || len(res.Released) != 6 || res.DryRun {
		t.Errorf("result = %+v", res)
	}
}

// A dry run is the same request marked as one: the plane reports what would
// go and releases nothing.
func TestUndeploy_DryRunMarksTheRequest(t *testing.T) {
	stubAWSEnv(t)
	var dryRun string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dryRun = r.URL.Query().Get("dryRun")
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"node":"dev","released":["instance"]}`)
	}))
	defer server.Close()

	res, err := Undeploy(context.Background(), Config{StopURL: server.URL, Node: "dev", Region: "eu-west-1"}, true)
	if err != nil {
		t.Fatalf("undeploy --dry-run: %v", err)
	}
	if dryRun != "true" {
		t.Errorf("dryRun = %q, want true on the request", dryRun)
	}
	if !res.DryRun {
		t.Error("the result should say it was a dry run")
	}
}

// A plane that answers the undeploy for an unknown name with 404 resolves to
// NotDeployed — a fact about the name, not a failed call.
func TestUndeploy_UnknownNameIsNotDeployed(t *testing.T) {
	stubAWSEnv(t)
	server := httptest.NewServer(planeReplyCode(http.StatusNotFound, `{"error":"no node named ghost"}`))
	defer server.Close()

	_, err := Undeploy(context.Background(), Config{StopURL: server.URL, Node: "ghost", Region: "eu-west-1"}, false)
	var missing *NotDeployed
	if !errors.As(err, &missing) || missing.Node != "ghost" {
		t.Errorf("err = %v, want NotDeployed naming ghost", err)
	}
}

// A refused undeploy keeps the plane's reason and the status code; a missing
// stop endpoint names the variable that supplies one.
func TestUndeploy_FailuresNameTheCause(t *testing.T) {
	stubAWSEnv(t)
	server := httptest.NewServer(planeReplyCode(http.StatusInternalServerError, `{"error":"instance release failed"}`))
	defer server.Close()

	_, err := Undeploy(context.Background(), Config{StopURL: server.URL, Node: "dev", Region: "eu-west-1"}, false)
	if err == nil || !strings.Contains(err.Error(), "500") || !strings.Contains(err.Error(), "instance release failed") {
		t.Errorf("err = %v, want the refusal with the plane's reason", err)
	}

	if _, err := Undeploy(context.Background(), Config{Node: "dev", Region: "eu-west-1"}, false); err == nil ||
		!strings.Contains(err.Error(), "SPINLOOP_CLOUD_STOP_URL") {
		t.Errorf("err = %v, want it to name the stop variable", err)
	}
}

// The NotDeployed text is the repair, not just the absence: it says where the
// name was looked for and what would create it.
func TestNotDeployedWordsTheRepair(t *testing.T) {
	err := (&NotDeployed{Node: "dev", Region: "eu-west-1"}).Error()
	for _, want := range []string{"eu-west-1", "dev", "spinloop cloud deploy --node dev"} {
		if !strings.Contains(err, want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}
}

// planeReply answers every request with one JSON body at 200.
func planeReply(body string) http.Handler { return planeReplyCode(http.StatusOK, body) }

// planeReplyCode answers every request with one JSON body at one status.
func planeReplyCode(code int, body string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		io.WriteString(w, body)
	})
}

// Reply decoding keeps the fields the fleet reads from an inventory entry —
// the served name beside the model id, the partial marker beside the gaps.
func TestInventoryEntryFields(t *testing.T) {
	var e Node
	if err := json.Unmarshal([]byte(`{"node":"e","modelId":"m","partial":true}`), &e); err != nil {
		t.Fatal(err)
	}
	if e.Served() != "m" || !e.Partial {
		t.Errorf("entry = %+v", e)
	}
	if (Node{ModelID: "m", ServedName: "served"}).Served() != "served" {
		t.Error("the served name should win over the model id")
	}
	if (Node{DeployedAt: "not a time"}).Since() != "not a time" {
		t.Error("an unparsable deploy time should be shown as reported")
	}
}

// A plane deployed before undeploy existed answers action=undeploy the way it
// answers any unknown action: by terminating. Its echo names no node under the
// new key and lists nothing released — the verb must refuse that as "this
// plane cannot undeploy" rather than report a completed teardown (and, worst
// of all, a successful dry run over a destroy).
func TestUndeploy_OldPlaneTerminateEchoIsRefused(t *testing.T) {
	stubAWSEnv(t)
	server := httptest.NewServer(planeReply(`{"state":"terminating","environment":"dev"}`))
	defer server.Close()

	for _, dry := range []bool{false, true} {
		_, err := Undeploy(context.Background(), Config{StopURL: server.URL, Node: "dev", Region: "eu-west-1"}, dry)
		if err == nil || !strings.Contains(err.Error(), "does not understand undeploy") ||
			!strings.Contains(err.Error(), "spinloop cloud bootstrap") {
			t.Errorf("dryRun=%v: err = %v, want the old-plane refusal naming the repair", dry, err)
		}
	}
}
