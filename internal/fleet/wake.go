// Waking a node: starting an engine on a machine that is not running one, so a
// launch has somewhere to go. It reuses the daemon's start-with-config call, so
// what a host can serve is decided by the same validation that would reject a
// bad config anywhere else — there is no second description of a node's
// capabilities to drift from the first.

package fleet

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/spinloop-ai/spinloop/internal/daemon"
	"github.com/spinloop-ai/spinloop/internal/remote"
)

// WakeTimeout bounds waiting for a woken node's engine to answer. A cold node
// loads weights first, which on a large model is minutes. A variable so tests
// do not wait.
var WakeTimeout = 5 * time.Minute

// wakePoll is how often a waking node is re-checked.
var wakePoll = 2 * time.Second

// Waker reports progress while a node is woken. A silent five-minute pause
// reads as a hang, so the caller is given something to print.
type Waker func(format string, args ...any)

// ConfigFor resolves the deploy config a candidate node would be started
// with. The launch path resolves the same config for every candidate — one
// Spinloop describes what any node should start — while the gateway resolves
// each node's own Spinloop source, so different nodes may describe different
// engines.
type ConfigFor func(entry NodeConfig) (remote.DeployConfig, error)

// ConstantConfig adapts one deploy config, already resolved, to a per-candidate
// resolver. The launch path uses it: its Spinloop describes what any node would
// start.
func ConstantConfig(dc remote.DeployConfig, err error) ConfigFor {
	return func(NodeConfig) (remote.DeployConfig, error) { return dc, err }
}

// configResolver resolves each candidate's deploy config at most once per wake,
// by node name. Resolving can cost more than a status read — the gateway
// parses each node's Spinloop source — and a wake asks for the same config
// twice per candidate: once to order them, once to start them.
type configResolver struct {
	fn   ConfigFor
	dcs  map[string]remote.DeployConfig
	errs map[string]error
}

func newConfigResolver(fn ConfigFor) *configResolver {
	return &configResolver{fn: fn, dcs: map[string]remote.DeployConfig{}, errs: map[string]error{}}
}

func (r *configResolver) config(entry NodeConfig) (remote.DeployConfig, error) {
	if err, ok := r.errs[entry.Name]; ok {
		return r.dcs[entry.Name], err
	}
	dc, err := r.fn(entry)
	r.dcs[entry.Name] = dc
	r.errs[entry.Name] = err
	return dc, err
}

// Wake starts an engine for want on a node that is not running one, and waits
// until its engine answers. Candidates are tried in fleet-file order, and a
// node whose stored config already matches is preferred: it has the weights.
//
// A node that refuses the config — a runner or model it cannot serve — is not
// fatal while other candidates remain. When none succeeds, every refusal is
// reported together.
func (c *Config) Wake(ctx context.Context, w Want, cfgFor ConfigFor, results []NodeResult, log Waker) (*Choice, error) {
	if log == nil {
		log = func(string, ...any) {}
	}
	resolver := newConfigResolver(cfgFor)
	cands := wakeable(c.candidates(results), resolver)
	if len(cands) == 0 {
		return nil, &ErrNoneServing{Results: results, Want: w, Path: c.Path}
	}

	var refused []string
	for _, cand := range cands {
		dc, err := resolver.config(cand.entry)
		if err != nil {
			refused = append(refused, fmt.Sprintf("%s: %v", cand.entry.Name, err))
			continue
		}
		node, err := c.NewNode(cand.entry)
		if err != nil {
			refused = append(refused, fmt.Sprintf("%s: %v", cand.entry.Name, err))
			continue
		}
		// The key the engine will be gated with is resolved before it starts,
		// so a variable that is set nowhere fails before anything runs rather
		// than after. A node naming none wakes an ungated engine, which is
		// right for one reached over loopback.
		engineKey, err := c.EngineToken(cand.entry)
		if err != nil {
			return nil, err
		}
		log("Waking %s to serve %s...\n", cand.entry.Name, w.wanted())
		_, err = node.StartWith(ctx, &dc, engineKey)
		if err != nil {
			// Another client may have woken this node first. That is
			// another route to the same place, not a failure — re-read
			// its state and take it if it is now serving what we want.
			// The state alone is not the answer, though: the other start may
			// still be loading, so the same readiness wait applies to a node
			// we did not start ourselves.
			if isAlreadyRunning(err) {
				if status, err := node.Status(ctx); err == nil && w.matches(servingNames(status)...) {
					log("%s was already started by someone else; waiting for its engine to answer...\n", cand.entry.Name)
					ready, err := c.waitReady(ctx, node, cand.entry, w, log)
					if err != nil {
						return nil, err
					}
					cand.result = NodeResult{Name: cand.entry.Name, Outcome: OutcomeOK, Status: ready}
					return c.choiceFor(cand, w, true, engineKey)
				}
			}
			refused = append(refused, fmt.Sprintf("%s: %v", cand.entry.Name, err))
			continue
		}
		ready, err := c.waitReady(ctx, node, cand.entry, w, log)
		if err != nil {
			return nil, err
		}
		cand.result = NodeResult{Name: cand.entry.Name, Outcome: OutcomeOK, Status: ready}
		return c.choiceFor(cand, w, true, engineKey)
	}
	return nil, fmt.Errorf(
		"no node in %s could serve %s:\n  %s",
		c.Path, w.wanted(), strings.Join(refused, "\n  "))
}

