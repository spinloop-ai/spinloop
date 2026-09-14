// The `work` command family: the work items file — the backlog the
// orchestrator works — and the state the orchestrator keeps beside it,
// worked from the shell: add an item, read the backlog's state, stop a
// running item, remove an item. The commands work the files, and they work
// whether or not the orchestrator is running: a running orchestrator picks
// the change up on its next pass, the file beside it the whole hand-off.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/spinloop-ai/spinloop/internal/orchestrator"
)

// workWaitBound is how long abort and remove wait for a running orchestrator
// to take the change in, and workWaitTick the cadence they poll at.
var (
	workWaitBound = 30 * time.Second
	workWaitTick  = 200 * time.Millisecond
)

func cmdWork(args []string) error { return execCmd(workCmd(), args) }

// workCmd builds the `work` group.
func workCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "work",
		Short: "work the work items file",
		Long: `works the work items file — the backlog the orchestrator works —
and the state the orchestrator keeps beside it, from the shell: add an item,
read the backlog's state, stop a running item, remove an item.

Each subcommand takes --items, defaulting to ./work.yaml in the working
directory: the same file the orchestrator reads. The commands work the file
and the state beside it, and they work whether or not the orchestrator is
running: a running orchestrator sees the change on its next pass, and the
file beside it is the whole hand-off.`,
	}
	c.AddCommand(
		workAddCmd(),
		workListCmd(),
		workAbortCmd(),
		workRemoveCmd(),
	)
	return c
}

// loadWorkItems reads the items file the way a command needs it: the file's
// items, or an error naming the file where it is missing or not a list of
// items.
func loadWorkItems(itemsPath string) ([]orchestrator.Item, error) {
	items, err := orchestrator.LoadItems(itemsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf(
				"no work items file at %s: create it, or point --items at the file the orchestrator works",
				itemsPath)
		}
		return nil, err
	}
	return items, nil
}

// workAddCmd builds `work add`.
func workAddCmd() *cobra.Command {
	var itemsPath, id, instructions, dir string
	var tags []string
	var priority int
	c := &cobra.Command{
		Use:   "add",
		Short: "add an item to the work items file",
		Long: `appends an item to the work items file: the item's fields come
from the flags, the file a valid items file after the append, with the
validation the orchestrator applies — the item's instructions and working
directory present, and its tags well-formed key=value pairs, no key named
twice.

An id the file already carries is refused, naming it, and an id the state
beside the file has recorded done or failed is refused too, naming the
record — an ended item is not worked again under its own id. A missing file
is created, carrying the one item, and a running orchestrator picks the item
up on its next pass.`,
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(_ *cobra.Command, _ []string) error {
			if id == "" {
				return fmt.Errorf("work add needs the item's id: --id")
			}
			if instructions == "" {
				return fmt.Errorf("work add needs the item's instructions: --instructions")
			}
			if dir == "" {
				return fmt.Errorf("work add needs the item's working directory: --dir")
			}
			item := orchestrator.Item{
				ID: id, Instructions: instructions, Dir: dir,
				Tags: tags, Priority: priority,
			}
			if err := orchestrator.ValidateItem(item); err != nil {
				return err
			}
			items, err := orchestrator.LoadItems(itemsPath)
			if err != nil {
				if !os.IsNotExist(err) {
					return err
				}
				items = nil // a missing file is created, carrying the one item
			}
			for _, it := range items {
				if it.ID == item.ID {
					return fmt.Errorf("the items file %s already carries an item with id %q",
						itemsPath, item.ID)
				}
			}
			records, err := orchestrator.LoadStateFile(itemsPath)
			if err != nil {
				return err
			}
			if st, recorded := records[item.ID]; recorded &&
				(st.State == orchestrator.StateDone || st.State == orchestrator.StateFailed) {
				return fmt.Errorf("item %q is recorded %s in the state beside %s: an ended item is not added again",
					item.ID, st.State, itemsPath)
			}
			if err := orchestrator.SaveItems(itemsPath, append(items, item)); err != nil {
				return err
			}
			fmt.Printf("item %q added to %s\n", item.ID, itemsPath)
			return nil
		},
	}
	fs := c.Flags()
	fs.StringVar(&itemsPath, "items", "./work.yaml", "the work items file")
	fs.StringVar(&id, "id", "", "the item's id")
	fs.StringVar(&instructions, "instructions", "", "the instructions the item's agent is given")
	fs.StringVar(&dir, "dir", "", "the directory the agent works in")
	fs.StringArrayVar(&tags, "tag", nil, "a tag the item carries, key=value; repeatable")
	fs.IntVar(&priority, "priority", 0, "the item's priority, higher first")
	return c
}

