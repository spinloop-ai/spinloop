package orchestrator

import (
	"context"
	"time"

	"github.com/spinloop-ai/spinloop/internal/daemon"
)

// Topology is the orchestrator's view of the fleet: what the fleet's gateway
// reports about its nodes, and the limits the fleet's file declares. It is
// the whole of what the orchestrator knows of the fleet — it holds no fleet
// file, no node token, and no engine key, and it contacts no node itself.
type Topology struct {
	// Wake is whether the fleet starts an engine on a node that is not
	// running one: a stopped node is a candidate only where it is.
	Wake bool `json:"wake"`
	// Prefer is how the fleet ranks several matching nodes, as the file
	// declares it: empty where the file declares nothing, which ranks idle.
	Prefer string `json:"prefer"`
	// Concurrency is the fleet's declared capacity, absent where the file
	// declares no limits.
	Concurrency *Concurrency `json:"concurrency"`
	// Nodes is the fleet, in the file's order.
	Nodes []Node `json:"nodes"`
}

// Concurrency is the fleet's declared capacity: the most in-flight work
// items the fleet takes on, fleet-wide and per tag.
type Concurrency struct {
	// Total is the most items the fleet may have in flight at once; absent
	// where the file declares no fleet-wide limit.
	Total *int `json:"total"`
	// Tags bounds the items carrying each tag, keyed the way tags are named
	// — a key and a value joined by =.
	Tags map[string]int `json:"tags"`
}

// Node is one node as the gateway reports it: the file's claims about it
// (name, kind, tags) and the facts it reports (state, what it serves), or
// the way the gateway failed to reach it.
type Node struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
	// Tags is what the file gives the node: its description of what work
	// the node takes on. Empty where the file gives it none.
	Tags map[string]string `json:"tags"`
	// State is the node's state: the daemon's, where it answers, and the
	// way the call to it ended, where it does not.
	State string `json:"state"`
	// Detail is the failure's text where the node did not answer.
	Detail string `json:"detail"`
	// Model and ServedName are what the running engine serves: the model id
	// and the name it answers under where it reports one.
	Model      string `json:"model"`
	ServedName string `json:"servedName"`
	// Ready is whether the running engine has answered its own health
	// check: "ready" or "not-ready", absent where it does not apply.
	Ready string `json:"ready"`
	// LastActiveAt is when the engine last did work, RFC 3339, absent until
	// it has.
	LastActiveAt string `json:"lastActiveAt"`
	// WakeableModel is the model a request would start a node that is not
	// running with, from its own source: absent for a running node, a node
	// whose source describes no model, and a fleet that does not wake.
	WakeableModel string `json:"wakeableModel"`
}

// answered reports whether the node's daemon answered: its state is one of
// the daemon's own, not the way a call to it ended.
func (n Node) answered() bool {
	switch daemon.State(n.State) {
	case daemon.StateIdle, daemon.StateRunning, daemon.StateStopped, daemon.StateCrashed:
		return true
	}
	return false
}

// ModelName is what a request matched to this node names: for a running
// node, the served name where it reports one, else the model id — the same
// name the gateway matches a request on; for a node that is not running,
// the model a request would start it with.
func (n Node) ModelName() string {
	if n.State == string(daemon.StateRunning) {
		if n.ServedName != "" {
			return n.ServedName
		}
		return n.Model
	}
	return n.WakeableModel
}

// Topologist is how the orchestrator reads the fleet: the one interface it
// has on it. The command's implementation talks to the gateway's topology
// endpoint; tests supply a fake.
type Topologist interface {
	// Topology reads the fleet's current topology. An error means the
	// gateway did not answer — the command's own contract says that ends
	// the orchestrator, naming the gateway.
	Topology(ctx context.Context) (Topology, error)
}

// tickInterval is how often the loop re-reads the topology and the items
// file. A variable so tests do not wait.
var tickInterval = 2 * time.Second
