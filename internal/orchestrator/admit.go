package orchestrator

import (
	"sort"
	"time"

	"github.com/spinloop-ai/spinloop/internal/daemon"
	"github.com/spinloop-ai/spinloop/internal/fleet"
)

// Match ranks the nodes the item may run on: the nodes that carry every tag
// the item carries, of which a node whose engine is running comes before a
// node that would have to be started, and within each tier the fleet's
// preference ranks. A node that is not running is a candidate only where the
// fleet wakes; a node the gateway did not reach is no candidate at all. An
// item nothing matches gets nothing: it waits, and is tried again as the
// fleet changes.
func Match(item Item, topo Topology) []Node {
	var running, toStart []Node
	for _, n := range topo.Nodes {
		if !carriesAll(item.Tags, n.Tags) {
			continue
		}
		if !n.answered() {
			continue
		}
		if n.State == string(daemon.StateRunning) {
			running = append(running, n)
			continue
		}
		if topo.Wake {
			toStart = append(toStart, n)
		}
	}
	rankBy(topo.Prefer, running)
	rankBy(topo.Prefer, toStart)
	return append(running, toStart...)
}

// carriesAll reports whether the node carries every tag the item carries: an
// item that names no tags matches any node.
func carriesAll(itemTags []string, nodeTags map[string]string) bool {
	for _, tag := range itemTags {
		key, value, _ := fleet.SplitTag(tag)
		if nodeTags[key] != value {
			return false
		}
	}
	return true
}

// rankBy orders matching nodes best-first under the preference in force, the
// way the fleet's own ranking does: a node that reports no activity has
// never done any work, so it is the most idle there is — and correspondingly
// the least active. Ties keep the topology's order, which is the file's.
func rankBy(prefer string, nodes []Node) {
	active := make([]time.Time, len(nodes))
	everActive := make([]bool, len(nodes))
	for i, n := range nodes {
		if n.LastActiveAt != "" {
			if t, err := time.Parse(time.RFC3339, n.LastActiveAt); err == nil {
				active[i] = t
				everActive[i] = true
			}
		}
	}
	byActive := prefer == "active"
	sort.SliceStable(nodes, func(i, j int) bool {
		switch {
		case everActive[i] != everActive[j]:
			// A never-active node ranks first under idle, last under active.
			return byActive != everActive[j]
		case active[i] != active[j]:
			if byActive {
				return active[i].After(active[j])
			}
			return active[i].Before(active[j])
		default:
			return false
		}
	})
}

// Admits reports whether the in-flight set may take the item: the fleet's
// total, where declared, against the items in flight, and the limit on each
// tag the item carries, against the items in flight that carry it. The item
// counts against every tag it carries and against the total. Where the fleet
// declares no limits, every item a node will take is admitted: the engines
// absorb what they can, and the limits are the operator's, not the
// orchestrator's, to invent.
func Admits(topo Topology, inflight []Item, item Item) bool {
	c := topo.Concurrency
	if c == nil {
		return true
	}
	if c.Total != nil && len(inflight) >= *c.Total {
		return false
	}
	for _, tag := range item.Tags {
		if limit, ok := c.Tags[tag]; ok && countCarrying(inflight, tag) >= limit {
			return false
		}
	}
	return true
}

// countCarrying counts the items in flight that carry the tag.
func countCarrying(items []Item, tag string) int {
	n := 0
	for _, item := range items {
		for _, carried := range item.Tags {
			if carried == tag {
				n++
				break
			}
		}
	}
	return n
}