// workListCmd builds `work list`.
func workListCmd() *cobra.Command {
	var itemsPath string
	c := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "report the work items file's items and their state",
		Long: `reports every item in the work items file with its record from the
state the orchestrator keeps beside it, in the file's order: the item's id,
its state — backlog, running, done or failed — the node a running item runs
on, and when it started and ended, one plain line per item, a dash where a
value is absent.

An item with no record in the state is backlog, and no state file at all —
the orchestrator has never run the file — means everything is backlog. The
list reads the file and the state and takes no lock, so it works whether or
not the orchestrator is running; the state colour is drawn only where there
is a terminal to draw it on.`,
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			items, err := loadWorkItems(itemsPath)
			if err != nil {
				return err
			}
			records, err := orchestrator.LoadStateFile(itemsPath)
			if err != nil {
				return err
			}
			tty := term.IsTerminal(int(os.Stdout.Fd()))
			out := cmd.OutOrStdout()
			for _, v := range orchestrator.Join(items, records) {
				fmt.Fprintln(out, workListLine(v, tty))
			}
			return nil
		},
	}
	fs := c.Flags()
	fs.StringVar(&itemsPath, "items", "./work.yaml", "the work items file")
	return c
}

// workListLine is one item's line: the columns a program can split, the
// state colour where the output is a terminal and plain elsewhere.
func workListLine(v orchestrator.ItemView, tty bool) string {
	node := v.Node
	if node == "" {
		node = "-"
	}
	started := v.StartedAt
	if started == "" {
		started = "-"
	}
	ended := v.EndedAt
	if ended == "" {
		ended = "-"
	}
	state := v.State
	if tty {
		switch v.State {
		case orchestrator.StateDone:
			state = ansiGreen + v.State + ansiReset
		case orchestrator.StateFailed:
			state = ansiRed + v.State + ansiReset
		case orchestrator.StateRunning:
			state = ansiYellow + v.State + ansiReset
		}
	}
	return v.ID + "\t" + state + "\t" + node + "\t" + started + "\t" + ended
}

// workAbortCmd builds `work abort`.
func workAbortCmd() *cobra.Command {
	var itemsPath string
	c := &cobra.Command{
		Use:   "abort <id>",
		Short: "stop a running item and put it back in the backlog",
		Long: `stops a running item: the marker beside the items file the run
takes up on its pass, the item's agent stopped the way a clean interrupt
stops it, and its record removed from the state — the item is back in the
backlog, and the run's next pass admits it again.

Only a running item can be aborted: an id the file does not carry is
refused, naming it, and an item the state records backlog, done or failed is
refused too, naming the item and its state. Where no orchestrator is running
the file there is nothing running, and the command says so — a stale running
record is for the next start to record failed, not for abort to clear.`,
		Args:          cobra.ExactArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(_ *cobra.Command, args []string) error {
			id := args[0]
			items, err := loadWorkItems(itemsPath)
			if err != nil {
				return err
			}
			carries := false
			for _, it := range items {
				if it.ID == id {
					carries = true
					break
				}
			}
			if !carries {
				return fmt.Errorf("the items file %s carries no item with id %q", itemsPath, id)
			}
			records, err := orchestrator.LoadStateFile(itemsPath)
			if err != nil {
				return err
			}
			st, recorded := records[id]
			if !recorded {
				return fmt.Errorf("item %q is not running: it is %s", id, orchestrator.StateBacklog)
			}
			if st.State != orchestrator.StateRunning {
				return fmt.Errorf("item %q is not running: it is %s", id, st.State)
			}
			pid, live, err := orchestrator.LockStatus(itemsPath)
			if err != nil {
				return err
			}
			if !live {
				return fmt.Errorf(
					"no orchestrator is running %s: nothing is running, and the record is left for the next start to record failed",
					itemsPath)
			}
			if err := orchestrator.RequestAbort(itemsPath, id); err != nil {
				return err
			}
			return awaitAbort(itemsPath, id, pid)
		},
	}
	fs := c.Flags()
	fs.StringVar(&itemsPath, "items", "./work.yaml", "the work items file")
	return c
}

