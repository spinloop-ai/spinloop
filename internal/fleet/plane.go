package fleet

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/spinloop-ai/spinloop/internal/cloud"
)

// The cloud nodes of a fleet live on a control plane, and which plane that is
// comes from the operator's own credentials and region — never from the fleet
// file, which names nodes and holds no address, account or host. So the same
// committed file serves anyone with credentials for that plane.
//
// Two facts are needed per command: the plane's endpoints, which internal/cloud
// resolves once per process from the stack's outputs, and the plane's inventory
// — its own record of what is deployed — which is what settles whether a name
// is a cloud node at all. The inventory is read once per fleet rather than
// once per node, and kept in memory only, with an expiry: a one-shot command
// reads it once, and a long-lived gateway or dashboard re-reads it often enough
// that a colleague's new deployment becomes routable in seconds rather than
// never. Nothing about the plane is ever written to disk.

// planeInventoryTTL bounds how long a fleet will answer from an inventory it has
// already read. Long enough that a fan-out, and a dashboard's next refresh, cost
// nothing; short enough that no long-lived process is permanently wrong about
// what is deployed.
const planeInventoryTTL = 10 * time.Second

// planeResolve resolves the plane behind this fleet's cloud nodes, from the
// caller's credentials and region. Cheap: internal/cloud resolves it once per
// process and remembers it.
func (c *Config) planeResolve(ctx context.Context) (cloud.ControlPlane, error) {
	return cloud.ResolvePlane(ctx, cloud.PlaneRequest{GetName: os.Getenv})
}

// planeConfig returns the resolved endpoints for one cloud node: the plane's,
// with the node's name as the identifier every control call carries.
func (c *Config) planeConfig(ctx context.Context, name string) (cloud.Config, error) {
	plane, err := c.planeResolve(ctx)
	if err != nil {
		return cloud.Config{}, err
	}
	cfg := plane.Config
	cfg.Node = name
	return cfg, nil
}

// planeNodes reads the plane's inventory, once per TTL. A plane that cannot be
// reached is an error the caller reports — never an empty list, which would read
// as an empty fleet.
//
// The read is serialised through the fleet's cache: a fan-out asks from one
// goroutine per node against a cold cache, and the queue means the plane answers
// one inventory request for the whole fleet rather than one per node. A fleet
// with no cache — one assembled by hand rather than loaded or targeted — asks
// each time, which is correct and merely slower.
func (c *Config) planeNodes(ctx context.Context) ([]cloud.Node, error) {
	cache := c.plane
	if cache == nil {
		plane, err := c.planeResolve(ctx)
		if err != nil {
			return nil, err
		}
		return cloud.Inventory(ctx, plane.Config)
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if !cache.at.IsZero() && time.Since(cache.at) < planeInventoryTTL {
		return cache.nodes, cache.err
	}
	plane, err := c.planeResolve(ctx)
	if err != nil {
		// Nothing was read, so nothing is recorded: a plane that cannot be
		// resolved is asked again by the next node rather than once per fleet,
		// because resolution is memoised downstream and costs nothing to retry.
		return nil, err
	}
	nodes, err := cloud.Inventory(ctx, plane.Config)
	// The read is recorded either way, so a plane that is down is asked again
	// after the TTL rather than once per node, and an empty plane is not
	// re-read per node either.
	cache.at, cache.nodes, cache.err = time.Now(), nodes, err
	if err != nil {
		return nil, err
	}
	return nodes, nil
}

// planeNode returns the plane's record of one cloud node, and whether the
// plane holds it at all. Existence is settled by that record rather than
// inferred from a control call that failed for some other reason: a
// node the plane does not carry is not a node, whatever a machine
// remembers about it.
func (c *Config) planeNode(ctx context.Context, name string) (cloud.Node, bool, error) {
	nodes, err := c.planeNodes(ctx)
	if err != nil {
		return cloud.Node{}, false, err
	}
	i := slices.IndexFunc(nodes, func(e cloud.Node) bool { return e.Name == name })
	if i < 0 {
		return cloud.Node{}, false, nil
	}
	return nodes[i], true, nil
}

// requireCloudNode fails unless the plane holds the named cloud node, naming
// the plane it was looked for on and the command that would create it. A plane
// that could not be read reports its own problem, so a broken credential or a
// missing stack is never misread as an empty account.
func (c *Config) requireCloudNode(ctx context.Context, name string) error {
	_, found, err := c.planeNode(ctx, name)
	if err != nil {
		return err
	}
	if found {
		return nil
	}
	return &cloud.NotDeployed{Node: name, Region: c.planeRegion(ctx)}
}

// notDeployedError phrases a cloud node the plane does not
// hold, for a fan-out that renders it as one row. It says where the name was
// looked for, because on a shared plane the answer to "where did my node go?" is
// usually "a different account or region", and a bare name would not say so.
func (c *Config) notDeployedError(name, region string) error {
	return fmt.Errorf(
		"no cloud node %q is deployed on %s — `spinloop cloud deploy --node %s` creates one, "+
			"and `spinloop cloud ls` lists what that plane holds",
		name, PlanePhrase(region), name)
}

// planeRegion asks the plane which region it is, for a failure text. An
// unresolvable plane has no region to name, and the caller's own failure already
// says why.
func (c *Config) planeRegion(ctx context.Context) string {
	plane, err := c.planeResolve(ctx)
	if err != nil {
		return ""
	}
	return plane.Config.Region
}

// planePhrase words a region for a person, with a fallback that still tells them
// where the name was not found.
func PlanePhrase(region string) string {
	if region == "" {
		return "the resolved control plane"
	}
	return fmt.Sprintf("the control plane in %s", region)
}

// knownNodeNames lists what the plane holds, for a failure that should
// offer the operator something to type. It never fails: an unreadable plane
// simply has nothing to offer, and the caller already has its own error.
func (c *Config) knownNodeNames(ctx context.Context) []string {
	nodes, err := c.planeNodes(ctx)
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(nodes))
	for _, e := range nodes {
		names = append(names, e.Name)
	}
	slices.Sort(names)
	return names
}

// nodeNameList words a list of names for the inside of a failure.
func nodeNameList(names []string) string {
	return strings.Join(names, ", ")
}
