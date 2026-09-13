// The Cobra command tree: one source of truth for the CLI's command surface.
// Every invocation builds a fresh tree, so values parsed into one run's flags
// can never leak into the next — the property the old code got from building
// a new FlagSet per command.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spinloop-ai/spinloop/internal/fleet"
	"github.com/spinloop-ai/spinloop/internal/harness"
	"github.com/spinloop-ai/spinloop/internal/opencode"
	"github.com/spinloop-ai/spinloop/internal/remote"
	"github.com/spinloop-ai/spinloop/internal/spinloop"
)

// newRootCmd builds the command tree for one invocation of spinloop.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "spinloop",
		Short: "configure coding-agent model providers",
		Long: `configures a coding agent — a harness — to use a model provider, by
deep-merging provider settings into that harness's config. The supported
harnesses are opencode, Pi and lucinate; the harness is chosen at runtime
(--harness/-H, SPINLOOP_HARNESS, or a stored default set with spinloop harness
config --set), never baked into a Spinloop, so the same Spinloop applies to any
of them.

Each command's --help carries its full description; spinloop provider list
shows what a harness could be configured with, spinloop harness show what it
has been.`,
		Version: version,
		// The version prints as the bare version string, as the version
		// subcommand does.
		SilenceErrors: true,
		SilenceUsage:  true,
		// Cobra's own check for a word the root does not recognise (its
		// fallback while a command sets no Args) rejects it with a message
		// that can only suggest among the root's current children, so it
		// cannot say where a removed spelling moved. rootArgs replaces
		// that check, and the RunE shows the help a non-runnable root
		// used to: execute returns help for a non-runnable command before
		// it validates args, so the validator is only reached with a
		// runnable root.
		Args: rootArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			// Every non-empty first word is an error by the time rootArgs
			// has spoken, so only the bare invocation gets here.
			return pflag.ErrHelp
		},
	}
	root.SetVersionTemplate("{{.Version}}\n")
	// --version comes from the Version field; -v joins it, as both spellings
	// worked before the tree existed.
	root.InitDefaultVersionFlag()
	if f := root.Flags().Lookup("version"); f != nil {
		f.Shorthand = "v"
	}
	// The completion protocol must never write to stderr, and the engine's
	// status line flows through the command's error stream: discard it.
	// User-facing errors are unaffected — main prints them to os.Stderr
	// itself, and pflag's flag errors go to its own buffer.
	root.SetErr(io.Discard)

	root.AddCommand(
		aliasCmd(),
		unaliasCmd(),
		serveCmd(),
		upCmd(),
		daemonCmd(),
		gatewayCmd(),
		hfCmd(),
		harnessCmd(),
		providerCmd(),
		versionCmd(),
		completionCmd(),
	)

	root.AddCommand(fleetCmd())
	root.AddCommand(remoteCmd())
	defaultFlagCompletions(root)
	return root
}

// execCmd parses args through the command's flags and runs it — the seam the
// suite uses to call a command the way the tree does, without a full root
// dispatch.
func execCmd(c *cobra.Command, args []string) error {
	c.SetArgs(args)
	return c.Execute()
}

// movedTopLevelCommands maps each top-level spelling the harness/provider
// grouping removed to the command that replaced it, so the error for the old
// spelling can give the new one.
var movedTopLevelCommands = map[string]string{
	"add":            "harness add",
	"remove":         "harness remove",
	"apply":          "harness apply",
	"unapply":        "harness unapply",
	"show":           "harness show",
	"export":         "harness export",
	"list":           "provider list",
	"init-providers": "provider init",
}

// rootArgs is the root's Args validator. A first word the root does not
// recognise fails with the error Cobra's fallback used to produce — same
// message, suggestions included — unless it is one of the spellings the
// grouping removed, which fails naming the command that replaced it.
func rootArgs(c *cobra.Command, args []string) error {
	if len(args) == 0 {
		return nil
	}
	if replacement, moved := movedTopLevelCommands[args[0]]; moved {
		return fmt.Errorf("%q moved: run spinloop %s", args[0], replacement)
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "unknown command %q for %q", args[0], c.CommandPath())
	if !c.DisableSuggestions {
		if suggestions := c.SuggestionsFor(args[0]); len(suggestions) > 0 {
			sb.WriteString("\n\nDid you mean this?\n")
			for _, s := range suggestions {
				fmt.Fprintf(&sb, "\t%v\n", s)
			}
		}
	}
	return errors.New(sb.String())
}