// awaitAbort waits for the run to take the marker up — the agent stopped,
// the item out of the state — and reports the item's state from the state
// the wait leaves: the bound runs out first, the run dies mid-wait, and the
// outcome where it is in.
func awaitAbort(itemsPath, id string, holder int) error {
	deadline := time.Now().Add(workWaitBound)
	for {
		pending, err := orchestrator.AbortsPending(itemsPath)
		if err != nil {
			return err
		}
		standing := false
		for _, p := range pending {
			if p == id {
				standing = true
				break
			}
		}
		if !standing {
			records, err := orchestrator.LoadStateFile(itemsPath)
			if err != nil {
				return err
			}
			if st, ok := records[id]; ok {
				fmt.Printf("item %q is %s\n", id, st.State)
			} else {
				fmt.Printf("item %q is stopped: it is back in the backlog\n", id)
			}
			return nil
		}
		if _, live, err := orchestrator.LockStatus(itemsPath); err != nil {
			return err
		} else if !live {
			return fmt.Errorf(
				"the orchestrator (pid %d) stopped while the abort of %q was pending: the marker stands beside %s, and the record is left for the next start to record failed",
				holder, id, itemsPath)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf(
				"the abort of %q is still pending after %v: the orchestrator (pid %d) has not taken the marker up beside %s",
				id, workWaitBound, holder, itemsPath)
		}
		time.Sleep(workWaitTick)
	}
}

// workRemoveCmd builds `work remove`.
func workRemoveCmd() *cobra.Command {
	var itemsPath string
	c := &cobra.Command{
		Use:   "remove <id>",
		Short: "remove an item from the work items file, its state, and its log",
		Long: `removes an item: the item out of the work items file, its record
out of the state beside it, and its kept output from the logs beside it —
the file a valid items file after the removal.

A running item cannot be removed: the refusal names it and the abort that
goes first. An id the file does not carry is refused, naming it. Backlog,
done and failed items are removed alike, and a running orchestrator picks
the change up on its next pass: the item out of its backlog, and the record
dropped with it.`,
		Args:          cobra.ExactArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(_ *cobra.Command, args []string) error {
			id := args[0]
			items, err := loadWorkItems(itemsPath)
			if err != nil {
				return err
			}
			kept := make([]orchestrator.Item, 0, len(items))
			found := false
			for _, it := range items {
				if it.ID == id {
					found = true
					continue
				}
				kept = append(kept, it)
			}
			if !found {
				return fmt.Errorf("the items file %s carries no item with id %q", itemsPath, id)
			}
			records, err := orchestrator.LoadStateFile(itemsPath)
			if err != nil {
				return err
			}
			if st, ok := records[id]; ok && st.State == orchestrator.StateRunning {
				return fmt.Errorf("item %q is running: abort it first, then remove it", id)
			}
			if err := orchestrator.SaveItems(itemsPath, kept); err != nil {
				return err
			}
			os.Remove(orchestrator.LogPathFor(itemsPath, id))
			if _, ok := records[id]; ok {
				delete(records, id)
				if err := orchestrator.SaveStateFile(itemsPath, records); err != nil {
					return err
				}
			}
			_, live, err := orchestrator.LockStatus(itemsPath)
			if err != nil {
				return err
			}
			if live {
				if err := awaitRecordGone(itemsPath, id); err != nil {
					return err
				}
			}
			fmt.Printf("item %q removed from %s\n", id, itemsPath)
			return nil
		},
	}
	fs := c.Flags()
	fs.StringVar(&itemsPath, "items", "./work.yaml", "the work items file")
	return c
}

// awaitRecordGone waits for the run's pass to drop the record — the file no
// longer carries the item, and the state must not either — and says nothing
// of its own: the remove's report goes on when the state agrees. Where the
// run dies mid-wait, the command's own write is the final word.
func awaitRecordGone(itemsPath, id string) error {
	deadline := time.Now().Add(workWaitBound)
	for {
		records, err := orchestrator.LoadStateFile(itemsPath)
		if err != nil {
			return err
		}
		if _, recorded := records[id]; !recorded {
			return nil
		}
		if _, live, err := orchestrator.LockStatus(itemsPath); err != nil {
			return err
		} else if !live {
			delete(records, id)
			if err := orchestrator.SaveStateFile(itemsPath, records); err != nil {
				return err
			}
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf(
				"the state beside %s still records %q after %v: the orchestrator has not dropped it",
				itemsPath, id, workWaitBound)
		}
		time.Sleep(workWaitTick)
	}
}
