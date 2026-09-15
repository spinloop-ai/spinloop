// The `work` command family: the orchestrator's work list — the backlog the
// orchestrator works — worked from the shell as a client of the work list API
// the orchestrator serves: add an item, read the work, stop a running item,
// remove an item. The commands name the API's address and present its token;
// the run's view of the items is the source of truth, and a refusal reads the
// way the API states it.
package main

import (
	"bytes"
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
	"github.com/spf13/pflag"
	"golang.org/x/term"

	"github.com/spinloop-ai/spinloop/internal/orchestrator"
)

// workRequestBound bounds one call to the work list API. The API's abort
// blocks for the run's stop grace, so the bound outlives a whole stop, and a
// run that never answers cannot hang the command forever.
var workRequestBound = 30 * time.Second

func cmdWork(args []string) error { return execCmd(workCmd(), args) }

// workCmd builds the `work` group.
func workCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "work",
		Short: "work the orchestrator's work list",
		Long: `works the orchestrator's work list — the backlog it works — from
the shell, as a client of the work list API the orchestrator serves: add an
item, read the work, stop a running item, remove an item.

Each subcommand takes --url, the API's base address, and presents the API's
token — from --api-token, else --api-token-file, else the SPINLOOP_API_TOKEN
environment — as a bearer on every call it makes. The run's view of the items
is the source of truth: the commands call the API and report its answer, and
a refusal reads the way the API states it. A command that names no --url
fails before it calls the API, naming the flag.`,
	}
	c.AddCommand(
		workAddCmd(),
		workListCmd(),
		workAbortCmd(),
		workRemoveCmd(),
	)
	return c
}

// workAPIFlags adds the flags that name the work list API — its address and
// its token — to a subcommand's flag set, the way the fleet subcommands each
// carry --fleet: the same flag on every subcommand, not a persistent flag
// the group owns.
func workAPIFlags(fs *pflag.FlagSet, base, apiToken, apiTokenFile *string) {
	fs.StringVar(base, "url", "", "the work list API's base address")
	fs.StringVar(apiToken, "api-token", "", "the work list API's bearer token")
	fs.StringVar(apiTokenFile, "api-token-file", "", "the file the work list API's bearer token stands in")
}

// workTarget resolves a work command's --url and token flags into the API it
// calls: the address, required and named where absent, and the token,
// resolved the way the daemon resolves its own.
func workTarget(use, base, apiToken, apiTokenFile string) (string, string, error) {
	if base == "" {
		return "", "", fmt.Errorf("%s needs the work list API's address: --url", use)
	}
	token, err := daemonToken(apiToken, apiTokenFile)
	if err != nil {
		return "", "", err
	}
	return base, token, nil
}

// workRequest makes one call to the work list API and checks its answer: the
// token as a bearer where there is one, the body as JSON where there is one,
// and a non-2xx an error carrying the API's message, so a refusal reads the
// way the API states it.
func workRequest(base, token, method, path string, body any) ([]byte, error) {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(data)
	}
	ctx, cancel := context.WithTimeout(context.Background(), workRequestBound)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimSuffix(base, "/")+path, reader)
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling the work list API at %s: %w", base, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("reading the answer from the work list API at %s: %w", base, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, workAPIError(base, resp.StatusCode, data)
	}
	return data, nil
}

// workAPIError is a non-2xx answer from the work list API, as the error the
// command reports: the API's message where it states one — the refusal reads
// the way the API states it — the status and the address where it does not.
func workAPIError(base string, status int, data []byte) error {
	var out struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(data, &out); err == nil && out.Error.Message != "" {
		return errors.New(out.Error.Message)
	}
	return fmt.Errorf("the work list API at %s answered %d", base, status)
}

// workAddBody is one item as the work list API takes it: the fields the items
// file takes, the API's JSON form.
type workAddBody struct {
	ID           string   `json:"id"`
	Instructions string   `json:"instructions"`
	Dir          string   `json:"dir"`
	Tags         []string `json:"tags"`
	Priority     int      `json:"priority"`
}