// harnessCmd builds the `harness` command group. The group itself does
// nothing — see groupFallback; every action is a subcommand: the six that
// manage the harness's config, `open` the launch, and `config` the active
// harness's selection.
func harnessCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "harness",
		Short: "launch the active harness, or manage its config",
		Long: `launches the active harness through ` + "`open`" + `, or manages the harness
through its subcommands: add, remove, apply, unapply, show and export work on
the harness's config, ` + "`config`" + ` reports or stores which harness is the
default, and ` + "`open`" + ` launches it. The harness is chosen at runtime
(--harness/-H, SPINLOOP_HARNESS, or a stored default), never baked into a
Spinloop, so the same selection works for any of them.`,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE:          groupFallback,
	}
	c.AddCommand(addCmd(), removeCmd(), applyCmd(), unapplyCmd(), showCmd(), exportCmd(), configCmd(), openCmd())
	return c
}

// configCmd reports or stores which harness is the default. --get (the default
// when no flag is given) prints the active harness and where that choice came
// from; --set <name> stores the default and exits.
func configCmd() *cobra.Command {
	var set, harnessName string
	var get bool
	c := &cobra.Command{
		Use:   "config",
		Short: "report or store the default harness",
		Long: `reports the active harness, or stores the default. With no flag — or
--get — it prints the active harness and where that choice came from (flag,
environment, stored preference, or the default). --set <name> stores the
default harness and exits.`,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(c *cobra.Command, args []string) error {
			if set != "" {
				if err := harness.SavePreference(set); err != nil {
					return err
				}
				prefPath, _ := harness.PreferencePath()
				fmt.Printf("Default harness set to %q (stored in %s).\n", set, prefPath)
				return nil
			}
			h, source, err := harness.Resolve(harnessName)
			if err != nil {
				return err
			}
			pref, _ := harness.LoadPreference()
			fmt.Printf("Active harness: %s (from %s)\n", h.Name(), source)
			if pref == "" {
				fmt.Printf("Stored preference: none (defaults to %s)\n", harness.Default)
			} else {
				fmt.Printf("Stored preference: %s\n", pref)
			}
			fmt.Printf("Available: %s\n", strings.Join(harness.Names(), ", "))
			return nil
		},
	}
	fs := c.Flags()
	fs.BoolVar(&get, "get", false, "print the active harness (the default)")
	fs.StringVar(&set, "set", "", "store this harness as the default and exit")
	fs.StringVarP(&harnessName, "harness", "H", "", "which harness to report with --get")
	compRegister(c, "set", compHarnessNames)
	compRegister(c, "harness", compHarnessNames)
	return c
}

