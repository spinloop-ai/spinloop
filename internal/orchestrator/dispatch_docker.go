package orchestrator

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spinloop-ai/spinloop/internal/harness"
)

// dockerConfigSuffix is the path, relative to the official agent image's
// fixed $HOME (/home/agent), each dispatchable harness resolves its own
// config file to — the same convention internal/opencode and internal/pi
// use against a real $HOME, just computed against the image's own rather
// than assumed from the orchestrator's host. A harness with a one-shot
// form but no entry here cannot run under the docker backend; lucinate has
// neither an entry here nor a one-shot form, so it never reaches this
// table at all.
var dockerConfigSuffix = map[string]string{
	"opencode": ".config/opencode/opencode.json",
	"pi":       ".pi/agent/models.json",
}

// dockerHome is the official agent image's fixed home directory, the base
// every dockerConfigSuffix mount target is resolved against.
const dockerHome = "/home/agent"

// DefaultDockerImage builds the docker backend's default image reference
// for spinloop's own version, so a given binary defaults to the image
// built alongside it: ghcr.io/spinloop-ai/agent:<version>.
func DefaultDockerImage(version string) string {
	if version == "" || version == "dev" {
		version = "latest"
	}
	return "ghcr.io/spinloop-ai/agent:" + version
}

// CheckDockerReachable runs `docker version` once, the way HasOneShotForm
// checks the chosen harness once: an unreachable daemon is refused before
// any item is worked, naming the fix, rather than failing the first item
// and leaving the rest to fail the same way one at a time.
func CheckDockerReachable() error {
	cmd := exec.Command("docker", "version")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("the docker backend needs a reachable docker daemon: %s", msg)
	}
	return nil
}

// dockerReachableGateway is the address a container reaches the gateway
// at, given the address the orchestrator itself — a bare process — reaches
// it at. A loopback-bound gateway is the orchestrator host's own loopback,
// which is not the container's: `--network host` would put it in the same
// namespace on Linux, but Docker Desktop's Mac and Windows builds only
// honour that with a setting most operators do not have on, so it cannot
// be relied on. `host.docker.internal` is the one address that reaches
// back to the real host on every platform docker runs on — Docker Desktop
// resolves it out of the box, and `--add-host
// host.docker.internal:host-gateway` (see the docker run invocation below)
// makes it resolve on a native Linux docker host too, where it is not
// automatic. A gateway already on a routable, non-loopback address is
// reachable from the container's own network exactly as it is from the
// host, unchanged.
func dockerReachableGateway(gateway string) string {
	u, err := url.Parse(gateway)
	if err != nil || u.Hostname() == "" {
		return gateway
	}
	switch u.Hostname() {
	case "localhost", "127.0.0.1", "::1":
	default:
		return gateway
	}
	host := "host.docker.internal"
	if port := u.Port(); port != "" {
		host += ":" + port
	}
	u.Host = host
	return u.String()
}

// dockerLauncher is the docker Launcher: it renders a scoped, per-launch
// config carrying one provider (harness.ConfigRenderer), mounts it and the
// item's directory into a container from the official agent image, and
// runs the harness — wrapped where harness.yaml names a lifecycle script,
// see wrapCommand — as the container's own command, on the default bridge
// network, reaching a loopback-bound gateway via host.docker.internal
// (dockerReachableGateway) rather than the host's own network namespace.
type dockerLauncher struct {
	dispatchConfig

	image     string // the agent image reference
	itemsPath string // where ConfigDirFor resolves a launch's scoped config dir from

	// run begins `docker run` for one launch as this process's own child,
	// given the full argv and the log path its output is kept at; a test
	// seam standing in for exec, mirroring Dispatcher.start.
	run func(args []string, containerName, logPath string) (Child, error)
}

// NewDockerLauncher builds a docker launcher for the active harness,
// pointed at the gateway and the agent image, holding the token its
// caller presents. Where createItemDirs is set, a missing item directory
// is created rather than failing the item, the same as the bare backend.
func NewDockerLauncher(h harness.Harness, gateway, token, image, itemsPath string, createItemDirs bool) *dockerLauncher {
	l := &dockerLauncher{
		dispatchConfig: dispatchConfig{h: h, gateway: gateway, token: token, createItemDirs: createItemDirs},
		image:          image,
		itemsPath:      itemsPath,
	}
	l.run = runDockerContainer
	return l
}

// WithHarnessConfig sets harness.yaml's own config, the way Dispatcher's
// does — see its doc comment.
func (l *dockerLauncher) WithHarnessConfig(hc HarnessConfig) *dockerLauncher {
	l.harnessConfig = hc
	return l
}

