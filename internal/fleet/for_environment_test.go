package fleet

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spinloop-ai/spinloop/internal/cloud"
)

// An environment the plane holds becomes a fleet of one, whose single node is
// the cloud node the same name in a fleet file would build.
func TestForEnvironmentBuildsAFleetOfOne(t *testing.T) {
	stubPlane(t, controlAnswers(`{"state":"running","healthy":true}`, http.StatusOK), "prod")

	cfg, err := ForEnvironment(context.Background(), "prod")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Nodes) != 1 {
		t.Fatalf("nodes = %d, want 1", len(cfg.Nodes))
	}
	if cfg.Nodes[0].Name != "prod" {
		t.Errorf("name = %q, want prod", cfg.Nodes[0].Name)
	}
	if cfg.Nodes[0].Kind != KindCloud {
		t.Errorf("kind = %q, want %q", cfg.Nodes[0].Kind, KindCloud)
	}
	// The node builds and answers, which is the whole claim: the fan-out and
	// the renderers need nothing special for a fleet assembled this way.
	node, err := cfg.NewNode(context.Background(), cfg.Nodes[0])
	if err != nil {
		t.Fatal(err)
	}
	if _, err := node.Status(context.Background()); err != nil {
		t.Fatalf("status: %v", err)
	}
}

// The same environment named by a fleet file and by ForEnvironment yields the
// same reading, so what a command prints does not depend on how the
// environment was named.
func TestForEnvironmentMatchesTheSameNodeInAFile(t *testing.T) {
	stubPlane(t, controlAnswers(`{"state":"running","healthy":true}`, http.StatusOK), "prod")

	path := writeFleet(t, "nodes:\n  - name: prod\n    kind: cloud\n", "")
	fromFile, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	fromEnv, err := ForEnvironment(context.Background(), "prod")
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	a := fromFile.FanOut(ctx, StatusCall)
	b := fromEnv.FanOut(ctx, StatusCall)
	if len(a) != 1 || len(b) != 1 {
		t.Fatalf("results = %d and %d, want 1 each", len(a), len(b))
	}
	if a[0].Outcome != b[0].Outcome || a[0].Status.State != b[0].Status.State {
		t.Errorf("file gave %s/%s, env gave %s/%s",
			a[0].Outcome, a[0].Status.State, b[0].Outcome, b[0].Status.State)
	}
}

// A name that is not a plain identifier fails before the plane is asked
// anything at all, and a name the plane does not hold fails naming the plane it
// was looked for on and the command that would create it.
func TestForEnvironmentRejects(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want string
	}{
		{"a path", "./cloud.json", "plain identifier"},
		{"a nested path", "envs/prod", "plain identifier"},
		{"a json file", "prod.json", "plain identifier"},
		{"empty", "", "plain identifier"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// No plane is stubbed: a name that is not an identifier must not
			// cost a call to find out it is not one.
			_, err := ForEnvironment(context.Background(), tt.env)
			if err == nil {
				t.Fatalf("ForEnvironment(%q) succeeded, want an error", tt.env)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error %q does not mention %q", err, tt.want)
			}
		})
	}
}

