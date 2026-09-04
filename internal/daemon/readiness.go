package daemon

import "sync"

// readinessCheckedRunners lists the runners with a known health-check
// convention: GET /health, treating 200 or 401 (a gated engine's expected
// answer to an unauthenticated probe) as ready. A runner not listed here has
// no established convention (omlx, currently) and is left unchecked — the
// readiness field stays absent for it, rather than reporting a guess.
var readinessCheckedRunners = map[string]bool{
	"llamacpp": true,
	"vllm":     true,
	"mtplx":    true,
}

// readiness is the daemon's record of whether the running engine last
// answered its health check. Mirrors engineSample's shape: guarded, holds
// one reading, forgotten on each new start so a previous engine's answer is
// never reported against the one that replaced it.
type readiness struct {
	mu    sync.Mutex
	ready bool
	have  bool
}

// record stores one reading.
func (r *readiness) record(ready bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ready, r.have = ready, true
}

// read returns the last reading, and whether one has landed at all. Both are
// zero before the first check, or when the running engine's runner is not in
// readinessCheckedRunners.
func (r *readiness) read() (ready, have bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ready, r.have
}

// forget drops the reading, so a stopped or restarted engine's readiness is
// not reported against the next one.
func (r *readiness) forget() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ready, r.have = false, false
}
