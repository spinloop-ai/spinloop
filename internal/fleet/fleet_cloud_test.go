package fleet

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/spinloop-ai/spinloop/internal/cloud"
)

// A kind-cloud node in the fleet file is built by the regular NewNode and
// observed through the regular fan-out, its endpoints resolved from the control
// plane the caller's credentials reach.
func TestFanOutLoadsARemoteNodeFromTheFile(t *testing.T) {
	stubPlane(t, controlAnswers(`{"state":"running","healthy":true}`, http.StatusOK), "prod")

	path := writeFleet(t, "nodes:\n  - name: prod\n    kind: cloud\n", "")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	r := cfg.FanOut(context.Background(), StatusCall)
	if len(r) != 1 {
		t.Fatalf("got %d results, want 1", len(r))
	}
	if !r[0].OK() || r[0].Status.State != "running" {
		t.Errorf("cloud node via the file = %+v (err %v)", r[0], r[0].Err)
	}
}

// A node for a cloud node the plane does not hold cannot be built, so the
// fan-out reports it as a config error — naming the node and the plane it
// was looked for on — rather than blanking the view or surfacing a 401.
func TestFanOutCloudsANUnregisteredEnvAsAConfigError(t *testing.T) {
	stubPlaneEmpty(t)

	path := writeFleet(t, "nodes:\n  - name: prod\n    kind: cloud\n", "")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	r := cfg.FanOut(context.Background(), StatusCall)
	if len(r) != 1 {
		t.Fatalf("got %d results, want 1", len(r))
	}
	if r[0].Outcome != OutcomeConfigError {
		t.Errorf("outcome = %q, want %q", r[0].Outcome, OutcomeConfigError)
	}
	if r[0].Name != "prod" {
		t.Errorf("name = %q, want the node name", r[0].Name)
	}
	if !strings.Contains(r[0].Detail(), "us-east-1") {
		t.Errorf("detail %q should name the plane the name was looked for on", r[0].Detail())
	}
}

// A fleet file is safe to commit because it names nodes and holds no address,
// so the same file drives a cloud node whoever deployed it: the plane's
// record of another deployer's node builds the same node here.
func TestFanOutDrivesAnotherDeployersCloudNode(t *testing.T) {
	stubPlaneNodes(t, controlAnswers(`{"state":"running","healthy":true}`, http.StatusOK),
		[]cloud.Node{{Name: "prod", DeployedBy: "colleague", State: "running"}})

	path := writeFleet(t, "nodes:\n  - name: prod\n    kind: cloud\n", "")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	r := cfg.FanOut(context.Background(), StatusCall)
	if len(r) != 1 || !r[0].OK() {
		t.Fatalf("results = %+v, want prod observed running", r)
	}
}

// A plane that cannot be read is one bad row, not a blanked view: the daemon
// node in the same file is still observed, and the cloud row says the plane's
// own problem rather than that the node is gone.
func TestFanOutReportsAnUnreadablePlaneAsOneRow(t *testing.T) {
	stubPlaneUnresolvable(t)

	cfg := fleetFor(t, stubDaemon(t, "", "running"), "")
	cfg.Nodes = append(cfg.Nodes, NodeConfig{Name: "prod", Kind: KindCloud})

	r := cfg.FanOut(context.Background(), StatusCall)
	if len(r) != 2 {
		t.Fatalf("got %d results, want 2", len(r))
	}
	if !r[0].OK() {
		t.Errorf("daemon node = %+v, want observed whatever the plane is doing", r[0])
	}
	if r[1].Outcome != OutcomeConfigError {
		t.Errorf("cloud outcome = %q, want a config error", r[1].Outcome)
	}
	if strings.Contains(r[1].Detail(), "no cloud node") {
		t.Errorf("detail %q reports an empty plane rather than an unreadable one", r[1].Detail())
	}
}

// A fleet file that mixes a local daemon node and a cloud node is
// observed through the one fan-out, in file order, as the same kind of row.
func TestFanOutOverAMixedFile(t *testing.T) {
	stubPlane(t, controlAnswers(`{"state":"running","healthy":true}`, http.StatusOK), "prod")

	// A daemon node, built the way the daemon tests build it.
	cfg := fleetFor(t, stubDaemon(t, "", "running"), "")
	cfg.Nodes = append(cfg.Nodes, NodeConfig{Name: "prod", Kind: KindCloud})

	r := cfg.FanOut(context.Background(), StatusCall)
	if len(r) != 2 {
		t.Fatalf("got %d results, want 2", len(r))
	}
	// File order: the daemon node comes first, the cloud node after.
	if r[0].Name != "box" || r[1].Name != "prod" {
		t.Fatalf("order = %q, %q; want box then prod", r[0].Name, r[1].Name)
	}
	if !r[0].OK() || r[0].Status.State != "running" {
		t.Errorf("daemon node via the file = %+v (err %v)", r[0], r[0].Err)
	}
	if !r[1].OK() || r[1].Status.State != "running" {
		t.Errorf("cloud node via the file = %+v (err %v)", r[1], r[1].Err)
	}
}
