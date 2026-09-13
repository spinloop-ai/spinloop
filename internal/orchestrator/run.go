package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"os"
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
	tick := cfg.Tick
	if tick <= 0 {
		tick = tickInterval
	}

	store, err := OpenStore(cfg.ItemsPath)
	if err != nil {
		return err
	}
	defer store.Close()
	sf, err := store.load()
	if err != nil {
		return err
	}

	topo, err := cfg.Topologist.Topology(ctx)
	if err != nil {
		if ctx.Err() != nil {
			// The interrupt arrived before the first pass: nothing is in
			// flight, so the clean end is to end.
			return nil
		}
		return fmt.Errorf("the gateway %s did not answer: %v", cfg.Gateway, err)
	}
	items, err := LoadItems(cfg.ItemsPath)
	if err != nil {
		return err
	}
	fi, err := os.Stat(cfg.ItemsPath)
	if err != nil {
		return fmt.Errorf("reading the work items file: %v", err)
	}
	lastMod, lastSize := fi.ModTime(), fi.Size()

	type flight struct {
		item  Item
		node  Node
		child Child
	}
	inflight := map[string]*flight{}
	results := make(chan result)

	save := func() {
		if err := store.save(sf); err != nil {
			log.Error("saving the state", slog.String("error", err.Error()))
		}
	}

	// reap takes every finished child the waits have reported: done where
	// the agent ended cleanly, failed where it did not, its slot freed.
	reap := func() {
		for {
			select {
			case r := <-results:
				fl, ok := inflight[r.id]
				if !ok {
					continue
				}
				delete(inflight, r.id)
				st := ItemState{Node: fl.node.Name, EndedAt: time.Now().UTC().Format(time.RFC3339)}
				if r.err == nil {
					st.State = StateDone
				} else {
					st.State = StateFailed
					st.Why = r.err.Error()
				}
				sf.Items[r.id] = st
				log.Info("item ended",
					slog.String("item", r.id),
					slog.String("node", fl.node.Name),
					slog.String("state", st.State))
			default:
				return
			}
		}
	}

	// shutdown is the clean interrupt: stop the agents, put their items
	// back in the backlog — no work lost, none run twice — and give them
	// the grace before the hard end.
	shutdown := func() error {
		for id, fl := range inflight {
			fl.child.Stop()
			delete(sf.Items, id)
		}
		save()
		deadline := time.After(stopGrace)
		for len(inflight) > 0 {
			select {
			case r := <-results:
				delete(inflight, r.id)
			case <-deadline:
				for _, fl := range inflight {
					fl.child.Kill()
				}
				for len(inflight) > 0 {
					select {
					case r := <-results:
						delete(inflight, r.id)
					}
				}
			}
		}
		return nil
	}

	for {
		reap()

		topo, err = cfg.Topologist.Topology(ctx)
		if err != nil {
			if ctx.Err() != nil {
				// The read failed because the interrupt cancelled it, not
				// because the gateway went away: take the clean end.
				return shutdown()
			}
			save()
			return fmt.Errorf("the gateway %s stopped answering: %v", cfg.Gateway, err)
		}

		// A changed items file is re-read: new items enter the backlog,
		// and an item dropped from it keeps its recorded end.
		if fi, err := os.Stat(cfg.ItemsPath); err == nil {
			if !fi.ModTime().Equal(lastMod) || fi.Size() != lastSize {
				items, err = LoadItems(cfg.ItemsPath)
				if err != nil {
					save()
					return err
				}
				lastMod, lastSize = fi.ModTime(), fi.Size()
			}
		}

		for _, item := range sortedForAdmission(items) {
			if st, recorded := sf.Items[item.ID]; recorded && (st.State == StateDone || st.State == StateFailed) {
				continue // a finished end stands: no retry on its own
			}
			if _, running := inflight[item.ID]; running {
				continue
			}
			nodes := Match(item, topo)
			if len(nodes) == 0 {
				continue // waiting is not failing: the fleet may change
			}
			// Rebuilt per item: an item admitted earlier this pass is in
			// flight and counts against the limits for the rest of it.
			inflightItems := make([]Item, 0, len(inflight))
			for _, fl := range inflight {
				inflightItems = append(inflightItems, fl.item)
			}
			if !Admits(topo, inflightItems, item) {
				continue // the limits hold the line until a slot frees
			}
			node := nodes[0]
			child, err := cfg.Dispatcher.Launch(item, node, store.LogPath(item.ID))
			if err != nil {
				sf.Items[item.ID] = ItemState{
					State: StateFailed, Why: err.Error(), Node: node.Name,
					EndedAt: time.Now().UTC().Format(time.RFC3339),
				}
				log.Info("item failed to launch",
					slog.String("item", item.ID),
					slog.String("error", err.Error()))
				continue // the rest of the backlog goes on
			}
			inflight[item.ID] = &flight{item: item, node: node, child: child}
			sf.Items[item.ID] = ItemState{
				State: StateRunning, Node: node.Name,
				StartedAt: time.Now().UTC().Format(time.RFC3339),
			}
			log.Info("item launched",
				slog.String("item", item.ID),
				slog.String("node", node.Name))
			go func(id string, c Child) {
				results <- result{id: id, err: c.Wait()}
			}(item.ID, child)
		}

		save()

		select {
		case <-ctx.Done():
			return shutdown()
		case <-time.After(tick):
		}
	}
}
