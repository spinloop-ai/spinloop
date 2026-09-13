// spinloop orchestrator: works a backlog of work items against a fleet at a
// pace the fleet can absorb. It holds the items, reads the fleet's topology
// from the fleet's gateway, admits items while the fleet's declared limits
// allow, and runs each admitted item as a one-shot agent of the active
// harness whose inference goes through that same gateway. The loop, the
// state beside the items file, and the dispatch live in
// internal/orchestrator; this command resolves the gateway, its token, and
// the harness, and holds the process's lifecycle.

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spinloop-ai/spinloop/internal/fleet"
	"github.com/spinloop-ai/spinloop/internal/harness"
	"github.com/spinloop-ai/spinloop/internal/orchestrator"

	"github.com/spf13/cobra"
)

func orchestratorCmd() *cobra.Command {
	var gatewayAddr, itemsPath, tokenEnv, harnessName, logLevel string
	var createItemDirs bool
	c := &cobra.Command{
		Use:   "orchestrator",
		Short: "work a backlog of items against the fleet, at the fleet's pace",
		Long: `works a backlog of work items against the fleet at a pace the fleet
can absorb: it reads the fleet's topology from the fleet's gateway,
admits an item while the fleet's declared concurrency limits allow, and
runs each admitted item as a one-shot agent of the active harness in the
item's own directory, the agent's inference going through the same
gateway. It runs in the foreground, the way spinloop gateway does: the
signal is the only exit, and a clean interrupt puts its in-flight items
back in the backlog, none lost, none run twice.

It takes no fleet file: the gateway is its only view of the fleet, and it
holds no node token and no engine key. The items file is a list of items,
each with an id, the instructions its agent is given, and the directory
the agent works in:

    - id: fix-the-parser
      instructions: fix the failing tests
      dir: ./parser

An item names the tags of the nodes it may run on, and a priority, higher
first. What it keeps — one record per item, and each agent's output —
lives beside the items file: <file>.state.json and <file>.logs/. An item
that has ended is not run again on a restart.`,
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(c *cobra.Command, args []string) error {
			resolve(c)
			return runOrchestratorCommand(gatewayAddr, itemsPath, tokenEnv, harnessName, logLevel, createItemDirs)
		},
	}
	fs := c.Flags()
	fs.StringVar(&gatewayAddr, "gateway", "", "the address of the fleet's gateway")
	fs.StringVar(&itemsPath, "items", "./work.yaml", "the work items file to work")
	fs.StringVar(&tokenEnv, "token-env", fleet.DefaultGatewayTokenEnv, "the environment variable holding the gateway's bearer token")
	fs.StringVarP(&harnessName, "harness", "H", "", "which harness to run the agents with")
	fs.StringVar(&logLevel, "log-level", "", logLevelUsage)
	fs.BoolVar(&createItemDirs, "create-item-dirs", false, "create an item's working directory if it does not exist")
	compRegister(c, "items", compFiles)
	compRegister(c, "harness", compHarnessNames)
	compRegister(c, "log-level", compLogLevel)
	return c
}

// cmdOrchestrator runs the command through the tree — the seam the suite calls.
func cmdOrchestrator(args []string) error { return execCmd(orchestratorCmd(), args) }

// runOrchestratorCommand is the body of `spinloop orchestrator`: the checks a
// startup owes — the gateway named, its token resolvable, the harness able to
// run an item, the items file a list of items — and then the loop, held
// until the signal ends it.
func runOrchestratorCommand(gatewayAddr, itemsPath, tokenEnv, harnessName, logLevel string, createItemDirs bool) error {
	if gatewayAddr == "" {
		return errors.New("--gateway is required: the address of the fleet's gateway")
	}
	token := os.Getenv(tokenEnv)
	if token == "" {
		return fmt.Errorf("the gateway %s needs its bearer token in %s, which is not set", gatewayAddr, tokenEnv)
	}
	h, _, err := harness.Resolve(harnessName)
	if err != nil {
		return err
	}
	if !orchestrator.HasOneShotForm(h.Name()) {
		return fmt.Errorf("the %s harness has no single-task form the orchestrator can run", h.Name())
	}
	items, err := orchestrator.LoadItems(itemsPath)
	if err != nil {
		return err
	}
	logger, err := commandLogger(logLevel)
	if err != nil {
		return err
	}

	unit := "items"
	if len(items) == 1 {
		unit = "item"
	}
	fmt.Printf("Working %s against %s: %d %s in the backlog\n", itemsPath, gatewayAddr, len(items), unit)

	// The signal ends the run the way the loop's contract says it does: the
	// agents it has launched are stopped, their items back in the backlog.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return orchestrator.Run(ctx, orchestrator.Config{
		Gateway:    gatewayAddr,
		ItemsPath:  itemsPath,
		Topologist: orchestrator.NewGatewayTopologist(gatewayAddr, token),
		Dispatcher: orchestrator.NewDispatcher(h, gatewayAddr, token, createItemDirs),
		Log:        logger,
	})
}
