// `spinloop metrics`: what every engine in the target is doing with its
// hardware. The target is whatever names one — a registered environment, a
// fleet file, or the fleet file in the working directory — so an environment
// and a fleet holding it are read by the one command. The formats live beside
// the fleet's other views in fleet.go and metrics_render.go; this is the
// command.

package main

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	"github.com/spinloop-ai/spinloop/internal/fleet"
)

func metricsCmd() *cobra.Command {
	var (
		path         string
		envName      string
		format       string
		spinloopPath string
		watch        bool
		withCost     bool
	)
	c := &cobra.Command{
		Use:   "metrics",
		Short: "sample every engine's metrics",
		Long: `samples what each engine in the target is doing with its hardware: its
resource use, its token and request counters, and the release the node is
running. One block per node, queried concurrently.

The target is a registered environment (--env), a fleet file (--fleet), or
the fleet.yaml in the working directory. A node that cannot be reached is
reported rather than omitted, and never fails the command.

--cost adds what a node has spent on its running session, priced from the
instance type it launched as. Only a cloud environment can be priced — a
machine you already own has no hourly rate — so nodes that cannot be read
as they are without the flag, and a target with none succeeds showing no
cost at all.`,
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(c *cobra.Command, _ []string) error {
			resolve(c)
			if err := applyReadSpinloopEnv(spinloopPath); err != nil {
				return err
			}
			if err := validateMetricsFormat(format); err != nil {
				return err
			}
			cfg, err := resolveFleetTarget(fleetTarget{envName: envName, fleetPath: path})
			if err != nil {
				return err
			}
			call := fleet.MetricsCall
			if withCost {
				call = fleet.PricedMetricsCall
			}
			if watch {
				return runFleetMetricsWatch(cfg, format, call)
			}
			results := cfg.FanOut(context.Background(), call)
			return renderFleetMetrics(os.Stdout, results, format)
		},
	}
	fs := c.Flags()
	fs.StringVarP(&path, "fleet", "f", "", fleetFileUsage)
	fs.StringVar(&envName, "env", "", envFlagTargetUsage)
	registerSpinloopEnvFlag(fs, &spinloopPath)
	fs.StringVar(&format, "format", "gauge", "output format: gauge (default), bar, table or json")
	fs.BoolVarP(&watch, "watch", "w", false, "redraw every 60 seconds")
	fs.BoolVar(&withCost, "cost", false, "include what each priceable node has cost so far")
	c.ValidArgsFunction = noPositionals
	compRegister(c, "fleet", compFiles)
	compRegister(c, "env", compEnvs)
	return c
}

// cmdMetrics runs the command through the tree — the seam the suite calls.
func cmdMetrics(args []string) error { return execCmd(metricsCmd(), args) }
