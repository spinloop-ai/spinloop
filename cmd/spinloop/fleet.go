// The `fleet` command group: one spinloop observing every engine you run.
// It reads a fleet.yaml naming the machines, fans out over their daemon
// control APIs, and renders the cluster. Observation is fleet-wide; starting
// and stopping an engine is deliberately one node at a time.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spinloop-ai/spinloop/internal/config"
	"github.com/spinloop-ai/spinloop/internal/daemon"
	"github.com/spinloop-ai/spinloop/internal/fleet"
)

// cmdFleet runs the fleet subcommands through the tree — the seam the suite
// calls directly.
func cmdFleet(args []string) error {
	return execCmd(fleetCmd(), args)
}

// fleetFileFlag is the --fleet flag's help, shared by every fleet subcommand.
const fleetFileUsage = "path to the fleet file (default ./fleet.yaml)"

// fleetStatusCmd reports every node's engine state, one row per node. A node
// that cannot be reached is a row, not a failure: the rest of the fleet still
// renders and the command still succeeds.
func fleetStatusCmd() *cobra.Command {
	var path string
	c := &cobra.Command{
		Use:           "status",
		Short:         "report every node's engine state",
		Args:          cobra.ArbitraryArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(c *cobra.Command, _ []string) error {
			resolve(c)
			cfg, err := fleet.Resolve(path)
			if err != nil {
				return err
			}
			results := cfg.FanOut(context.Background(), fleet.StatusCall)
			renderFleetStatus(os.Stdout, results)
			return nil
		},
	}
	c.Flags().StringVar(&path, "fleet", "", fleetFileUsage)
	c.ValidArgsFunction = noPositionals
	compRegister(c, "fleet", compFiles)
	return c
}

// renderFleetStatus writes the status table: node, state, what it serves, and
// the reason when a node did not answer.
func renderFleetStatus(w io.Writer, results []fleet.NodeResult) {
	nameWidth := len("NODE")
	for _, r := range results {
		if len(r.Name) > nameWidth {
			nameWidth = len(r.Name)
		}
	}
	fmt.Fprintf(w, "%-*s  %-12s  %s\n", nameWidth, "NODE", "STATE", "SERVING")
	for _, r := range results {
		state, serving := fleetRow(r)
		fmt.Fprintf(w, "%-*s  %-12s  %s\n", nameWidth, r.Name, state, serving)
	}
}

// fleetRow renders one result's state and detail columns. A failed node shows
// its outcome as the state and the reason where the model would be, so the
// table stays one line per node however the node is doing.
func fleetRow(r fleet.NodeResult) (state, serving string) {
	if !r.OK() {
		return string(r.Outcome), r.Detail()
	}
	// The shared facts come from the same source the remote status view reads,
	// so the two cannot word or compute them differently.
	f := statusFact{
		State:         r.Status.State,
		Model:         r.Status.Model,
		Runner:        r.Status.Runner,
		Version:       r.Status.Version,
		UptimeSeconds: r.Status.UptimeSeconds,
		LastActiveAt:  r.Status.LastActiveAt,
		IdleSeconds:   r.Status.IdleSeconds,
	}
	return f.State, f.servingText()
}

// fleetMetricsCmd renders every node's engine and system metrics. --watch
// redraws the whole fleet on an interval.
func fleetMetricsCmd() *cobra.Command {
	var (
		path   string
		format string
		watch  bool
	)
	c := &cobra.Command{
		Use:           "metrics",
		Short:         "sample every node's engine metrics",
		Args:          cobra.ArbitraryArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(c *cobra.Command, _ []string) error {
			resolve(c)
			if format != "bar" && format != "table" && format != "json" {
				return fmt.Errorf("--format must be \"bar\", \"table\", or \"json\", got %q", format)
			}
			cfg, err := fleet.Resolve(path)
			if err != nil {
				return err
			}
			if watch {
				return runFleetMetricsWatch(cfg, format)
			}
			results := cfg.FanOut(context.Background(), fleet.MetricsCall)
			return renderFleetMetrics(os.Stdout, results, format)
		},
	}
	fs := c.Flags()
	fs.StringVar(&path, "fleet", "", fleetFileUsage)
	fs.StringVar(&format, "format", "bar", "output format: bar (default), table or json")
	fs.BoolVarP(&watch, "watch", "w", false, "redraw the fleet every 60 seconds")
	c.ValidArgsFunction = noPositionals
	compRegister(c, "fleet", compFiles)
	return c
}

