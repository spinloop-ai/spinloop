// Which fleet a command acts on. Every fleet command and the launch path ask
// the same question — a fleet file and optionally one node within it, else the
// working directory's fleet.yaml, else a named environment — so they ask it
// here, and one message words the failure where nothing answers.

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spinloop-ai/spinloop/internal/fleet"
)

// nodeFlagTargetUsage is the --node flag's help on every fleet subcommand.
// Beside a fleet — named by --fleet or discovered in the working directory —
// the name picks the one node of it the command acts on. With no fleet in
// force the name is the whole target: a registered environment's node.
const nodeFlagTargetUsage = "act on this node alone: one node of the fleet in force, or a registered environment when no fleet holds the name"

// fleetTarget is what a command's flags say about which fleet to act on.
// Both fields empty means the working directory's fleet.yaml.
type fleetTarget struct {
	// nodeName names the one node to act on: a node of the fleet the
	// command resolves — the named file, the working directory's
	// fleet.yaml — or, with no fleet holding it, a registered environment
	// to act on as a fleet of one.
	nodeName string
	// fleetPath names a fleet file to act on.
	fleetPath string
}

// resolveFleetTarget turns what the flags say into the fleet to act on: the
// named file, or the working directory's fleet.yaml beside which a
// discovered fleet beats the registry, or, with neither, the environment's
// fleet of one. Commands then act on only the --node's node, via
// narrowedTarget.
//
// A --fleet file answers for the node named beside it: a name the file does
// not hold fails naming the file and its nodes. Only a fleet *discovered*
// in the working directory falls through to the registry — a file that
// happens to be there is not a statement of intent, and a flag is.
func resolveFleetTarget(t fleetTarget) (*fleet.Config, error) {
	if t.nodeName != "" && t.fleetPath == "" {
		// A discovered fleet beats the registry — that is what a name
		// beside fleet.yaml answers — but only if it holds the node. A
		// name it does not hold is a registered environment's when one
		// answers it; when nothing does, a refusal naming the fleet the
		// name was checked against is the truest message.
		if _, err := os.Stat(fleet.DefaultFile); err == nil {
			cfg, ferr := fleet.Load(fleet.DefaultFile)
			if ferr != nil {
				return nil, ferr
			}
			for _, n := range cfg.Nodes {
				if n.Name == t.nodeName {
					return cfg, nil
				}
			}
			if _, rerr := fleet.ForEnvironment(t.nodeName); rerr == nil {
				return fleet.ForEnvironment(t.nodeName)
			}
			return nil, fmt.Errorf(
				"node %q is no node of %s (its nodes: %s) and no registered environment: name one of its nodes, or a registered environment",
				t.nodeName, fleet.DefaultFile, strings.Join(cfg.Names(), ", "))
		}
		return fleet.ForEnvironment(t.nodeName)
	}
	cfg, err := fleet.Resolve(t.fleetPath)
	if err != nil {
		return nil, err
	}
	if t.nodeName != "" {
		// --fleet names the fleet and --node the node within it. The
		// named file is the stated fleet, so nothing else answers for
		// it: no registry fallback, only its own nodes.
		for _, n := range cfg.Nodes {
			if n.Name == t.nodeName {
				return cfg, nil
			}
		}
		return nil, fmt.Errorf(
			"node %q is no node of %s (its nodes: %s): name one of them, or drop --fleet and name a registered environment",
			t.nodeName, t.fleetPath, strings.Join(cfg.Names(), ", "))
	}
	return cfg, nil
}

// narrowedTarget returns the fleet a command with a fleetTarget fans out
// over: the whole config when --node names nothing, or a narrowed copy of it
// holding only that node. A name no node of the resolved fleet answers was
// refused before this ran — resolveFleetTarget for both the named file and a
// discovered one, ForEnvironment for a registry name — so OnlyNames here
// cannot fail; the fall back to the whole config keeps a fan-out honest
// rather than empty should it ever change.
func narrowedTarget(cfg *fleet.Config, nodeName string) *fleet.Config {
	if nodeName == "" {
		return cfg
	}
	narrowed, err := cfg.OnlyNames([]string{nodeName})
	if err != nil {
		return cfg
	}
	return narrowed
}

// describeTarget names the fleet a command acted on, for the lines that report
// it. A fleet of one has no path to name, so it is named by its environment.
func describeTarget(cfg *fleet.Config) string {
	if cfg.Path != "" {
		return cfg.Path
	}
	if len(cfg.Nodes) == 1 {
		return fmt.Sprintf("environment %s", cfg.Nodes[0].Name)
	}
	return "the fleet"
}
