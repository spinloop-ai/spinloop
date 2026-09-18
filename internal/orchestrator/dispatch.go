package orchestrator

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/spinloop-ai/spinloop/internal/catalog"
	"github.com/spinloop-ai/spinloop/internal/fleet"
	"github.com/spinloop-ai/spinloop/internal/harness"
	"github.com/spinloop-ai/spinloop/internal/spinloop"
)

// plumbingProvider is the catalogue provider whose plumbing a dispatch
// writes under: the generic OpenAI-compatible endpoint, whose base URL the
// selection gives and whose key variable the token is resolved under.
const plumbingProvider = "openai-compatible"

// providerKey is the key a node's provider block is written under in the
// harness config: one per node, so a block is only ever touched by
// dispatches matched to its node, and a dispatch never rewrites another
// node's block out from under a running agent.
func providerKey(node Node) string {
	return "spinloop-orchestrator-" + node.Name
}

// HasOneShotForm reports whether the harness has a single-task form the
// orchestrator can run: the command's startup check, so a harness the
// orchestrator cannot run fails before an item is worked, not as every item
// it works.
func HasOneShotForm(harnessName string) bool {
	_, ok := oneShot[harnessName]
	return ok
}

// oneShot is the harness's non-interactive single-task form, given the
// provider key, the model, and the instructions. A harness not in the table
// has no single-task form the orchestrator knows, and a launch for it fails
// naming it.
var oneShot = map[string]func(providerKey, model, instructions string) []string{
	"opencode": func(key, model, instructions string) []string {
		// --auto: with no terminal to answer one, a permission prompt
		// opencode raises otherwise blocks forever, or — off a terminal —
		// is auto-rejected, which silently stops the agent from doing the
		// item's own work rather than failing loudly. There is no
		// dispatch-time way to know which tools an item's instructions
		// will need, so every one-shot launch trusts them all.
		return []string{"run", "-m", key + "/" + model, "--auto", instructions}
	},
	"pi": func(key, model, instructions string) []string {
		return []string{"--print", "--model", key + "/" + model, instructions}
	},
	// lucinate's one-shot form (send) takes the agent its saved state names,
	// which the orchestrator does not hold, so it has no entry here.
}

// Child is a launched agent: what the loop holds on to, reaps, and stops.
type Child interface {
	// Wait blocks until the agent ends and reports its error, if any.
	Wait() error
	// Stop asks the agent to end, politely.
	Stop()
	// Kill ends the agent hard, after the grace a Stop has had.
	Kill()
}

// Launcher starts an admitted item's harness and returns the Child that
// tracks it: what the loop holds on to, reaps, and stops. Each backend
// chooses how and where that process actually runs — a bare host process,
// a container — behind the one method; everything above it (admission,
// matching, abort, state) works the same regardless.
type Launcher interface {
	Launch(item Item, node Node, logPath string) (Child, error)
}

// dispatchConfig is what every launcher backend needs to resolve a launch
// plan: the harness, the gateway a launch's inference points at, the
// token the harness's own bearer needs, whether a missing item directory
// is created rather than failing the item, and the base an item's own
// relative directory resolves against.
type dispatchConfig struct {
	h       harness.Harness
	gateway string // the gateway's address
	token   string // the gateway's token, resolved by the caller

	// createItemDirs makes a missing item directory get created rather than
	// failing the item.
	createItemDirs bool

	// baseDir is harness.yaml's own baseDir: an item's relative dir
	// resolves against it (see ResolveItemDir) rather than the
	// orchestrator's own working directory. Empty means unchanged
	// behavior — an item's dir is used exactly as the items file gives it.
	baseDir string

	// harnessConfig is harness.yaml's own: env added to every launch, and
	// the startup/shutdown scripts wrapping it. Its env has already been
	// checked against the token's own variable (CheckHarnessEnvCollision)
	// before the run ever admits an item, so resolvePlan's caller does not
	// check it again per launch.
	harnessConfig HarnessConfig
}

// launchPlan is what every backend needs before it diverges into how the
// process actually runs: the item's resolved directory (see
// ResolveItemDir), the args a one-shot harness runs with, the catalogue
// provider its config carries, and the selection that provider is written
// under.
type launchPlan struct {
	dir      string
	args     []string
	provider *catalog.Provider
	sel      spinloop.Selection
}

