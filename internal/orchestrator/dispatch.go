package orchestrator

import (
	"fmt"
	"os"
	"os/exec"
	"sync"

	"github.com/spinloop-ai/spinloop/internal/catalog"
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
		return []string{"run", "-m", key + "/" + model, instructions}
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

// Dispatcher launches admitted items as one-shot agents: it verifies the
// item's directory, applies the node's provider into the harness config,
// and runs the active harness in its single-task form in the item's
// directory, the agent's output kept per item beside the items file.
type Dispatcher struct {
	h       harness.Harness
	gateway string // the gateway's address
	token   string // the gateway's token, resolved by the caller

	// createItemDirs makes a missing item directory get created rather than
	// failing the item.
	createItemDirs bool

	mu sync.Mutex // serialises the applies into the shared harness config

	// start begins a child process; a test seam standing in for exec.
	start func(bin string, args []string, dir, logPath string, env []string) (Child, error)
}

// NewDispatcher builds a dispatcher for the active harness, pointed at the
// gateway, holding the token its caller presents. Where createItemDirs is
// set, a missing item directory is created rather than failing the item.
func NewDispatcher(h harness.Harness, gateway, token string, createItemDirs bool) *Dispatcher {
	d := &Dispatcher{h: h, gateway: gateway, token: token, createItemDirs: createItemDirs}
	d.start = startChild
	return d
}

// Launch runs the item against the node. A failure names the item and the
// cause — a missing directory, a harness without a single-task form, an
// apply the harness refused — and the caller records it failed rather than
// retrying it.
func (d *Dispatcher) Launch(item Item, node Node, logPath string) (Child, error) {
	if fi, err := os.Stat(item.Dir); err != nil || !fi.IsDir() {
		if !d.createItemDirs {
			return nil, fmt.Errorf("item %q's working directory %s does not exist", item.ID, item.Dir)
		}
		if err := os.MkdirAll(item.Dir, 0o755); err != nil {
			return nil, fmt.Errorf("item %q's working directory %s: creating it: %v", item.ID, item.Dir, err)
		}
	}
	form, ok := oneShot[d.h.Name()]
	if !ok {
		return nil, fmt.Errorf("the %q harness has no single-task form the orchestrator can run", d.h.Name())
	}
	model := node.ModelName()
	if model == "" {
		return nil, fmt.Errorf("node %s reports no model to run item %q against", node.Name, item.ID)
	}

	cat, err := catalog.LoadFrom(catalog.ResolveCatalogPath(""))
	if err != nil {
		return nil, err
	}
	p := cat.Providers[plumbingProvider]
	if p == nil {
		return nil, fmt.Errorf("the catalogue names no %q provider for the dispatch", plumbingProvider)
	}
	resolve := func(name string) string {
		// The config carries the variable's name, never the token's value:
		// the value reaches the agent through its environment.
		if name == p.APIKeyEnv {
			return d.token
		}
		return os.Getenv(name)
	}
	sel := spinloop.Selection{
		Provider:    providerKey(node),
		Model:       model,
		BaseURL:     d.gateway,
		DisplayName: "Spinloop fleet (" + node.Name + ")",
	}
	// Under the lock, with the exec inside it: the next dispatch's apply
	// must not rewrite this node's block before this agent has read it.
	d.mu.Lock()
	_, err = d.h.Apply(p, sel, 0, 0, false, resolve)
	d.mu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("applying node %s's provider for item %q: %v", node.Name, item.ID, err)
	}

	var env []string
	if p.APIKeyEnv != "" && os.Getenv(p.APIKeyEnv) == "" {
		env = append(os.Environ(), p.APIKeyEnv+"="+d.token)
	} else {
		env = os.Environ()
	}
	return d.start(d.h.Command(), form(providerKey(node), model, item.Instructions), item.Dir, logPath, env)
}

// startChild begins the agent as this process's child: its own process
// group, its working directory the item's, and its output — whatever the
// outcome — written to the item's log.
func startChild(bin string, args []string, dir, logPath string, env []string) (Child, error) {
	log, err := os.OpenFile(logPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("opening the item's log: %v", err)
	}
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
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