// A name the plane does not hold is not an environment, and the failure says
// where it looked and what to type — whoever holds the credentials, since the
// plane is shared and nothing local can settle existence.
func TestForEnvironmentNamesThePlaneItLookedOn(t *testing.T) {
	stubPlaneEmpty(t)
	_, err := ForEnvironment(context.Background(), "nope")
	if err == nil {
		t.Fatal("want an error")
	}
	var missing *cloud.NotDeployed
	if !errors.As(err, &missing) {
		t.Fatalf("error is %T, want *cloud.NotDeployed: %v", err, err)
	}
	if missing.Region != "us-east-1" {
		t.Errorf("region = %q, want the plane it was looked for on", missing.Region)
	}
	for _, want := range []string{"nope", "us-east-1", "spinloop cloud deploy --node nope"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

// A plane that cannot be read is never reported as an empty one: an unusable
// credential or a region with no stack is not "your environment is gone".
func TestForEnvironmentReportsAnUnreadablePlane(t *testing.T) {
	t.Run("nothing resolves", func(t *testing.T) {
		stubPlaneUnresolvable(t)
		_, err := ForEnvironment(context.Background(), "prod")
		if err == nil {
			t.Fatal("want an error")
		}
		if !strings.Contains(err.Error(), "region") {
			t.Errorf("error %q should say no plane resolved and why", err)
		}
		if strings.Contains(err.Error(), "no environment") {
			t.Errorf("error %q reports an empty plane rather than an unreadable one", err)
		}
	})

	t.Run("the plane refuses the inventory", func(t *testing.T) {
		stubPlaneRefused(t, `{"error":"env is required"}`)
		_, err := ForEnvironment(context.Background(), "prod")
		if err == nil {
			t.Fatal("want an error")
		}
		if strings.Contains(err.Error(), "no environment") {
			t.Errorf("error %q reports an empty plane rather than a refused read", err)
		}
		if !strings.Contains(err.Error(), "bootstrap") {
			t.Errorf("error %q should name the fix", err)
		}
	})
}

// A colleague's deployment is driven exactly as one's own: the plane's record
// carries who deployed it, and nothing about that changes what the fleet of one
// looks like here.
func TestForEnvironmentHoldsAnotherDeployersEnvironment(t *testing.T) {
	stubPlaneEnvs(t, nil, []cloud.Environment{{
		Name: "prod", DeployedBy: "colleague", State: "stopped",
		BaseURL: "http://203.0.113.9:8000/v1",
	}})

	cfg, err := ForEnvironment(context.Background(), "prod")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Nodes) != 1 || cfg.Nodes[0].Name != "prod" {
		t.Fatalf("nodes = %+v, want just prod", cfg.Nodes)
	}
}

// A fleet of one carries no file, so nothing can read a path or a directory
// off it — and the fleet-wide settings a file would supply are absent rather
// than inherited from anywhere.
func TestForEnvironmentCarriesNoFileOrFleetWideSettings(t *testing.T) {
	stubPlane(t, controlAnswers(`{"state":"running"}`, http.StatusOK), "prod")

	cfg, err := ForEnvironment(context.Background(), "prod")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Path != "" || cfg.Dir != "" {
		t.Errorf("Path = %q, Dir = %q, want both empty", cfg.Path, cfg.Dir)
	}
	if cfg.Prefer != "" {
		t.Errorf("Prefer = %q, want unset", cfg.Prefer)
	}
	if cfg.WakePolicy != "" {
		t.Errorf("WakePolicy = %q, want unset", cfg.WakePolicy)
	}
	if cfg.Gateway != nil {
		t.Error("Gateway set, want none")
	}
	if cfg.Concurrency != nil {
		t.Error("Concurrency set, want none")
	}
	if cfg.APIKeyEnv != "" {
		t.Errorf("APIKeyEnv = %q, want unset", cfg.APIKeyEnv)
	}
	// The defaults still apply, so a caller reading them gets an answer
	// rather than a zero value it has to interpret.
	prefer, err := cfg.Preference("")
	if err != nil {
		t.Fatal(err)
	}
	if prefer != PreferIdle {
		t.Errorf("Preference = %q, want %q", prefer, PreferIdle)
	}
}

// A fleet.yaml in the working directory supplies a fleet of one with nothing:
// the target is the environment, and the file is not consulted.
func TestForEnvironmentIgnoresADirectoryFleetFile(t *testing.T) {
	stubPlane(t, controlAnswers(`{"state":"running"}`, http.StatusOK), "prod")

	dir := t.TempDir()
	body := "prefer: active\nnodes:\n  - name: other\n    host: elsewhere\n"
	if err := os.WriteFile(filepath.Join(dir, DefaultFile), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	cfg, err := ForEnvironment(context.Background(), "prod")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Nodes) != 1 || cfg.Nodes[0].Name != "prod" {
		t.Fatalf("nodes = %+v, want just prod", cfg.Nodes)
	}
	if cfg.Prefer != "" {
		t.Errorf("Prefer = %q — the directory's fleet file was consulted", cfg.Prefer)
	}
}

// A cloud node names no token variable, so a fleet of one resolves none and
// never looks for a .env — which is what lets it carry no directory at all.
func TestForEnvironmentReadsNoAdjacentEnvFile(t *testing.T) {
	stubPlane(t, controlAnswers(`{"state":"running"}`, http.StatusOK), "prod")

	dir := t.TempDir()
	// A .env that would be read if the lookup ever ran, holding a value no
	// node should pick up.
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("PROD_TOKEN=from-dotenv\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	cfg, err := ForEnvironment(context.Background(), "prod")
	if err != nil {
		t.Fatal(err)
	}
	token, err := cfg.Token(cfg.Nodes[0])
	if err != nil {
		t.Fatalf("resolving a token for a cloud node: %v", err)
	}
	if token != "" {
		t.Errorf("token = %q, want empty: a cloud node names none", token)
	}
}

// The plane is asked for its inventory once per fleet rather than once per
// node: a fan-out over ten cloud nodes reads it once, so a board refresh does
// not multiply a read that always gives the same answer.
func TestEnvironmentsReadsThePlaneOncePerFleet(t *testing.T) {
	reads := 0
	stubPlaneCounting(t, []cloud.Environment{{Name: "a"}, {Name: "b"}, {Name: "c"}}, &reads)

	path := writeFleet(t, "nodes:\n"+
		"  - name: a\n    kind: cloud\n"+
		"  - name: b\n    kind: cloud\n"+
		"  - name: c\n    kind: cloud\n", "")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	results := cfg.FanOut(context.Background(), StatusCall)
	if len(results) != 3 {
		t.Fatalf("results = %d, want 3", len(results))
	}
	for _, r := range results {
		if !r.OK() {
			t.Errorf("node %s = %+v, want observed", r.Name, r)
		}
	}
	if reads != 1 {
		t.Errorf("inventory reads = %d, want the one read the fleet shares", reads)
	}
}
