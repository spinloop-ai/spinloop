package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// The plane is the record of what is deployed on it: each node's
// deploy config, its address, its key and its instance all live there. These
// two calls ask the plane the questions the per-user registry used to answer
// from memory — what exists, and how one is taken apart — so a machine that
// has never deployed anything sees the same inventory as the one that did.
//
// Both ride endpoints the caller is already authorised to use: the inventory is
// the stats endpoint asked without naming a node, and undeploy is the
// stop endpoint asked with a different action. Listing therefore needs no AWS
// permission beyond the one that starts and stops a node.

// Node is one cloud node as the plane reports it: what it is deployed
// to serve, where it answers, whether anything is running, and who put it
// there. Name alone identifies it — a name the inventory does not carry is a
// node that does not exist on that plane, whatever any machine
// remembers about it.
type Node struct {
	Name         string `json:"node"`
	DeployedBy   string `json:"deployedBy"`
	DeployedAt   string `json:"deployedAt"`
	Runner       string `json:"runner"`
	ModelID      string `json:"modelId"`
	ServedName   string `json:"servedName"`
	BaseURL      string `json:"baseUrl"`
	State        string `json:"state"`
	InstanceType string `json:"instanceType"`
	InstanceID   string `json:"instanceId"`

	// Partial marks a node the plane could name but not fully read —
	// its instance terminated mid-read, say. It is listed anyway, with its
	// gaps shown, rather than dropping the listing that got this far.
	Partial bool `json:"partial"`
}

// Served says what a node runs, in the words an operator compares
// against the model they want: the served name the deploy gave it when it has
// one, the model id otherwise.
func (e Node) Served() string {
	if e.ServedName != "" {
		return e.ServedName
	}
	return e.ModelID
}

// Since phrases the deploy time for a listing, empty when the plane predates
// ownership or the record could not be read. A shared plane is unreadable
// without it: a name that is taken has to say who took it.
func (e Node) Since() string {
	if e.DeployedAt == "" {
		return ""
	}
	at, err := time.Parse(time.RFC3339, e.DeployedAt)
	if err != nil {
		return e.DeployedAt
	}
	return at.Local().Format("2006-01-02 15:04")
}

// Inventory asks the plane for every node deployed on it.
//
// cfg selects the plane; its Node field is deliberately ignored, since
// an inventory is not about one node. Where the plane holds nothing the
// answer is empty rather than an error, and where a per-node read failed
// the answer carries what could be read, with the failures reported beside them.
func Inventory(ctx context.Context, cfg Config) ([]Node, error) {
	if cfg.StatsURL == "" {
		return nil, fmt.Errorf(
			"the control plane reports no stats endpoint to list nodes from — " +
				"re-bootstrap it with `spinloop cloud bootstrap` (or set SPINLOOP_CLOUD_STATS_URL)")
	}
	plane := cfg
	plane.Node = ""
	status, body, err := send(ctx, plane, http.MethodGet, cfg.StatsURL, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, inventoryRefused(status, body)
	}
	var reply struct {
		Nodes  []Node   `json:"nodes"`
		Errors []string `json:"errors"`
	}
	if err := json.Unmarshal(body, &reply); err != nil {
		return nil, fmt.Errorf("parsing the control plane's inventory (HTTP %d): %w", status, err)
	}
	// A plane that answers without the field predates the inventory: saying so
	// beats reporting an empty plane, which would be a confidently wrong answer.
	if reply.Nodes == nil {
		return nil, fmt.Errorf(
			"the control plane in %s does not report its nodes — re-bootstrap it with `spinloop cloud bootstrap`",
			cfg.Region)
	}
	return reply.Nodes, nil
}

// inventoryRefused turns a rejected inventory into an error that names the
// likely cause. A control plane deployed before the inventory existed answers
// the env-less stats request the way it always has — by demanding a
// node — which would otherwise read as a broken command.
func inventoryRefused(status int, body []byte) error {
	detail := ""
	var reply struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &reply) == nil {
		detail = reply.Error
		if detail == "" {
			detail = reply.Message
		}
	}
	hint := ""
	if status == http.StatusForbidden {
		hint = forbiddenHint("", detail)
	}
	if status == http.StatusBadRequest {
		return fmt.Errorf(
			"the control plane in this region does not report its nodes (it answered %q) — "+
				"re-bootstrap it with `spinloop cloud bootstrap`%s", truncate(detail, 120), hint)
	}
	return fmt.Errorf("inventory returned HTTP %d%s: %s", status, hint, truncate(detail, 200))
}

// NotDeployed says the plane holds no node of that name. It is a
// separate outcome from a call that failed because a credential was rejected or
// a host was unreachable: existence is settled by the plane's inventory, so a
// name the plane does not carry resolves to nothing rather than to a failure,
// and a command can say which of the two happened.
type NotDeployed struct {
	Node   string
	Region string
}

func (e *NotDeployed) Error() string {
	return fmt.Sprintf(
		"the control plane in %s has no cloud node %q — `spinloop cloud deploy --node %s` creates one",
		e.Region, e.Node, e.Node)
}

// UndeployResult is what an undeploy released: the node's own resources,
// by kind, so the report says what stopped existing rather than that a call
// succeeded.
type UndeployResult struct {
	Node     string   `json:"node"`
	Released []string `json:"released"`
	DryRun   bool     `json:"dryRun"`
}

// Undeploy takes one node apart on the plane: its instance, address,
// security group, key, schedules and stored deploy configuration, and nothing
// else. The plane's shared resources and every other node are left
// alone, and a node that is not deployed answers NotDeployed rather
// than destroying nothing in silence.
//
// When dryRun is set, the plane reports what it would release and releases
// nothing.
func Undeploy(ctx context.Context, cfg Config, dryRun bool) (*UndeployResult, error) {
	if cfg.StopURL == "" {
		return nil, fmt.Errorf(
			"the control plane reports no stop endpoint to undeploy through — " +
				"re-bootstrap it with `spinloop cloud bootstrap` (or set SPINLOOP_CLOUD_STOP_URL)")
	}
	endpoint, err := url.Parse(cfg.StopURL)
	if err != nil {
		return nil, err
	}
	q := endpoint.Query()
	q.Set("action", "undeploy")
	if dryRun {
		q.Set("dryRun", "true")
	}
	endpoint.RawQuery = q.Encode()

	// Through call, like every other lifecycle verb: the node's name travels
	// as the `env` query parameter on the way out, and a plane this process
	// has outlived re-resolves once before the request is counted as failed.
	out, err := call(ctx, cfg, http.MethodPost, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	if out.StatusCode == http.StatusNotFound {
		return nil, &NotDeployed{Node: cfg.Node, Region: cfg.Region}
	}
	if out.StatusCode != http.StatusOK {
		hint := ""
		if out.StatusCode == http.StatusForbidden {
			hint = forbiddenHint(cfg.Region, out.summary())
		}
		return nil, fmt.Errorf("undeploy returned HTTP %d%s: %s",
			out.StatusCode, hint, truncate(out.summary(), 200))
	}
	// The reply names the node under the new key; a plane that predates the
	// rename still answers with the old one, and the config we sent always
	// names the node anyway, so a missing key never loses the name.
	name := out.Undeployed
	if name == "" {
		name = out.Node
	}
	if name == "" {
		name = cfg.Node
	}
	res := &UndeployResult{Node: name, Released: out.Released, DryRun: dryRun}
	if res.Node == "" {
		res.Node = cfg.Node
	}
	return res, nil
}