// Launch runs the item against the node inside a container. A failure
// names the item and the cause — a missing directory, a harness without a
// single-task form or a docker config mount, a docker failure — and the
// caller records it failed rather than retrying it.
func (l *dockerLauncher) Launch(item Item, node Node, logPath string) (Child, error) {
	// resolvePlan is shared with the bare backend, which needs the
	// orchestrator host's own gateway address unchanged — so the
	// substitution happens on a local copy of the config, not on
	// l.dispatchConfig itself.
	cfg := l.dispatchConfig
	cfg.gateway = dockerReachableGateway(l.gateway)
	plan, err := cfg.resolvePlan(item, node)
	if err != nil {
		return nil, err
	}
	renderer, ok := l.h.(harness.ConfigRenderer)
	if !ok {
		return nil, fmt.Errorf("the %q harness cannot render a config for the docker backend", l.h.Name())
	}
	suffix, ok := dockerConfigSuffix[l.h.Name()]
	if !ok {
		return nil, fmt.Errorf("the %q harness has no docker config mount known to the docker backend", l.h.Name())
	}
	resolve := func(name string) string {
		if name == plan.provider.APIKeyEnv {
			return l.token
		}
		return os.Getenv(name)
	}
	rendered, err := renderer.RenderProviderConfig(plan.provider, plan.sel, resolve)
	if err != nil {
		return nil, fmt.Errorf("rendering the docker config for item %q: %v", item.ID, err)
	}

	configDir := ConfigDirFor(l.itemsPath, item.ID)
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return nil, fmt.Errorf("item %q's docker config directory %s: %v", item.ID, configDir, err)
	}
	configFile := filepath.Join(configDir, filepath.Base(suffix))
	if err := os.WriteFile(configFile, rendered, 0o600); err != nil {
		return nil, fmt.Errorf("item %q's docker config %s: %v", item.ID, configFile, err)
	}

	workspace, err := filepath.Abs(item.Dir)
	if err != nil {
		return nil, fmt.Errorf("resolving item %q's directory %s: %v", item.ID, item.Dir, err)
	}
	absConfigDir, err := filepath.Abs(configDir)
	if err != nil {
		return nil, fmt.Errorf("resolving item %q's docker config directory %s: %v", item.ID, configDir, err)
	}

	bin, runArgs, wrapEnv := wrapCommand(l.h.Command(), plan.args, l.harnessConfig)
	name := dockerContainerName(item.ID)
	args := []string{
		"run", "--rm", "--name", name,
		"--add-host", "host.docker.internal:host-gateway",
		"-v", workspace + ":/workspace",
		"-w", "/workspace",
		"-v", absConfigDir + ":" + dockerHome + "/" + filepath.Dir(suffix),
	}
	for _, e := range l.providerEnv(plan) {
		args = append(args, "-e", e)
	}
	for _, e := range wrapEnv {
		args = append(args, "-e", e)
	}
	args = append(args, l.image, bin)
	args = append(args, runArgs...)

	child, err := l.run(args, name, logPath)
	if err != nil {
		return nil, fmt.Errorf("item %q: %v", item.ID, err)
	}
	return wrapChild(child, l.harnessConfig), nil
}

// dockerContainerName is the container name a launch runs under: unique
// per item, and stable, so a second launch for the same id under the same
// run cannot collide with one docker has not yet finished tearing down.
func dockerContainerName(itemID string) string {
	return "spinloop-" + itemID
}

// runDockerContainer begins `docker run` as this process's own child, in
// the foreground rather than detached, so its own exit is the container's
// exit — the same contract startChild gives for a bare process — and its
// output, whatever the outcome, is written to the item's log the same way.
func runDockerContainer(args []string, containerName, logPath string) (Child, error) {
	log, err := os.OpenFile(logPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("opening the item's log: %v", err)
	}
	cmd := exec.Command("docker", args...)
	cmd.Stdout = log
	cmd.Stderr = log
	if err := cmd.Start(); err != nil {
		log.Close()
		return nil, fmt.Errorf("starting the container: %v", err)
	}
	log.Close() // the child holds its own copy of the handle
	return &dockerChild{cmd: cmd, name: containerName}, nil
}

// dockerChild is the container a docker launch starts. Stop and Kill act
// on the container by name — a separate `docker stop`/`docker kill`
// invocation each — rather than a signal to the local `docker run`
// client: that client process simply exits, taking Wait with it, once the
// container it is attached to does.
type dockerChild struct {
	cmd  *exec.Cmd
	name string
}

// Wait blocks until the container ends, reporting its exit as the error —
// an *exec.ExitError carrying the container's own exit code, the same
// shape procChild's Wait already gives, so sentinelChild's exitCoder
// check works unchanged for either backend.
func (c *dockerChild) Wait() error { return c.cmd.Wait() }

// Stop asks the container to end politely, with no grace of docker's own:
// the orchestrator's own stopGrace already bounds the wait between Stop
// and Kill, and a second, stacked grace inside `docker stop` would only
// make an abort take longer for no benefit.
func (c *dockerChild) Stop() {
	dockerCLI("stop", "--time", "0", c.name)
}

// Kill ends the container hard.
func (c *dockerChild) Kill() {
	dockerCLI("kill", c.name)
}

// dockerCLI runs `docker <args...>`, its outcome not reported — Stop and
// Kill are already best-effort, matching the bare backend's own silent
// Stop/Kill. A test seam standing in for exec.
var dockerCLI = func(args ...string) { exec.Command("docker", args...).Run() }