// workAddCmd builds `work add`.
func workAddCmd() *cobra.Command {
	var base, apiToken, apiTokenFile string
	var id, instructions, dir string
	var tags []string
	var priority int
	c := &cobra.Command{
		Use:   "add",
		Short: "add an item to the orchestrator's work list",
		Long: `adds an item to the orchestrator's work list: the item's fields
come from the flags, sent to the work list API the orchestrator serves, and
the API applies the items file's validation on them — the instructions and
the working directory present, the tags well-formed key=value pairs, no key
named twice — and its own refusals.

An id the file already carries is refused, naming it, and an id the state
beside the file has recorded done or failed is refused too, naming the
record — an ended item is not worked again under its own id. The API answers
once the item is in the file, and the command reports its answer: a refusal
reads the way the API states it.`,
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(_ *cobra.Command, _ []string) error {
			b, token, err := workTarget("work add", base, apiToken, apiTokenFile)
			if err != nil {
				return err
			}
			body := workAddBody{
				ID: id, Instructions: instructions, Dir: dir,
				Tags: tags, Priority: priority,
			}
			if _, err := workRequest(b, token, http.MethodPost, "/v1/items", body); err != nil {
				return err
			}
			fmt.Printf("item %q added\n", id)
			return nil
		},
	}
	fs := c.Flags()
	workAPIFlags(fs, &base, &apiToken, &apiTokenFile)
	fs.StringVar(&id, "id", "", "the item's id")
	fs.StringVar(&instructions, "instructions", "", "the instructions the item's agent is given")
	fs.StringVar(&dir, "dir", "", "the directory the agent works in")
	fs.StringArrayVar(&tags, "tag", nil, "a tag the item carries, key=value; repeatable")
	fs.IntVar(&priority, "priority", 0, "the item's priority, higher first")
	return c
}

// workListCmd builds `work list`.
func workListCmd() *cobra.Command {
	var base, apiToken, apiTokenFile string
	c := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "report the orchestrator's work list",
		Long: `reports every item in the orchestrator's work list with its record,
in the file's order: the item's id, its state — backlog, running, done or
failed — the node a running item runs on, and when it started and ended, one
plain line per item, a dash where a value is absent.

The list reads the work list API the orchestrator serves: the run's view of
the items, the source of truth, an item with no record backlog. The state
colour is drawn only where there is a terminal to draw it on.`,
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			b, token, err := workTarget("work list", base, apiToken, apiTokenFile)
			if err != nil {
				return err
			}
			data, err := workRequest(b, token, http.MethodGet, "/v1/items", nil)
			if err != nil {
				return err
			}
			var out struct {
				Data []orchestrator.ItemView `json:"data"`
			}
			if err := json.Unmarshal(data, &out); err != nil {
				return fmt.Errorf("reading the work list from the work list API at %s: %w", b, err)
			}
			tty := term.IsTerminal(int(os.Stdout.Fd()))
			w := cmd.OutOrStdout()
			for _, v := range out.Data {
				fmt.Fprintln(w, workListLine(v, tty))
			}
			return nil
		},
	}
	fs := c.Flags()
	workAPIFlags(fs, &base, &apiToken, &apiTokenFile)
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
	var base, apiToken, apiTokenFile string
	c := &cobra.Command{
		Use:   "abort <id>",
		Short: "stop a running item and put it back in the backlog",
		Long: `stops a running item, through the work list API the orchestrator
serves: the item's agent stopped the way a clean interrupt stops it, and its
record removed — the item back in the backlog, and the run's next pass
admits it again.

Only a running item can be aborted: an item the run records backlog, done or
failed is refused, naming the item and its state, and an id the file does not
carry is refused, naming it. The API answers once the item is stopped, and
the command reports its answer: a refusal reads the way the API states it.`,
		Args:          cobra.ExactArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(_ *cobra.Command, args []string) error {
			id := args[0]
			b, token, err := workTarget("work abort", base, apiToken, apiTokenFile)
			if err != nil {
				return err
			}
			if _, err := workRequest(b, token, http.MethodPost, "/v1/items/"+url.PathEscape(id)+"/abort", nil); err != nil {
				return err
			}
			fmt.Printf("item %q stopped: it is back in the backlog\n", id)
			return nil
		},
	}
	fs := c.Flags()
	workAPIFlags(fs, &base, &apiToken, &apiTokenFile)
	return c
}

// workRemoveCmd builds `work remove`.
func workRemoveCmd() *cobra.Command {
	var base, apiToken, apiTokenFile string
	c := &cobra.Command{
		Use:   "remove <id>",
		Short: "remove an item from the orchestrator's work list",
		Long: `removes an item, through the work list API the orchestrator
serves: the item out of the work list — the items file, its record, and its
kept output — the file a valid items file after the removal.

A running item cannot be removed: the refusal names it and the abort that
goes first, and an id the file does not carry is refused, naming it. The API
answers once the item is out, and the command reports its answer: a refusal
reads the way the API states it.`,
		Args:          cobra.ExactArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(_ *cobra.Command, args []string) error {
			id := args[0]
			b, token, err := workTarget("work remove", base, apiToken, apiTokenFile)
			if err != nil {
				return err
			}
			if _, err := workRequest(b, token, http.MethodDelete, "/v1/items/"+url.PathEscape(id), nil); err != nil {
				return err
			}
			fmt.Printf("item %q removed\n", id)
			return nil
		},
	}
	fs := c.Flags()
	workAPIFlags(fs, &base, &apiToken, &apiTokenFile)
	return c
}