// resolvePlan resolves the item's directory (see ResolveItemDir), checks
// it (creating it where cfg allows), ensures its workspace subdirectory
// exists, resolves the harness's one-shot form and the node's model, and
// builds the catalogue provider and selection a launch's config is
// written under. A failure names the item and the cause — a missing
// directory, a harness without a single-task form — and the caller
// records it failed rather than retrying it.
func (cfg dispatchConfig) resolvePlan(item Item, node Node) (launchPlan, error) {
	dir := ResolveItemDir(cfg.baseDir, item.Dir)
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		if !cfg.createItemDirs {
			return launchPlan{}, fmt.Errorf("item %q's directory %s does not exist", item.ID, dir)
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return launchPlan{}, fmt.Errorf("item %q's directory %s: creating it: %v", item.ID, dir, err)
		}
	}
	// The workspace subdirectory is the orchestrator's own, unlike dir
	// itself: always created if missing, regardless of createItemDirs.
	if err := os.MkdirAll(ItemWorkspaceDir(dir), 0o755); err != nil {
		return launchPlan{}, fmt.Errorf("item %q's workspace directory: %v", item.ID, err)
	}
	form, ok := oneShot[cfg.h.Name()]
	if !ok {
		return launchPlan{}, fmt.Errorf("the %q harness has no single-task form the orchestrator can run", cfg.h.Name())
	}
	model := node.ModelName()
	if model == "" {
		return launchPlan{}, fmt.Errorf("node %s reports no model to run item %q against", node.Name, item.ID)
	}

	cat, err := catalog.LoadFrom(catalog.ResolveCatalogPath(""))
	if err != nil {
		return launchPlan{}, err
	}
	p := cat.Providers[plumbingProvider]
	if p == nil {
		return launchPlan{}, fmt.Errorf("the catalogue names no %q provider for the dispatch", plumbingProvider)
	}
	sel := spinloop.Selection{
		Provider: providerKey(node),
		Model:    model,
		// The agent's address for the gateway is the OpenAI-compatible
		// prefix the way a gateway-routed launch gives it: a gateway at
		// http://gw:4000 is handed to the agent as http://gw:4000/v1.
		BaseURL:     fleet.EndpointBaseURL(cfg.gateway),
		DisplayName: "Spinloop fleet (" + node.Name + ")",
	}
	return launchPlan{dir: dir, args: form(providerKey(node), model, item.Instructions), provider: p, sel: sel}, nil
}

// sortedEnvPairs returns m as NAME=VALUE pairs in a stable, sorted order —
// deterministic for a test to assert on, unlike a bare map range.
func sortedEnvPairs(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, k+"="+m[k])
	}
	return out
}

// providerEnv is the token's own entry (where the provider takes one) and
// harness.yaml's env, as NAME=VALUE pairs — what a launch that starts from
// nothing (the docker backend's container) needs in full; the bare
// backend adds these to its inherited host environment instead.
func (cfg dispatchConfig) providerEnv(plan launchPlan) []string {
	var out []string
	if plan.provider.APIKeyEnv != "" {
		out = append(out, plan.provider.APIKeyEnv+"="+cfg.token)
	}
	return append(out, sortedEnvPairs(cfg.harnessConfig.Env)...)
}

// Dispatcher is the bare-process Launcher: it applies the node's provider
// into the harness's own host config, and runs the active harness in its
// single-task form in the item's workspace directory (see
// ItemWorkspaceDir), the agent's output kept per item beside the items
// file.
type Dispatcher struct {
	dispatchConfig

	mu sync.Mutex // serialises the applies into the shared harness config

	// start begins a child process; a test seam standing in for exec.
	start func(bin string, args []string, dir, logPath string, env []string) (Child, error)
}

// NewDispatcher builds a dispatcher for the active harness, pointed at the
// gateway, holding the token its caller presents. Where createItemDirs is
// set, a missing item directory is created rather than failing the item.
func NewDispatcher(h harness.Harness, gateway, token string, createItemDirs bool) *Dispatcher {
	d := &Dispatcher{dispatchConfig: dispatchConfig{h: h, gateway: gateway, token: token, createItemDirs: createItemDirs}}
	d.start = startChild
	return d
}

