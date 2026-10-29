// Which fleet a command acts on. Every fleet command and the launch path ask
// the same question — a named environment, a named file, or the working
// directory's fleet.yaml — so they ask it here, and a command line that names
// two targets is refused in one place with one sentence.

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spinloop-ai/spinloop/internal/fleet"
)

// nodeFlagTargetUsage is the --node flag's help on every fleet subcommand.
// With no fleet in force the name is the whole target: a registered
// environment's node. Beside a discovered fleet.yaml it names the one node
// of that fleet the command acts on. Beside an explicit --fleet it adds
// nothing the file does not already say, and is refused there.
const nodeFlagTargetUsage = "act on this node alone: a registered environment when no fleet is in force, else one node of the working directory's fleet"

// fleetTarget is what a command's flags say about which fleet to act on.
// Both fields empty means the working directory's fleet.yaml.
type fleetTarget struct {
	// nodeName names the one node to act on: a registered environment to
	// act on as a fleet of one when no fleet is in force, or a node of
	// the working directory's discovered fleet.
	nodeName string
	// fleetPath names a fleet file to act on.
	fleetPath string
}

// resolveFleetTarget turns what the flags say into the fleet to act on: the
// environment's fleet of one, the named file, or the working directory's
// fleet.yaml. A --node beside a discovered fleet.yaml resolves that fleet;
// commands then act on only that node, via narrowedTarget.
//
// A directory's fleet.yaml is not consulted when an environment is named, and
// is not a conflict with one: a file that happens to be in the working
// directory is not a statement of intent, and a flag is. Only two flags
// conflict.
func resolveFleetTarget(t fleetTarget) (*fleet.Config, error) {
	if err := checkOneTarget(t.nodeName, t.fleetPath); err != nil {
		return nil, err
	}
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
	return fleet.Resolve(t.fleetPath)
}

// checkOneTarget refuses --node beside an explicit --fleet. --fleet names
// the file whose nodes the command acts on; --node is the same flag's other
// answer — a registered environment, or the one node of the fleet the
// working directory happens to hold. Beside --fleet both answers can be
// meant, and one flag cannot carry a may-be, so the line is refused. The
// narrowing --fleet wants is already expressible: start, stop and deploy
// take node names as arguments, and `--node` beside a discovered fleet is
// the narrowing for the commands that fan a whole fleet.
//
// It is enforced from one place and worded once; an operator meets it once.
func checkOneTarget(nodeName, fleetTarget string) error {
	if nodeName == "" || fleetTarget == "" {
		return nil
	}
	return fmt.Errorf(
		"--node %s beside --fleet %s is two answers to one target: --fleet already names the nodes to act on — name the node as an argument to narrow it, or drop --fleet and let --node work the fleet of the working directory",
		nodeName, fleetTarget)
}

// narrowedTarget returns the fleet a command with a fleetTarget fans out
// over: the whole config when --node names nothing, or a narrowed copy of it
// holding only that node. A name no node of the resolved fleet answers was
// refused before this ran — checkOneTarget and resolveFleetTarget for a
// discovered file, ForEnvironment for a registry name — so OnlyNames here
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