// openCmd builds the `open` subcommand — the launch a bare `harness` used to
// be. It disables Cobra's flag parsing because the command has to decide,
// token by token, which arguments are spinloop's own and which belong to the
// launched harness: a leading positional that names a Spinloop is consumed, a
// `--` opts out, and everything after the hand-off point forwards
// byte-for-byte. The flags are registered on the command's own flag set — the
// body parses that set, and the completion surface reads the same one, so
// parsing and completion cannot drift apart.
func openCmd() *cobra.Command {
	var harnessName, providers string
	var spinloopPath spinloopPathFlag
	var route routeOptions
	c := &cobra.Command{
		Use:   "open",
		Short: "launch the active harness, forwarding trailing args",
		Long: `launches the active harness, forwarding any trailing args to it. A
leading argument that names a Spinloop — a registered alias or a path — is
applied first and not forwarded; put -- before the harness's own args to keep
them, and a leading -- opts out of this entirely. --spinloop/-O applies a
Spinloop first, as if you had run harness apply before it. Honours -H/
--harness and SPINLOOP_HARNESS.`,
		Args:               cobra.ArbitraryArgs,
		DisableFlagParsing: true,
		SilenceErrors:      true,
		SilenceUsage:       true,
		// The engine cannot see past a DisableFlagParsing command, so every
		// word here — attached flag values included — lands in this slot.
		ValidArgsFunction: launchSlot,
		RunE: func(c *cobra.Command, args []string) error {
			resolve(c)
			// Parsing is spinloop's own (not Cobra's): spinloop's own flags are
			// recognised wherever they appear, one leading positional that
			// names a Spinloop is consumed alongside them, and everything from
			// the first argument that is neither forwards byte-for-byte.
			fs := c.Flags()
			rest, err := splitHarnessArgs(fs, &spinloopPath, args)
			if err != nil {
				return err
			}
			// -h/--help is consumed by the flag set, not intercepted: Cobra
			// registers the help flag before RunE runs, but with flag parsing
			// off it never checks it, so without this the flag would be set and
			// the agent launched. A `--` before it leaves it a forwarded
			// positional, so the harness still gets its own --help.
			if fs.Changed("help") {
				return c.Help()
			}

			h, _, err := harness.Resolve(harnessName)
			if err != nil {
				return err
			}
			// A named Spinloop — the flag's value, a leading positional, or the
			// alias SPINLOOP_ALIAS names — travels to its fleet only by flag, so
			// a fleet.yaml in the working directory is not picked up for it. A
			// valueless --spinloop wears the default Spinloop and is not named.
			route.spinloopNamed = spinloopPath.path != "" || spinloopAliasInForce()

			// A .env beside the applied Spinloop is where its keys live, so the
			// launched agent is given the same ones. Without a Spinloop there is
			// no such file and only the environment (plus any provider key
			// spinloop resolves) is passed on. remoteResp carries the live key of
			// a remote endpoint, fetched while applying so the config is
			// written knowing it will be there.
			var envDir string
			var sel spinloop.Selection
			var remoteResp *remote.Response
			var choice *fleet.Choice
			if spinloopPath.set {
				var err error
				sel, envDir, remoteResp, choice, err = applyBeforeLaunch(spinloopPath, providers, h, rest, route)
				if err != nil {
					return err
				}
			} else if route.envName != "" {
				// No Spinloop was applied, but --env still names where the
				// model is served from: configure the harness from what is
				// actually deployed there instead of doing nothing with the
				// flag.
				var err error
				sel, envDir, remoteResp, choice, err = applyFromEnvironment(providers, h, route)
				if err != nil {
					return err
				}
			} else if route.fleetPath != "" {
				return fmt.Errorf("--fleet needs a Spinloop: it is the Spinloop's model that decides which node can serve you")
			}
			return launchAgent(h, rest, providers, envDir, remoteResp, sel, spinloopPath.set, choice)
		},
	}
	fs := c.Flags()
	fs.SetInterspersed(false)
	fs.StringVarP(&harnessName, "harness", "H", "", "which harness to launch")
	fs.VarP(&spinloopPath, "spinloop", "O", "apply this Spinloop before launching (bare: ./"+spinloop.DefaultFile+")")
	// Bare -O arrives as NoOptDefVal; spinloopPathFlag maps it to the empty
	// path readSpinloop resolves as SPINLOOP_ALIAS > ./Spinloop.
	fs.Lookup("spinloop").NoOptDefVal = "true"
	fs.StringVar(&providers, "providers", "", "path to a providers.yaml override")
	fs.StringVarP(&route.envName, "env", "e", "", "launch against this registered environment (its name keys the provider, its API key is fetched and injected)")
	compRegister(c, "env", compEnvs)
	fs.StringVarP(&route.fleetPath, "fleet", "f", "", "route through this fleet file (default: ./fleet.yaml, when the Spinloop is not named)")
	fs.StringVar(&route.node, "node", "", "pin the launch to this fleet node")
	fs.StringVar(&route.prefer, "prefer", "", "rank fleet nodes by `idle` or `active` (overrides the fleet file)")
	fs.BoolVar(&route.noWake, "no-wake", false, "fail rather than starting an engine on an idle fleet node")
	fs.DurationVar(&route.wakeTimeout, "wake-timeout", 0, "how long to wait for a woken node's engine")
	return c
}

// providerCmd builds the provider parent and its subcommands. The parent
// does nothing itself — see groupFallback.
func providerCmd() *cobra.Command {
	provider := &cobra.Command{
		Use:   "provider",
		Short: "work with the provider catalogue",
		Long: `works with the provider catalogue: list shows what a harness could be
configured with, init writes the built-in catalogue out as a starting point
for a custom one. Each subcommand's --help says what it does.`,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE:          groupFallback,
	}
	provider.AddCommand(listCmd(), initProviderCmd())
	return provider
}

