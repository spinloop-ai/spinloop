package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/spinloop-ai/spinloop/internal/fleet"
)

// upCmd is `spinloop up`: the one-word way to start the engine for what is in
// the current directory. A fleet.yaml there makes it a fleet start — every
// node, or the ones named — and without one it is serve, resolving the
// Spinloop the same way. It is a dispatcher: each branch runs the other
// command's own body, so up cannot drift from what fleet start and serve do.
func upCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "up",
		Short: "start the engine this directory holds: the fleet, or the Spinloop's server",
		Long: `starts the engine for what is in the current directory. With a
fleet.yaml here it starts the fleet's engines — every node, or the ones named
— as spinloop fleet start does; without one it serves the Spinloop exactly as
spinloop serve does, resolving it the same way (a path, a registered alias,
SPINLOOP_ALIAS, or ./Spinloop). A fleet.yaml wins over a Spinloop. It takes
no flags; for the full options of either, use spinloop fleet start or
spinloop serve directly.`,
		Args:          cobra.ArbitraryArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(c *cobra.Command, args []string) error {
			resolve(c)
			return runUp(args)
		},
	}
	c.ValidArgsFunction = upSlot
	return c
}

// cmdUp runs the up command through the tree — the seam the suite calls
// directly.
func cmdUp(args []string) error { return execCmd(upCmd(), args) }

// runUp is the body of `spinloop up`: a fleet.yaml in the working directory
// makes it a fleet start, and anything else a serve.
func runUp(args []string) error {
	if info, err := os.Stat(fleet.DefaultFile); err == nil && !info.IsDir() {
		cfg, err := fleet.Resolve("")
		if err != nil {
			return err
		}
		// A bare up starts the whole fleet: fleet start refuses to run bare,
		// and picking one node out of several would be a guess.
		return runFleetDrive("start", cfg, len(args) == 0, args, fleetStartCall(cfg))
	}
	return runServe(args, false, false, "", "")
}

// upSlot is up's completion: the fleet's node names while a fleet file is in
// the working directory, the Spinloop slot otherwise. Completion must stay
// silent on failure, so a fleet file that cannot be read offers nothing.
func upSlot(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if info, err := os.Stat(fleet.DefaultFile); err == nil && !info.IsDir() {
		cfg, err := fleet.Resolve("")
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return cfg.Names(), cobra.ShellCompDirectiveNoFileComp
	}
	return aliasSlot(nil, args, "")
}