// wakeable keeps the nodes that could be started, in the order to try them: a
// node whose stored config already names the wanted model first, since it has
// the weights and starts sooner.
func wakeable(cands []candidate, resolver *configResolver) []candidate {
	var warm, cold []candidate
	for _, c := range cands {
		if !c.result.OK() || c.result.Status.State == string(daemon.StateRunning) {
			// A running engine is never displaced to make room.
			continue
		}
		dc, err := resolver.config(c.entry)
		if err == nil && c.result.Status.Model != "" && c.result.Status.Model == dc.ModelID {
			warm = append(warm, c)
			continue
		}
		cold = append(cold, c)
	}
	return append(warm, cold...)
}

// WaitLoading holds a request for a node that is already running the wanted
// model but has not answered yet, and returns it once its engine does. It is
// the counterpart to Wake for a node nothing needs to start: the weights are
// already being fetched or loaded, so waiting reaches a served request sooner
// than starting a second engine somewhere else — and on a fleet whose other
// nodes cost money to run, it avoids starting one at all.
//
// Candidates are taken in fleet-file order. Only the first is waited for:
// they are all loading the same model, so a second wait would just be the
// first one's timeout twice over.
func (c *Config) WaitLoading(ctx context.Context, w Want, results []NodeResult, log Waker) (*Choice, error) {
	if log == nil {
		log = func(string, ...any) {}
	}
	cands := loading(c.candidates(results), w)
	if len(cands) == 0 {
		return nil, &ErrNoneServing{Results: results, Want: w, Path: c.Path}
	}
	cand := cands[0]
	node, err := c.NewNode(cand.entry)
	if err != nil {
		return nil, err
	}
	log("%s is starting %s; waiting for its engine to answer...\n", cand.entry.Name, w.wanted())
	ready, err := c.waitReady(ctx, node, cand.entry, w, log)
	if err != nil {
		return nil, err
	}
	cand.result = NodeResult{Name: cand.entry.Name, Outcome: OutcomeOK, Status: ready}
	// Not woken: this engine was started by whoever started it, so its key is
	// looked up rather than being the one a wake just gated it with.
	return c.choiceFor(cand, w, false, "")
}

// waitReady blocks until the node reports running *and* its engine answers.
// The supervisor reports running when the process exists; llama.cpp then loads
// weights, so a launch that trusted the state alone would hand the agent an
// endpoint that refuses connections.
//
// A daemon that reports its own readiness reading — the engine has answered
// its health check — is taken on that word; it checked from the same machine
// the engine runs on. A daemon that reports none (older builds, or a runner
// with no known health-check convention) falls back to the TCP probe.
//
// On timeout the started engine is deliberately left running: it is probably
// still loading, and stopping it throws away the only expensive part.
func (c *Config) waitReady(ctx context.Context, node Node, entry NodeConfig, w Want, log Waker) (daemon.StatusResponse, error) {
	deadline := time.Now().Add(WakeTimeout)
	var last daemon.StatusResponse
	announced := false
	for {
		status, err := node.Status(ctx)
		if err == nil {
			last = status
			if status.State == string(daemon.StateRunning) {
				if status.Ready == daemon.ReadyYes {
					return status, nil
				}
				baseURL, urlErr := c.EngineBaseURL(entry, status)
				if urlErr != nil {
					return status, urlErr
				}
				if engineAnswers(ctx, baseURL) {
					return status, nil
				}
				if !announced {
					log("%s is up; waiting for its engine to load...\n", entry.Name)
					announced = true
				}
			}
			if status.State == string(daemon.StateCrashed) {
				return status, fmt.Errorf(
					"node %q crashed while starting %s: check its engine log (%s)",
					entry.Name, w.wanted(), status.LogPath)
			}
		}
		if time.Now().After(deadline) {
			return last, fmt.Errorf(
				"node %q did not answer within %s: its engine may still be loading, "+
					"so it has been left running — check `spinloop fleet status`",
				entry.Name, WakeTimeout)
		}
		select {
		case <-ctx.Done():
			return last, ctx.Err()
		case <-time.After(wakePoll):
		}
	}
}

// engineDialTimeout bounds one readiness probe.
var engineDialTimeout = 2 * time.Second

// engineAnswers reports whether an engine is accepting connections at baseURL.
// A TCP connect is deliberate: it asks the one question that matters — will a
// request reach it — without assuming which paths this engine serves.
func engineAnswers(ctx context.Context, baseURL string) bool {
	addr := hostPortOf(baseURL)
	if addr == "" {
		return false
	}
	dialer := net.Dialer{Timeout: engineDialTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// isAlreadyRunning reports whether a start was refused because an engine is
// already running — the daemon's conflict, which is a race rather than a fault.
func isAlreadyRunning(err error) bool {
	var he *httpError
	if errors.As(err, &he) && he.status == http.StatusConflict {
		return true
	}
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "already running")
}

// WouldWake names the node Wake would try first, without starting anything. It
// is what lets a routing decision be explained before an agent depends on it —
// and the reason the ordering lives in one place rather than being described
// twice.
func (c *Config) WouldWake(results []NodeResult, cfgFor ConfigFor) (NodeConfig, bool) {
	cands := wakeable(c.candidates(results), newConfigResolver(cfgFor))
	if len(cands) == 0 {
		return NodeConfig{}, false
	}
	return cands[0].entry, true
}