// runFleetMetricsWatch redraws the fleet until interrupted. Each refresh is
// rendered into a buffer first, so the screen is cleared and rewritten in one
// go — a slow node delays a refresh but never tears the display.
func runFleetMetricsWatch(cfg *fleet.Config, format string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	go func() {
		<-sigCh
		cancel()
	}()

	first := true
	for {
		var buf strings.Builder
		results := cfg.FanOut(ctx, fleet.MetricsCall)
		if ctx.Err() != nil {
			return nil
		}
		if err := renderFleetMetrics(&buf, results, format); err != nil {
			return err
		}
		if !first {
			fmt.Fprint(os.Stdout, "\033[2J\033[H")
		}
		first = false
		fmt.Fprint(os.Stdout, buf.String())

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(metricsWatchInterval):
		}
	}
}

// renderFleetMetrics writes every node's metrics in the chosen format.
// Unreachable nodes are reported rather than omitted, so the view always
// accounts for the whole fleet.
func renderFleetMetrics(w io.Writer, results []fleet.NodeResult, format string) error {
	if format == "json" {
		return renderFleetMetricsJSON(w, results)
	}
	for i, r := range results {
		if i > 0 {
			fmt.Fprintln(w)
		}
		if !r.OK() {
			fmt.Fprintf(w, "%s  %s: %s\n", r.Name, r.Outcome, r.Detail())
			continue
		}
		stats := r.Metrics
		fmt.Fprintf(w, "%s  %s", r.Name, stats.State)
		if stats.ModelID != "" {
			fmt.Fprintf(w, "  %s", stats.ModelID)
		}
		fmt.Fprintln(w)
		// Before the continue, for the same reason the remote formats show it
		// before theirs: a node whose engine has stopped still has a useful
		// answer to "when did this last do anything?".
		renderLastActiveIndented(w, stats.LastActiveAt, stats.IdleSeconds)
		if stats.State != "running" {
			continue
		}
		if format == "bar" {
			renderStatBars(w, stats.CPU, stats.Memory, stats.GPUs)
			renderTokenLines(w, stats.Tokens)
		} else {
			renderTokenLines(w, stats.Tokens)
			renderGPUTable(w, stats.GPUs)
			renderCPUMemTable(w, stats.CPU, stats.Memory)
		}
		renderCollectionErrors(os.Stderr, stats.Errors)
	}
	return nil
}

// fleetNodeJSON is one node in the JSON output: its metrics when it answered,
// its outcome and reason when it did not — so a consumer sees the whole fleet
// rather than silently missing the nodes that were down.
type fleetNodeJSON struct {
	Node    string `json:"node"`
	Outcome string `json:"outcome"`
	Error   string `json:"error,omitempty"`
	Metrics *any   `json:"metrics,omitempty"`
}

