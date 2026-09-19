package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// --- 1: a single read -------------------------------------------------------

func TestWorkLogs_CallsTheLogPathAndPrintsTheOutput(t *testing.T) {
	base, got := workAPIStub(t, http.StatusOK, map[string]any{"id": "a", "log": "hello\n"})
	out, err := runWork(t, "logs", "a", "--url", base, "--api-token", "tok")
	if err != nil {
		t.Fatalf("work logs: %v (out %s)", err, out)
	}
	if out != "hello\n" {
		t.Errorf("out = %q, want %q", out, "hello\n")
	}
	if got.method != http.MethodGet || got.path != "/v1/items/a/log" {
		t.Errorf("logs calls the API's log path for the id: %s %s", got.method, got.path)
	}
	if got.bearer != "Bearer tok" {
		t.Errorf("bearer = %q, want the token", got.bearer)
	}
}

func TestWorkLogs_NoOutputYetPrintsNothingNotAFault(t *testing.T) {
	base, _ := workAPIStub(t, http.StatusOK, map[string]any{"id": "a", "log": nil})
	out, err := runWork(t, "logs", "a", "--url", base)
	if err != nil {
		t.Fatalf("an item with no output yet should not fail: %v", err)
	}
	if out != "" {
		t.Errorf("out = %q, want empty", out)
	}
}

func TestWorkLogs_TheRefusalReadsTheWayTheAPIStatesIt(t *testing.T) {
	base, _ := workAPIStub(t, http.StatusNotFound,
		workAPIErrorReply(`the items file carries no item with id "ghost"`))
	_, err := runWork(t, "logs", "ghost", "--url", base)
	if err == nil || !strings.Contains(err.Error(), `no item with id "ghost"`) {
		t.Errorf("an id the run does not carry is refused, naming it: %v", err)
	}
}

// --- 2: following ------------------------------------------------------------

func TestWorkLogs_WithoutFollowMakesOneRequest(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		mu.Unlock()
		json.NewEncoder(w).Encode(map[string]any{"id": "a", "log": "hi\n"})
	}))
	t.Cleanup(srv.Close)

	out, err := runWork(t, "logs", "a", "--url", srv.URL)
	if err != nil {
		t.Fatalf("work logs: %v", err)
	}
	if out != "hi\n" {
		t.Errorf("out = %q, want %q", out, "hi\n")
	}
	mu.Lock()
	defer mu.Unlock()
	if calls != 1 {
		t.Errorf("without --follow, one request should be made, got %d", calls)
	}
}

// workLogsFollowStub answers /v1/items/<id>/log and /v1/items across a
// sequence of ticks it advances on every /v1/items poll (the loop's own
// last call each tick, after the log poll) — a state's own scenario
// controls when the item ends.
type workLogsFollowStub struct {
	mu    sync.Mutex
	tick  int
	logs  []string // one per tick; the last entry repeats past the end
	state []string // one per tick; "" (past the end of the slice) means removed
}

func (s *workLogsFollowStub) server(t *testing.T) string {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/items/a/log", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		i := s.tick
		if i >= len(s.logs) {
			i = len(s.logs) - 1
		}
		log := s.logs[i]
		s.mu.Unlock()
		json.NewEncoder(w).Encode(map[string]any{"id": "a", "log": log})
	})
	mux.HandleFunc("GET /v1/items", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		i := s.tick
		var state string
		found := i < len(s.state) && s.state[i] != ""
		if found {
			state = s.state[i]
		}
		s.tick++
		s.mu.Unlock()
		var data []map[string]any
		if found {
			data = []map[string]any{{"id": "a", "state": state}}
		}
		json.NewEncoder(w).Encode(map[string]any{"data": data})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestFollowWorkLogsLoop_PrintsOnlyNewOutputEachTick(t *testing.T) {
	prev := workLogsInterval
	workLogsInterval = time.Millisecond
	t.Cleanup(func() { workLogsInterval = prev })

	stub := &workLogsFollowStub{
		logs:  []string{"first\n", "first\nsecond\n", "first\nsecond\n"},
		state: []string{"running", "running", "done"},
	}
	base := stub.server(t)

	var buf bytes.Buffer
	if err := followWorkLogsLoop(context.Background(), base, "", "a", &buf); err != nil {
		t.Fatalf("followWorkLogsLoop: %v", err)
	}
	if got := buf.String(); got != "first\nsecond\n" {
		t.Errorf("out = %q, want %q (no duplicated suffix)", got, "first\nsecond\n")
	}
}

func TestFollowWorkLogsLoop_KeepsPollingThroughTheBacklog(t *testing.T) {
	prev := workLogsInterval
	workLogsInterval = time.Millisecond
	t.Cleanup(func() { workLogsInterval = prev })

	stub := &workLogsFollowStub{
		logs:  []string{"", "", "started\n"},
		state: []string{"backlog", "backlog", "running", "done"},
	}
	base := stub.server(t)

	var buf bytes.Buffer
	if err := followWorkLogsLoop(context.Background(), base, "", "a", &buf); err != nil {
		t.Fatalf("followWorkLogsLoop: %v", err)
	}
	if got := buf.String(); got != "started\n" {
		t.Errorf("out = %q, want %q — a backlog item should be waited on, not treated as ended", got, "started\n")
	}
}

func TestFollowWorkLogsLoop_EndsOnceTheItemIsDoneAfterAFinalPoll(t *testing.T) {
	prev := workLogsInterval
	workLogsInterval = time.Millisecond
	t.Cleanup(func() { workLogsInterval = prev })

	stub := &workLogsFollowStub{
		logs:  []string{"running\n", "running\nthe end\n"},
		state: []string{"running", "done"},
	}
	base := stub.server(t)

	var buf bytes.Buffer
	if err := followWorkLogsLoop(context.Background(), base, "", "a", &buf); err != nil {
		t.Fatalf("followWorkLogsLoop: %v", err)
	}
	if got := buf.String(); !strings.Contains(got, "the end\n") {
		t.Errorf("out = %q, want the final tick's own new output printed before ending", got)
	}
}

func TestFollowWorkLogsLoop_EndsCleanlyWhenTheItemIsRemoved(t *testing.T) {
	prev := workLogsInterval
	workLogsInterval = time.Millisecond
	t.Cleanup(func() { workLogsInterval = prev })

	mux := http.NewServeMux()
	var mu sync.Mutex
	removed := false
	mux.HandleFunc("GET /v1/items/a/log", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gone := removed
		mu.Unlock()
		if gone {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"message": `the items file carries no item with id "a"`}})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"id": "a", "log": "running\n"})
	})
	mux.HandleFunc("GET /v1/items", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		removed = true
		mu.Unlock()
		json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"id": "a", "state": "running"}}})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	var buf bytes.Buffer
	if err := followWorkLogsLoop(context.Background(), srv.URL, "", "a", &buf); err != nil {
		t.Fatalf("an item removed mid-follow should end cleanly, not fail: %v", err)
	}
}

func TestFollowWorkLogsLoop_StopsWhenCancelled(t *testing.T) {
	prev := workLogsInterval
	workLogsInterval = time.Millisecond
	t.Cleanup(func() { workLogsInterval = prev })

	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/items/a/log", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"id": "a", "log": "still going\n"})
	})
	mux.HandleFunc("GET /v1/items", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"id": "a", "state": "running"}}})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	var buf bytes.Buffer
	if err := followWorkLogsLoop(ctx, srv.URL, "", "a", &buf); err != nil {
		t.Fatalf("a cancelled follow is a clean exit, got: %v", err)
	}
}
