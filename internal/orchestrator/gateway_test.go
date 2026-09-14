package orchestrator

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGatewayTopologist_ReadsTheTopologyWithTheToken(t *testing.T) {
	var gotAuth, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		if r.URL.Path != "/v1/fleet" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, `{"wake":true,"prefer":"idle","concurrency":{"total":2},"nodes":[{"name":"n","kind":"daemon","tags":{"gpu":"a100"},"state":"running","model":"org/m"}]}`)
	}))
	defer srv.Close()

	topo, err := NewGatewayTopologist(srv.URL, "the-token").Topology(context.Background())
	if err != nil {
		t.Fatalf("the read should succeed: %v", err)
	}
	if gotAuth != "Bearer the-token" {
		t.Errorf("the read should present the token as a bearer, got %q", gotAuth)
	}
	if gotPath != "/v1/fleet" {
		t.Errorf("the read should ask the topology endpoint, got %q", gotPath)
	}
	if !topo.Wake || topo.Prefer != "idle" || topo.Concurrency == nil || topo.Concurrency.Total == nil || *topo.Concurrency.Total != 2 {
		t.Errorf("the reply's settings should ride through, got %+v", topo)
	}
	if len(topo.Nodes) != 1 || topo.Nodes[0].Name != "n" || topo.Nodes[0].State != "running" || topo.Nodes[0].Tags["gpu"] != "a100" {
		t.Errorf("the reply's nodes should ride through, got %+v", topo.Nodes)
	}
}

func TestGatewayTopologist_AGatewayThatDoesNotAnswerNamesIt(t *testing.T) {
	// A port nothing listens on: the read fails, and the run's contract ends
	// on it, naming the gateway.
	_, err := NewGatewayTopologist("http://127.0.0.1:1", "the-token").Topology(context.Background())
	if err == nil {
		t.Fatal("a gateway that does not answer should fail the read")
	}
	if !strings.Contains(err.Error(), "http://127.0.0.1:1") {
		t.Errorf("the failure should name the gateway, got %v", err)
	}
}

func TestGatewayTopologist_AGatewayThatRefusesTheTokenNamesIt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"missing or invalid bearer token"}}`, http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, err := NewGatewayTopologist(srv.URL, "the-wrong-token").Topology(context.Background())
	if err == nil {
		t.Fatal("a gateway that refuses the token should fail the read")
	}
	if !strings.Contains(err.Error(), srv.URL) || !strings.Contains(err.Error(), "401") {
		t.Errorf("the failure should name the gateway and its answer, got %v", err)
	}
	if strings.Contains(err.Error(), "the-wrong-token") {
		t.Errorf("the failure must not echo the token, got %v", err)
	}
}

func TestGatewayTopologist_AReplyThatIsNotARecord(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "not a record")
	}))
	defer srv.Close()

	_, err := NewGatewayTopologist(srv.URL, "the-token").Topology(context.Background())
	if err == nil {
		t.Fatal("a reply that is not a record should fail the read")
	}
	if !strings.Contains(err.Error(), srv.URL) {
		t.Errorf("the failure should name the gateway, got %v", err)
	}
}
