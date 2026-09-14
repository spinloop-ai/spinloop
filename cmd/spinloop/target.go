// Which fleet a command acts on. Every fleet command and the launch path ask
// the same question — a named environment, a named file, or the working
// directory's fleet.yaml — so they ask it here, and a command line that names
// two targets is refused in one place with one sentence.

package main

import (
	"fmt"

	"github.com/spinloop-ai/spinloop/internal/fleet"
)

// envFlagTargetUsage is the --env flag's help on every fleet subcommand. It
// says what the flag targets rather than what it names, since the fleet
// commands' other target flag names a file.
const envFlagTargetUsage = "act on this registered environment instead of a fleet file"

// fleetTarget is what a command's flags say about which fleet to act on.
// Both fields empty means the working directory's fleet.yaml.
type fleetTarget struct {
	// envName names a registered environment to act on as a fleet of one.
	envName string
	// fleetPath names a fleet file to act on.
	fleetPath string
}

// resolveFleetTarget turns what the flags say into the fleet to act on: the
// environment's fleet of one, the named file, or the working directory's
// fleet.yaml.
//
// A directory's fleet.yaml is not consulted when an environment is named, and
// is not a conflict with one: a file that happens to be in the working
// directory is not a statement of intent, and a flag is. Only two flags
// conflict.
func resolveFleetTarget(t fleetTarget) (*fleet.Config, error) {
	if err := checkOneTarget(t.envName, t.fleetPath); err != nil {
		return nil, err
	}
	if t.envName != "" {
		return fleet.ForEnvironment(t.envName)
	}
	return fleet.Resolve(t.fleetPath)
}

// checkOneTarget refuses a command line that names both an environment and a
// fleet. The two are different answers to where the model is served from — a
// named environment, or a set of nodes to pick from — so naming both is a
// mistake to report rather than a precedence to resolve.
//
// fleetTarget is the fleet as the caller has already worked it out: a --fleet
// value on the fleet commands, and on the launch path the file its own
// discovery settled on. Empty means no fleet was named.
//
// It is shared by the fleet commands and the launch so that one rule is
// enforced from one place and worded once; an operator meets it once.
func checkOneTarget(envName, fleetTarget string) error {
	if envName == "" || fleetTarget == "" {
		return nil
	}
	return fmt.Errorf(
		"both an environment (--env %s) and a fleet (%s) were given: each names where the model is served from, so state one",
		envName, fleetTarget)
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
