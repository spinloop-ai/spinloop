package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/spinloop-ai/spinloop/internal/orchestrator"
)

// workLogsInterval is how often a follow asks for more output, and checks
// the item's own state. A variable so tests need not wait on it.
var workLogsInterval = 1 * time.Second

// workLogsCmd builds `work logs`.
func workLogsCmd() *cobra.Command {
	var base, apiToken, apiTokenFile string
	var follow bool
	c := &cobra.Command{
		Use:   "logs <id>",
		Short: "print an item's kept agent output",
		Long: `prints an item's kept agent output, through the work list API the
orchestrator serves. An item with no output yet is printed as empty, not a
fault, and an id the run does not carry is refused, naming it, the way the
API states it.

-f/--follow polls for new output and prints it as it arrives, the way
tail -f does: it keeps polling through backlog and running — an item
named before, or just as, it starts is still followed — and stops once
the item is done or failed, or the operator interrupts.`,
		Args:          cobra.ExactArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(_ *cobra.Command, args []string) error {
			id := args[0]
			b, token, err := workTarget("work logs", base, apiToken, apiTokenFile)
			if err != nil {
				return err
			}
			if follow {
				return followWorkLogs(b, token, id)
			}
			return runWorkLogsOnce(b, token, id, os.Stdout)
		},
	}
	fs := c.Flags()
	workAPIFlags(fs, &base, &apiToken, &apiTokenFile)
	fs.BoolVarP(&follow, "follow", "f", false, "keep printing new output as it arrives")
	return c
}

// runWorkLogsOnce prints an item's kept output once.
func runWorkLogsOnce(base, token, id string, w io.Writer) error {
	log, err := workLogFetch(base, token, id)
	if err != nil {
		return err
	}
	fmt.Fprint(w, log)
	return nil
}

// workLogReply is the work list API's own answer shape for an item's log:
// carrying no field at all where there is none yet, the way handleLog
// answers `"log": null`.
type workLogReply struct {
	Log *string `json:"log"`
}

// workLogFetch reads an item's kept output through the work list API. An
// item with none yet answers "", not an error.
func workLogFetch(base, token, id string) (string, error) {
	data, err := workRequest(base, token, http.MethodGet, "/v1/items/"+url.PathEscape(id)+"/log", nil)
	if err != nil {
		return "", err
	}
	var out workLogReply
	if err := json.Unmarshal(data, &out); err != nil {
		return "", fmt.Errorf("reading item %q's log from the work list API at %s: %w", id, base, err)
	}
	if out.Log == nil {
		return "", nil
	}
	return *out.Log, nil
}

// workItemState reads one item's state from the work list API's own list:
// found and its state where the list still carries the id, found false
// where it has dropped out of the list entirely — a removal.
func workItemState(base, token, id string) (state string, found bool, err error) {
	data, err := workRequest(base, token, http.MethodGet, "/v1/items", nil)
	if err != nil {
		return "", false, err
	}
	var out struct {
		Data []orchestrator.ItemView `json:"data"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", false, fmt.Errorf("reading the work list from the work list API at %s: %w", base, err)
	}
	for _, v := range out.Data {
		if v.ID == id {
			return v.State, true, nil
		}
	}
	return "", false, nil
}

// followWorkLogs polls an item's log and its own state until it ends, or
// the operator interrupts.
func followWorkLogs(base, token, id string) error {
	return followUntilInterrupted(func(ctx context.Context) error {
		return followWorkLogsLoop(ctx, base, token, id, os.Stdout)
	})
}

// followWorkLogsLoop is the polling itself, with the interrupt wiring left
// to its caller so it can be driven directly. Each tick fetches the log,
// prints what is new, then checks the item's own state: the loop keeps
// going while it is backlog or running, and ends — after that tick's log
// poll, so nothing written right at the end is missed — once the state is
// done or failed, or the item has dropped out of the list entirely (a
// removal mid-follow), which ends the follow the same clean way.
func followWorkLogsLoop(ctx context.Context, base, token, id string, w io.Writer) error {
	var last string
	for {
		log, err := workLogFetch(base, token, id)
		if err != nil {
			if workItemGone(err) {
				return nil
			}
			return err
		}
		last = printWorkLogSuffix(w, log, last)

		state, found, err := workItemState(base, token, id)
		if err != nil {
			return err
		}
		if !found || (state != orchestrator.StateBacklog && state != orchestrator.StateRunning) {
			return nil
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(workLogsInterval):
		}
	}
}

// printWorkLogSuffix prints whatever of log is new since last, and returns
// log as the new "last" for the next call: where log still starts with
// last, only the suffix beyond it is new; otherwise — the kept output
// changed underneath, which does not happen in an item's own lifetime, but
// is not worth crashing on — log is printed in full, as if nothing had
// been printed yet.
func printWorkLogSuffix(w io.Writer, log, last string) string {
	if strings.HasPrefix(log, last) {
		fmt.Fprint(w, log[len(last):])
	} else {
		fmt.Fprint(w, log)
	}
	return log
}

// workItemGone reports whether err is the work list API's own "id not
// carried" refusal (a 404) — what a removal mid-follow leaves behind. A
// follow reads that as a clean end, not a failure.
func workItemGone(err error) bool {
	var apiErr *workAPIErr
	return errors.As(err, &apiErr) && apiErr.status == http.StatusNotFound
}
