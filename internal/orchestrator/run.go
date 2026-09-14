package orchestrator

import (
	"context"
	"log/slog"
	"time"
)

// stopGrace is how long a clean interrupt gives a stopped agent to end
// before it is killed hard. A variable so tests do not wait.
var stopGrace = 5 * time.Second

// Config is what the loop runs on: the gateway it names in its failures,
// the items file it works, the gateway it reads the fleet from, and the
// dispatcher that launches admitted items.
type Config struct {
	Gateway    string
	ItemsPath  string
	Topologist Topologist
	Dispatcher *Dispatcher
	// Tick is how often the loop re-reads the topology and the items file;
	// zero takes the default.
	Tick time.Duration
	// Log receives the loop's lines; nil discards them.
	Log *slog.Logger
	// WorkList is the shared work list the run acts on; nil opens one from
	// ItemsPath — the store beside it, the items, the records — and Run
	// closes it as it ends. Where the command serves the work list API, it
	// passes the one its handler acts on, so the run and the API share it,
	// and the command closes it, after the server has gone down.
	WorkList *WorkList
}

// result is one finished child, reported by its wait.
type result struct {
	id  string
	err error
}

// Run works the items file's backlog against the fleet the gateway reports,
// until the context ends. It is the whole lifecycle: the state it keeps
// beside the items file, the lock that keeps a second orchestrator off the
// file, admission under the fleet's declared limits, and the clean
// interrupt that stops its agents and re-queues their items. A gateway that
// stops answering ends the run, naming the gateway; the items are safe in
// the file and the state, and a restarted run picks them up.
func Run(ctx context.Context, cfg Config) error {
	log := cfg.Log
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	wl := cfg.WorkList
	if wl == nil {
		store, err := OpenStore(cfg.ItemsPath)
		if err != nil {
			return err
		}
		wl, err = NewWorkList(cfg.ItemsPath, store, cfg.Dispatcher, cfg.Gateway, log)
		if err != nil {
			return err
		}
		defer wl.Close()
	}
	return wl.run(ctx, cfg)
}