// launchAgent runs the harness as the launch's child: stdio and any trailing
// args forwarded, and the environment the apply step reported as the source of
// every key the agent is given. Both launch commands end here, so the agent a
// launch is given can only differ the way the apply that preceded it did.
func launchAgent(h harness.Harness, rest []string, providers, envDir string, remoteResp *remote.Response, sel spinloop.Selection, worn bool, choice *fleet.Choice) error {
	// The resolver the launch uses knows the remote key too, so every
	// key the agent is given comes from the same place the apply step
	// reported.
	resolveKey := remoteLaunchResolver(opencode.EnvResolver(envDir), remoteResp)

	// Launch the harness, forwarding stdio and any trailing args.
	bin := h.Command()
	cmd := exec.Command(bin, rest...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = harnessEnv(providers, resolveKey, remoteResp)
	// A routed launch points the agent at the node that was chosen. As
	// on the remote path, an explicit setting in the environment already
	// won: routing fills what is unset rather than overriding a
	// deliberate choice.
	if choice != nil {
		cmd.Env = setEnvIfBlank(cmd.Env, "OPENAI_BASE_URL", choice.BaseURL)
		if choice.APIKey != "" {
			cmd.Env = setEnvIfBlank(cmd.Env, "OPENAI_API_KEY", choice.APIKey)
		}
	}
	// A worn Spinloop brings its whole local environment to the launched
	// agent: its adjacent .env fills any gaps left above, and its ENV
	// instructions override everything. These shape only the child's
	// environment — spinloop never mutates its own — and follow the same
	// precedence the remote commands use: ENV > process environment >
	// .env.
	if worn {
		cmd.Env = overlayLocalEnv(cmd.Env, sel, envDir)
	}
	// lucinate reads an OpenAI-compatible key from
	// LUCINATE_OPENAI_API_KEY when its stored secret is empty — which is
	// exactly how spinloop configures it, with no secret on disk. Supply
	// the active provider's key here so the launched agent can
	// authenticate the model it boots into, without ever writing it to
	// lucinate's config. An explicit setting already in the child's env
	// wins.
	if h.Name() == "lucinate" {
		if choice != nil && choice.APIKey != "" {
			cmd.Env = setEnvIfBlank(cmd.Env, "LUCINATE_OPENAI_API_KEY", choice.APIKey)
		}
		if key, ok := lucinateLaunchKey(providers, resolveKey, sel, worn); ok {
			cmd.Env = setEnvIfAbsent(cmd.Env, "LUCINATE_OPENAI_API_KEY", key)
		}
	}
	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%s not found — install the %s harness or add it to your PATH", bin, h.Name())
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// The harness ran and chose its own exit code; surface it
			// verbatim.
			os.Exit(exitErr.ExitCode())
		}
		return err
	}
	return nil
}

// versionCmd prints the version, the same spelling the old dispatch gave
// `spinloop version`.
func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "version",
		Short:             "print the version",
		Args:              cobra.NoArgs,
		SilenceErrors:     true,
		SilenceUsage:      true,
		ValidArgsFunction: noPositionals,
		RunE: func(c *cobra.Command, _ []string) error {
			resolve(c)
			fmt.Println(version)
			return nil
		},
	}
}

// groupFallback is the RunE a command group (fleet, remote, seed) gets: bare,
// it shows the group's own help — the one cobra generates from the tree, so
// its subcommand list cannot drift from the tree — and a word that is not a
// subcommand is cobra's own unknown-command error. The help sentinel is
// pflag's, not the stdlib one: cobra's ExecuteC checks pflag.ErrHelp (cobra
// imports pflag as its flag package), and the stdlib twin would surface as a
// bare "flag: help requested" error instead of the help.
func groupFallback(c *cobra.Command, args []string) error {
	if len(args) == 0 {
		return pflag.ErrHelp
	}
	return cobra.NoArgs(c, args)
}

// fleetCmd builds the fleet parent and its subcommands. The parent does
// nothing itself — see groupFallback.
func fleetCmd() *cobra.Command {
	fleet := &cobra.Command{
		Use:   "fleet",
		Short: "observe and drive the engines in a fleet file",
		Long: `observes and drives the engines named in a fleet file (fleet.yaml by
default; --fleet names another). Observation is fleet-wide (status, metrics,
logs, and dashboard — the live tiled view); start and stop take one or more
node names, or --all for the whole fleet, and with neither they list the
fleet and touch nothing; deploy provisions kind: remote nodes' AWS
environments the same way. A node that fails is a rendered row, never an
error — only a problem with the fleet file itself fails a command.`,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE:          groupFallback,
	}
	fleet.AddCommand(
		fleetStatusCmd(),
		fleetMetricsCmd(),
		fleetLogsCmd(),
		fleetDashboardCmd(),
		fleetRouteCmd(),
		fleetHarnessCmd(),
		fleetStartCmd(),
		fleetStopCmd(),
		fleetDeployCmd(),
	)
	return fleet
}

// remoteCmd builds the remote parent and its subcommands. The parent does
// nothing itself — see groupFallback.
func remoteCmd() *cobra.Command {
	remote := &cobra.Command{
		Use:   "remote",
		Short: "control the remote GPU inference instance",
		Long: `runs the model on a cloud GPU that exists only while you use it, from
the same Spinloop. The endpoint's URLs come from the Spinloop's REMOTE — a bare
name selects an environment under ~/.config/spinloop/remotes/<name>/, a path
names a file — falling back to the default environment. Each subcommand's
--help says what that step does.`,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE:          groupFallback,
	}
	remote.AddCommand(
		remoteBootstrapCmd(),
		remoteAuthCmd(),
		remoteBakeCmd(),
		remoteStartCmd(),
		remotePauseCmd(),
		remoteRestartCmd(),
		remoteStopCmd(),
		remoteStatusCmd(),
		remoteMetricsCmd(),
		remoteLogsCmd(),
		remoteDeployCmd(),
		remoteSeedCmd(),
		remoteEnvCmd(),
		remoteListCmd(),
		remoteKeepCmd(),
	)
	return remote
}