func renderFleetMetricsJSON(w io.Writer, results []fleet.NodeResult) error {
	out := make([]fleetNodeJSON, 0, len(results))
	for _, r := range results {
		entry := fleetNodeJSON{Node: r.Name, Outcome: string(r.Outcome), Error: r.Detail()}
		if r.OK() {
			var m any = r.Metrics
			entry.Metrics = &m
		}
		out = append(out, entry)
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(w, string(data))
	return nil
}

// fleetStartCmd starts one or more nodes' engines, or every node with --all.
// A kind: daemon node whose Spinloop source resolves is started with the
// deploy config that source derives (StartWith) — telling the daemon what to
// run, exactly as a routed wake already does for the Spinloop being
// launched. A kind: daemon node with no resolvable source, and a kind:
// remote node regardless, get a plain start: a remote environment's
// StartWith always refuses a config, since what it serves is fixed at
// deploy time.
func fleetStartCmd() *cobra.Command {
	var (
		path string
		all  bool
	)
	c := &cobra.Command{
		Use:           "start",
		Short:         "start one or more nodes' engines",
		Args:          cobra.ArbitraryArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(c *cobra.Command, args []string) error {
			resolve(c)
			cfg, err := fleet.Resolve(path)
			if err != nil {
				return err
			}
			return runFleetDrive("start", cfg, all, args, fleetStartCall(cfg))
		},
	}
	fs := c.Flags()
	fs.StringVar(&path, "fleet", "", fleetFileUsage)
	fs.BoolVar(&all, "all", false, "start every node in the fleet")
	c.ValidArgsFunction = noPositionals
	compRegister(c, "fleet", compFiles)
	return c
}

// fleetStopCmd stops one or more nodes' engines, or every node with --all.
// Stopping takes no config, so unlike start it has nothing to resolve — only
// its target selection is shared with start.
func fleetStopCmd() *cobra.Command {
	var (
		path string
		all  bool
	)
	c := &cobra.Command{
		Use:           "stop",
		Short:         "stop one or more nodes' engines",
		Args:          cobra.ArbitraryArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(c *cobra.Command, args []string) error {
			resolve(c)
			cfg, err := fleet.Resolve(path)
			if err != nil {
				return err
			}
			call := func(ctx context.Context, n fleet.Node) fleet.NodeResult {
				status, err := n.Stop(ctx)
				return fleet.Result(n.Name(), err, status)
			}
			return runFleetDrive("stop", cfg, all, args, call)
		},
	}
	fs := c.Flags()
	fs.StringVar(&path, "fleet", "", fleetFileUsage)
	fs.BoolVar(&all, "all", false, "stop every node in the fleet")
	c.ValidArgsFunction = noPositionals
	compRegister(c, "fleet", compFiles)
	return c
}

// fleetStartCall builds fleet start's per-node call, closing over cfg so it
// can recover each targeted node's NodeConfig (kind, File) from the bare
// Node fleet.Call is handed — Call's signature carries no NodeConfig, but
// every node it is called with came from cfg in the first place, so the
// lookup by name always succeeds.
func fleetStartCall(cfg *fleet.Config) fleet.Call {
	return func(ctx context.Context, n fleet.Node) fleet.NodeResult {
		entry, _ := cfg.Node(n.Name())
		if entry.Kind != fleet.KindDaemon {
			status, err := n.Start(ctx)
			return fleet.Result(n.Name(), err, status)
		}
		arg, source, err := resolveNodeSpinloop(entry, cfg.Dir)
		if err != nil {
			return fleet.Result(n.Name(), err, daemon.StatusResponse{})
		}
		sel, spinloopPath, err := readSpinloop(fmt.Sprintf("spinloop fleet start %s", n.Name()), arg)
		if err != nil {
			return fleet.Result(n.Name(), err, daemon.StatusResponse{})
		}
		if err := applySpinloopEnv(sel, spinloopPath); err != nil {
			return fleet.Result(n.Name(), err, daemon.StatusResponse{})
		}
		dc, err := deployConfigForNode(sel, spinloopPath)
		if err != nil {
			return fleet.Result(n.Name(), err, daemon.StatusResponse{})
		}
		engineKey, err := cfg.EngineToken(entry)
		if err != nil {
			return fleet.Result(n.Name(), err, daemon.StatusResponse{})
		}
		fmt.Printf("%s: using %s (%s)\n", n.Name(), spinloopPath, source)
		status, err := n.StartWith(ctx, &dc, engineKey)
		return fleet.Result(n.Name(), err, status)
	}
}

// runFleetDrive selects the nodes a mutating fleet command targets and fans
// call out over them: no names and no --all fails, listing the fleet's
// nodes; --all and names together fails as ambiguous; named nodes fail
// before anything is touched if any name is unknown. Replaces the old
// driveOneNode now that start and stop both take several nodes or --all,
// not just one.
func runFleetDrive(verb string, cfg *fleet.Config, all bool, names []string, call fleet.Call) error {
	if all && len(names) > 0 {
		return fmt.Errorf("spinloop fleet %s: --all is ambiguous with node names", verb)
	}
	var target *fleet.Config
	if all {
		target = cfg
	} else {
		if len(names) == 0 {
			return fmt.Errorf(
				"spinloop fleet %s needs a node, or --all: %s",
				verb, strings.Join(cfg.Names(), ", "))
		}
		narrowed, err := cfg.OnlyNames(names)
		if err != nil {
			return err
		}
		target = narrowed
	}
	results := target.FanOut(context.Background(), call)
	var bad []string
	for _, r := range results {
		if !r.OK() {
			bad = append(bad, r.Name)
			fmt.Printf("%s  %s: %s\n", r.Name, r.Outcome, r.Detail())
			continue
		}
		fmt.Printf("%s  %s\n", r.Name, r.Status.State)
	}
	if len(bad) > 0 {
		return fmt.Errorf("%s: failed: %s", verb, strings.Join(bad, ", "))
	}
	return nil
}

// fleetDeployCmd creates the AWS environment for one or more kind: remote
// nodes, or every kind: remote node with --all, deriving what each serves
// from its resolved Spinloop source — the same derivation and registration
// a standalone `spinloop remote deploy` performs for one file, so the two
// can never disagree about what a given Spinloop deploys.
func fleetDeployCmd() *cobra.Command {
	var (
		path            string
		all             bool
		dryRun          bool
		overwrite       bool
		reseed          bool
		allowedCidr     string
		region          string
		spinloopVersion string
	)
	c := &cobra.Command{
		Use:   "deploy",
		Short: "create the AWS environment for one or more remote nodes",
		Long: `deploys the AWS environment for each named kind: remote node, or
every kind: remote node with --all, deriving what to serve from each node's
own Spinloop source: its file field, or its name resolved as a registered
alias or a same-named subdirectory beside the fleet file. Reuses the same
derivation, consent, and registration behavior as "spinloop remote deploy".`,
		Args:          cobra.ArbitraryArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(c *cobra.Command, args []string) error {
			resolve(c)
			return runFleetDeploy(path, all, args, deployOpts{
				dryRun:          dryRun,
				overwrite:       overwrite,
				reseed:          reseed,
				allowedCidr:     allowedCidr,
				region:          region,
				spinloopVersion: spinloopVersion,
			})
		},
	}
	fs := c.Flags()
	fs.StringVar(&path, "fleet", "", fleetFileUsage)
	fs.BoolVar(&all, "all", false, "deploy every kind: remote node in the fleet")
	fs.BoolVarP(&dryRun, "dry-run", "n", false, "print the config that would be deployed, without sending it")
	fs.BoolVar(&overwrite, "overwrite", false, "proceed against an already-registered or live environment")
	fs.BoolVar(&reseed, "reseed", false, "re-fetch the weights even if they are already in S3 (starts a ~20-minute seed)")
	fs.StringVar(&allowedCidr, "allowed-cidr", "", "who may reach each environment's instance (default: your public IP as a /32, on first deploy)")
	fs.StringVar(&region, "region", "", "AWS region of the control plane (default: AWS_REGION or us-east-1)")
	fs.StringVar(&spinloopVersion, "spinloop-version", "", "spinloop release each environment's instances install at boot (default: latest)")
	c.ValidArgsFunction = noPositionals
	compRegister(c, "fleet", compFiles)
	return c
}

// runFleetDeploy is the body of `spinloop fleet deploy`.
func runFleetDeploy(path string, all bool, names []string, opts deployOpts) error {
	cfg, err := fleet.Resolve(path)
	if err != nil {
		return err
	}
	if all && len(names) > 0 {
		return fmt.Errorf("spinloop fleet deploy: --all is ambiguous with node names")
	}

	remoteNames := make([]string, 0, len(cfg.Nodes))
	for _, n := range cfg.Nodes {
		if n.Kind == fleet.KindRemote {
			remoteNames = append(remoteNames, n.Name)
		}
	}

	var targets []string
	switch {
	case all:
		targets = remoteNames
	case len(names) == 0:
		return fmt.Errorf(
			"spinloop fleet deploy needs a node, or --all: %s",
			strings.Join(remoteNames, ", "))
	default:
		for _, name := range names {
			entry, ok := cfg.Node(name)
			if !ok {
				return fmt.Errorf("no node %q in %s (known nodes: %s)",
					name, cfg.Path, strings.Join(cfg.Names(), ", "))
			}
			if entry.Kind != fleet.KindRemote {
				return fmt.Errorf(
					"node %q is kind %q: fleet deploy provisions cloud environments, and %[1]s is not one",
					name, entry.Kind)
			}
		}
		targets = names
	}

	results := make([]fleetDeployResult, len(targets))
	var wg sync.WaitGroup
	for i, name := range targets {
		wg.Add(1)
		go func(i int, name string) {
			defer wg.Done()
			results[i] = deployOneNode(cfg, name, opts)
		}(i, name)
	}
	wg.Wait()

	var bad []string
	for _, r := range results {
		fmt.Print(r.text())
		if r.outcome != deployRowOK {
			bad = append(bad, r.node)
		}
	}
	if len(bad) > 0 {
		return fmt.Errorf("fleet deploy: failed or guarded: %s", strings.Join(bad, ", "))
	}
	return nil
}

// deployRowOutcome is one node's fleet-deploy outcome — a row, not an abort:
// one node's guard or failure never stops the others.
type deployRowOutcome int

const (
	deployRowOK deployRowOutcome = iota
	deployRowGuarded
	deployRowFailed
)

// fleetDeployResult is one targeted node's fleet-deploy outcome.
type fleetDeployResult struct {
	node    string
	outcome deployRowOutcome
	detail  string // guard/failure message, or the deploy's plan/result text on success
}

// text renders one node's result: the deploy's own plan/result text on
// success, or a labelled one-liner on guard or failure.
func (r fleetDeployResult) text() string {
	switch r.outcome {
	case deployRowGuarded:
		return fmt.Sprintf("%s: guarded: %s\n", r.node, r.detail)
	case deployRowFailed:
		return fmt.Sprintf("%s: failed: %s\n", r.node, r.detail)
	default:
		return r.detail
	}
}

// deployOneNode resolves and deploys a single targeted node. It never
// returns an error itself — a bad node becomes a fleetDeployResult, so the
// caller's fan-out can label it without aborting the others.
func deployOneNode(cfg *fleet.Config, name string, opts deployOpts) fleetDeployResult {
	entry, _ := cfg.Node(name)
	arg, source, err := resolveNodeSpinloop(entry, cfg.Dir)
	if err != nil {
		return fleetDeployResult{node: name, outcome: deployRowFailed, detail: err.Error()}
	}
	_, spinloopPath, dc, env, err := deriveDeployTarget(fmt.Sprintf("spinloop fleet deploy %s", name), arg)
	if err != nil {
		return fleetDeployResult{node: name, outcome: deployRowFailed, detail: err.Error()}
	}
	outcome, err := runDeploy(spinloopPath, env, dc, opts)
	if err != nil {
		var guarded *errDeployGuarded
		if errors.As(err, &guarded) {
			return fleetDeployResult{node: name, outcome: deployRowGuarded, detail: err.Error()}
		}
		return fleetDeployResult{node: name, outcome: deployRowFailed, detail: err.Error()}
	}
	text := fmt.Sprintf("%s: using %s (%s)\n%s", name, spinloopPath, source, outcome.Text)
	return fleetDeployResult{node: name, outcome: deployRowOK, detail: text}
}

// resolveNodeSpinloop resolves the argument to hand readSpinloop for one
// node's declared Spinloop source, trying in order and stopping at the
// first that resolves: node.File (relative to fleetDir), node.Name as a
// registered `spinloop alias`, node.Name as a subdirectory beside the fleet
// file containing a Spinloop file. source labels which one supplied it, for
// reporting. Neither `fleet deploy` nor `fleet start` falls back to acting
// without one — a node for which none resolves is a per-node failure naming
// all three.
//
// The alias tier resolves to the alias's own target path directly, rather
// than handing node.Name to readSpinloop and letting it resolve the alias
// itself: readSpinloop's resolveAlias deliberately lets a same-named path on
// disk beat a registered alias (so an existing invocation never changes
// meaning just because an alias gets registered later) — the opposite of
// this function's own precedence, alias before subdirectory. A node named
// the same as its own subdirectory would otherwise silently resolve to the
// subdirectory even with an alias registered, contradicting the order this
// function documents.
func resolveNodeSpinloop(node fleet.NodeConfig, fleetDir string) (arg, source string, err error) {
	if node.File != "" {
		return filepath.Join(fleetDir, node.File), fmt.Sprintf("file %s", node.File), nil
	}
	cfgFile, err := config.Load()
	if err != nil {
		return "", "", err
	}
	if aliasPath, ok := cfgFile.Alias(node.Name); ok {
		return aliasPath, fmt.Sprintf("alias %q", node.Name), nil
	}
	subdir := filepath.Join(fleetDir, node.Name)
	if info, statErr := os.Stat(subdir); statErr == nil && info.IsDir() {
		return subdir, fmt.Sprintf("subdirectory %s", subdir), nil
	}
	return "", "", fmt.Errorf(
		"node %q names no Spinloop source: no `file` field, no `spinloop alias` named %q, and no %s subdirectory beside the fleet file",
		node.Name, node.Name, node.Name)
}

// fleetRouteCmd reports the node a harness launch would choose for a Spinloop,
// and changes nothing: no config is pushed, no engine started, no harness
// config written. It is how a routing decision is checked before an agent
// depends on it, and how an unexpected choice is diagnosed after one.
func fleetRouteCmd() *cobra.Command {
	var path, node, prefer string
	c := &cobra.Command{
		Use:           "route",
		Short:         "report which node a launch would take",
		Args:          cobra.ArbitraryArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(c *cobra.Command, args []string) error {
			resolve(c)
			return runFleetRoute(path, node, prefer, args)
		},
	}
	fs := c.Flags()
	fs.StringVar(&path, "fleet", "", fleetFileUsage)
	fs.StringVar(&node, "node", "", "report this node rather than choosing one")
	fs.StringVar(&prefer, "prefer", "", "rank nodes by `idle` or `active` (overrides the fleet file)")
	c.ValidArgsFunction = aliasSlot
	compRegister(c, "fleet", compFiles)
	return c
}

// runFleetRoute is the body of `spinloop fleet route`.
func runFleetRoute(path, node, prefer string, args []string) error {
	var spinloopPath string
	if len(args) > 0 {
		spinloopPath = args[0]
	}
	sel, resolvedPath, err := readSpinloop("spinloop fleet route <spinloop>", spinloopPath)
	if err != nil {
		return err
	}

	target, fromFlag := path, true
	if target == "" {
		target, fromFlag = sel.Fleet, false
	}
	if target == "" {
		return fmt.Errorf(
			"%s names no FLEET: add one, or pass --fleet <path> to say which fleet to route through",
			resolvedPath)
	}
	if isEndpoint(target) {
		return fmt.Errorf(
			"FLEET %s names an endpoint, and gateway routing is not implemented yet: "+
				"name a fleet file to choose a node from", target)
	}
	cfg, err := fleet.Resolve(resolveFleetPath(target, fromFlag, resolvedPath))
	if err != nil {
		return err
	}
	preference, err := cfg.Preference(prefer)
	if err != nil {
		return err
	}
	// As for a launch: the model id a wake would push is a third name a node
	// may report itself serving.
	dc, dcErr := deployConfigForNode(sel, resolvedPath)
	want := fleet.Want{
		Model: sel.Model, Alias: sel.Alias, ModelID: dc.ModelID,
		Node: node, Prefer: preference,
	}

	fmt.Printf("Spinloop: %s\nFleet:  %s\nPrefer: %s\n\n", resolvedPath, cfg.Path, preference)
	if sel.BaseURL != "" {
		fmt.Printf("This Spinloop pins BASEURL %s, so a launch would not route at all.\n", sel.BaseURL)
		return nil
	}

	choice, err := cfg.Select(context.Background(), want)
	if err == nil {
		fmt.Printf("Would use %s at %s\n  %s\n", choice.Node.Name, choice.BaseURL, choice.Reason)
		return nil
	}

	// Nothing is serving it: say what a real launch would do next, and do
	// none of it.
	var none *fleet.ErrNoneServing
	if !errors.As(err, &none) {
		return err
	}
	fmt.Println(err)
	if dcErr != nil {
		fmt.Printf("\nA launch could not start one either: %v\n", dcErr)
		return nil
	}
	if wake, ok := cfg.WouldWake(none.Results, dc); ok {
		fmt.Printf("\nA launch would wake %s and wait for its engine. Nothing has been started.\n", wake.Name)
		return nil
	}
	fmt.Println("\nNo node could be woken for it either. Nothing has been started.")
	return nil
}

// cmdFleetRoute runs the command through the tree — the seam the suite calls.
func cmdFleetRoute(args []string) error { return execCmd(fleetRouteCmd(), args) }