// WithHarnessConfig sets harness.yaml's own config — its env, and the
// startup/shutdown scripts a launch runs under. Unset (the zero value)
// means no harness.yaml at all: every launch proceeds exactly as it did
// before this existed.
func (d *Dispatcher) WithHarnessConfig(hc HarnessConfig) *Dispatcher {
	d.harnessConfig = hc
	return d
}

// WithBaseDir sets harness.yaml's own baseDir — see dispatchConfig's
// baseDir field. Unset (empty) means unchanged behavior.
func (d *Dispatcher) WithBaseDir(dir string) *Dispatcher {
	d.baseDir = dir
	return d
}

// Launch runs the item against the node as a bare process. A failure names
// the item and the cause — a missing directory, a harness without a
// single-task form, an apply the harness refused — and the caller records
// it failed rather than retrying it.
func (d *Dispatcher) Launch(item Item, node Node, logPath string) (Child, error) {
	plan, err := d.resolvePlan(item, node)
	if err != nil {
		return nil, err
	}
	resolve := func(name string) string {
		// The config carries the variable's name, never the token's value:
		// the value reaches the agent through its environment.
		if name == plan.provider.APIKeyEnv {
			return d.token
		}
		return os.Getenv(name)
	}
	// Under the lock, with the exec inside it: the next dispatch's apply
	// must not rewrite this node's block before this agent has read it.
	d.mu.Lock()
	_, err = d.h.Apply(plan.provider, plan.sel, 0, 0, false, resolve)
	d.mu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("applying node %s's provider for item %q: %v", node.Name, item.ID, err)
	}

	var env []string
	if plan.provider.APIKeyEnv != "" && os.Getenv(plan.provider.APIKeyEnv) == "" {
		env = append(os.Environ(), plan.provider.APIKeyEnv+"="+d.token)
	} else {
		env = os.Environ()
	}
	env = append(env, sortedEnvPairs(d.harnessConfig.Env)...)

	bin, args, extraEnv := wrapCommand(d.h.Command(), plan.args, d.harnessConfig)
	child, err := d.start(bin, args, ItemWorkspaceDir(plan.dir), logPath, append(env, extraEnv...))
	if err != nil {
		return nil, err
	}
	return withItemLogCopy(wrapChild(child, d.harnessConfig), logPath, ItemLogFile(plan.dir)), nil
}

// startChild begins the agent as this process's child: its own process
// group, its working directory the item's, and its output — whatever the
// outcome — written to the item's log.
func startChild(bin string, args []string, dir, logPath string, env []string) (Child, error) {
	log, err := os.OpenFile(logPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("opening the item's log: %v", err)
	}
	// opencode run, given no directory of its own, works in the directory its
	// PWD variable names rather than its own working directory, so the
	// variable the child inherits must carry the item's directory, not the
	// place the orchestrator was started from.
	abs, err := filepath.Abs(dir)
	if err != nil {
		log.Close()
		return nil, fmt.Errorf("resolving the item's directory %s: %v", dir, err)
	}
	for i, e := range env {
		if name, _, ok := strings.Cut(e, "="); ok && name == "PWD" {
			env[i] = "PWD=" + abs
		}
	}
	cmd := exec.Command(bin, args...)
	cmd.Dir = abs
	cmd.Env = env
	cmd.Stdout = log
	cmd.Stderr = log
	setProcAttr(cmd)
	if err := cmd.Start(); err != nil {
		log.Close()
		return nil, fmt.Errorf("starting the agent: %v", err)
	}
	log.Close() // the child holds its own copy of the handle
	return &procChild{cmd: cmd}, nil
}

// procChild is the process a dispatch starts.
type procChild struct{ cmd *exec.Cmd }

// Wait blocks until the agent ends, reporting its exit as the error.
func (c *procChild) Wait() error { return c.cmd.Wait() }

// Stop asks the agent's process group to end.
func (c *procChild) Stop() {
	if c.cmd.Process != nil {
		terminateGroup(c.cmd.Process)
	}
}

// Kill ends the agent's process group hard.
func (c *procChild) Kill() {
	if c.cmd.Process != nil {
		killGroup(c.cmd.Process)
	}
}
