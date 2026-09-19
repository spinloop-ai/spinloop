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
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spinloop-ai/spinloop/internal/fleet"
	"github.com/spinloop-ai/spinloop/internal/harness"
	"github.com/spinloop-ai/spinloop/internal/orchestrator"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func orchestratorCmd() *cobra.Command {
	var gatewayAddr, fleetPath, itemsPath, tokenEnv, harnessName, logLevel string
	var createItemDirs bool
	var listen, apiToken, apiTokenFile string
	var loopback bool
	var dispatchBackend, dispatchImage, harnessConfigPath string
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

It reads the fleet file, where there is one, only to find the gateway: its
address, and the section's token variable where no flag names one. The
gateway is the run's only view of the fleet, and it holds no node token and
no engine key. The items file is a list of items,
each with an id, the instructions its agent is given, and the directory
the agent works in:

    - id: fix-the-parser
      instructions: fix the failing tests
      dir: ./parser

An item names the tags of the nodes it may run on, and a priority, higher
first. What it keeps — one record per item, and each agent's output —
lives beside the items file: <file>.state.json and <file>.logs/. An item
that has ended is not run again on a restart.

While it works, the orchestrator serves the work list — the items, their
state, and each agent's kept output — over HTTP, the gateway's server
pattern: an address and a token, loopback the bind that needs neither
beyond the machine.

Once the work list API is ready, the command prints the startup banner and
then the work list itself, the way spinloop work list prints it — a table
on a terminal, plain lines otherwise — so a restart's recovered state shows
without a separate call to spinloop work list.`,
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(c *cobra.Command, args []string) error {
			resolve(c)
			listenAddr, err := orchestratorListenAddr(listen, c.Flags().Changed("listen"), loopback)
			if err != nil {
				return err
			}
			gateway, tokenVar, err := orchestratorGateway(c, gatewayAddr, fleetPath, tokenEnv)
			if err != nil {
				return err
			}
			return runOrchestratorCommand(gateway, itemsPath, tokenVar, harnessName, c.Flags().Changed("harness"), logLevel, createItemDirs, listenAddr, apiToken, apiTokenFile, dispatchBackend, c.Flags().Changed("dispatch"), dispatchImage, harnessConfigPath)
		},
	}

	fs := c.Flags()
	fs.StringVar(&gatewayAddr, "gateway", "", "the address of a fleet's gateway (default: the fleet file's gateway section, where no flag is given)")
	fs.StringVarP(&fleetPath, "fleet", "f", "", fleetFileUsage)
	compRegister(c, "fleet", compFiles)
	fs.StringVar(&itemsPath, "items", "./work.yaml", "the work items file to work")
	fs.StringVar(&tokenEnv, "token-env", fleet.DefaultGatewayTokenEnv, "the environment variable holding the gateway's bearer token (default: the fleet file's section, where the gateway comes from it and no flag is given)")
	fs.StringVarP(&harnessName, "harness", "H", "", "which harness to run the agents with")
	fs.StringVar(&logLevel, "log-level", "", logLevelUsage)
	fs.BoolVar(&createItemDirs, "create-item-dirs", false, "create an item's working directory if it does not exist")
	fs.StringVar(&listen, "listen", orchestrator.DefaultListen, "the address to serve the work list API on")
	fs.BoolVarP(&loopback, "loopback", "l", false, "serve the work list API on loopback on the default port ("+orchestrator.LoopbackListen+"); needs no token")
	fs.StringVar(&apiTokenFile, "api-token-file", "", "read the work list API's bearer token from this file")
	fs.StringVar(&apiToken, "api-token", "", "the work list API's bearer token")
	fs.StringVar(&dispatchBackend, "dispatch", "bare", "how an admitted item's agent runs: bare (a host process) or docker (a container)")
	fs.StringVar(&dispatchImage, "dispatch-image", "", "the agent image the docker backend runs (default: ghcr.io/spinloop-ai/agent:<spinloop's version>); only meaningful with --dispatch docker")
	fs.StringVar(&harnessConfigPath, "harness-config", "", "the harness.yaml to read (default: harness.yaml beside the items file, where one exists)")
	compRegister(c, "items", compFiles)
	compRegister(c, "harness", compHarnessNames)
	compRegister(c, "log-level", compLogLevel)
	compRegister(c, "dispatch", compDispatchBackends)
	compRegister(c, "harness-config", compFiles)
	return c
}

// cmdOrchestrator runs the command through the tree — the seam the suite calls.
func cmdOrchestrator(args []string) error { return execCmd(orchestratorCmd(), args) }

// orchestratorGateway resolves the gateway the run works against and its
// bearer token: an explicit --gateway wins outright, its token read from the
// variable the token flag names; where it is not given, the fleet file — the
// one --fleet names, or ./fleet.yaml — must name a gateway in its section,
// and the token's variable is the flag's where the flag was given, the
// section's otherwise, the value read the way the file reads one: the
// environment first, then the .env beside the file.
func orchestratorGateway(c *cobra.Command, gatewayAddr, fleetPath, tokenEnv string) (string, string, error) {
	if gatewayAddr != "" {
		token := os.Getenv(tokenEnv)
		if token == "" {
			return "", "", fmt.Errorf("the gateway %s needs its bearer token in %s, which is not set", gatewayAddr, tokenEnv)
		}
		return gatewayAddr, token, nil
	}
	path := fleetPath
	if path == "" {
		path = fleet.DefaultFile
	}
	cfg, err := fleet.Resolve(fleetPath)
	if err != nil {
		return "", "", fmt.Errorf("no gateway named: set --gateway, or keep a fleet file with a gateway section — %v", err)
	}
	gw, ok := cfg.GatewaySection()
	if !ok {
		return "", "", fmt.Errorf("no gateway named: the fleet file %s has no gateway section — set --gateway, or give it one", path)
	}
	tokenVar := gw.TokenEnv
	if c.Flags().Changed("token-env") {
		tokenVar = tokenEnv
	}
	token, err := cfg.GatewayToken(tokenVar)
	if err != nil {
		return "", "", err
	}
	return gw.URL, token, nil
}

// orchestratorListenAddr resolves the address the work list API listens on:
// --loopback takes the default, and an explicit address and --loopback are
// two answers to one question.
func orchestratorListenAddr(listen string, listenExplicit, loopback bool) (string, error) {
	if loopback && listenExplicit {
		return "", fmt.Errorf("--loopback and --listen both given: pass one")
	}
	if loopback {
		return orchestrator.LoopbackListen, nil
	}
	return listen, nil
}

// runOrchestratorCommand is the body of `spinloop orchestrator`: the checks a
// startup owes — the gateway named, its token resolvable, the harness able to
// run an item, the items file a list of items, the API's address bindable —
// and then the work list API standing before the run's first pass, the way
// the gateway has its handler in before a signal can arrive, held until the
// signal ends the run and the server goes down with it.
func runOrchestratorCommand(gatewayAddr, itemsPath, token, harnessName string, harnessChanged bool, logLevel string, createItemDirs bool, listenAddr, apiToken, apiTokenFile, dispatchBackend string, dispatchChanged bool, dispatchImage, harnessConfigPath string) error {
	hc, err := orchestrator.LoadHarnessConfig(harnessConfigPath, itemsPath)
	if err != nil {
		return err
	}
	if err := orchestrator.CheckHarnessEnvCollision(hc.Env); err != nil {
		return err
	}

	// --harness, given explicitly, wins outright; otherwise harness.yaml's
	// own harness: is tried before harness.Resolve's own env-var/stored-
	// preference/default chain.
	resolveName := harnessName
	if !harnessChanged && hc.Harness != "" {
		resolveName = hc.Harness
	}
	h, _, err := harness.Resolve(resolveName)
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
	apiTok, err := daemonToken(apiToken, apiTokenFile)
	if err != nil {
		return err
	}
	ln, err := orchestrator.Listen(listenAddr, apiTok)
	if err != nil {
		return err
	}
	defer ln.Close()

	// --dispatch, given explicitly, wins outright; otherwise harness.yaml's
	// own dispatch: is the run's choice, and the flag's default ("bare")
	// only where neither says anything.
	backend, backendSource := dispatchBackend, "--dispatch"
	if !dispatchChanged && hc.Dispatch != "" {
		backend, backendSource = hc.Dispatch, "harness.yaml's dispatch"
	}

	var dispatch orchestrator.Launcher
	switch backend {
	case "", "bare":
		dispatch = orchestrator.NewDispatcher(h, gatewayAddr, token, createItemDirs).WithHarnessConfig(hc).WithBaseDir(hc.BaseDir)
	case "docker":
		if err := orchestrator.CheckDockerReachable(); err != nil {
			return err
		}
		image := dispatchImage
		if image == "" {
			image = orchestrator.DefaultDockerImage(version)
		}
		dispatch = orchestrator.NewDockerLauncher(h, gatewayAddr, token, image, createItemDirs).WithHarnessConfig(hc).WithBaseDir(hc.BaseDir)
	default:
		return fmt.Errorf("unknown %s %q: expected bare or docker", backendSource, backend)
	}

	store, err := orchestrator.OpenStore(itemsPath)
	if err != nil {
		return err
	}
	wl, err := orchestrator.NewWorkList(itemsPath, store, dispatch, gatewayAddr, logger)
	if err != nil {
		return err
	}
	wl = wl.WithBaseDir(hc.BaseDir)
	// The store's lock goes with the process, after the server has gone
	// down and with it any request that could still touch the store.
	defer wl.Close()

	handler := orchestrator.NewHandler(wl, apiTok, logger)
	srv := &http.Server{Handler: handler}

	unit := "items"
	if len(items) == 1 {
		unit = "item"
	}
	fmt.Printf("Working %s against %s: %d %s in the backlog\n", itemsPath, gatewayAddr, len(items), unit)
	fmt.Printf("Work list on %s\n\n", ln.Addr().String())
	printOrchestratorStartupWorkList(wl.List())

	// The signal ends the run the way the loop's contract says it does: the
	// agents it has launched are stopped, their items back in the backlog.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go srv.Serve(ln)
	runErr := orchestrator.Run(ctx, orchestrator.Config{
		Gateway:    gatewayAddr,
		ItemsPath:  itemsPath,
		Topologist: orchestrator.NewGatewayTopologist(gatewayAddr, token),
		Dispatcher: dispatch,
		Log:        logger,
		WorkList:   wl,
	})
	// The run is over — a clean interrupt, or a gateway that stopped
	// answering: take the server down before the store's lock goes.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	srv.Shutdown(shutdownCtx)
	cancel()
	return runErr
}

// printOrchestratorStartupWorkList prints the run's view of the items the
// way spinloop work list prints the work list API's: a table where stdout
// is a terminal, one tab-separated line per item otherwise.
func printOrchestratorStartupWorkList(items []orchestrator.ItemView) {
	if term.IsTerminal(int(os.Stdout.Fd())) {
		fmt.Print(workListTable(items))
		return
	}
	for _, v := range items {
		fmt.Println(workListLine(v, false))
	}
}
