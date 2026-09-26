// `work board`: the kanban view of a running orchestrator's work list.
// This is the command layer — flags, the terminal check, and the program;
// the model and the renderers live in work_board_model.go and
// work_board_render.go, so the screen logic is tested without a command
// and the command without a screen.

package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// cmdWorkBoard is the seam the suite calls, the family's own.
func cmdWorkBoard(args []string) error { return execCmd(workBoardCmd(), args) }

func workBoardCmd() *cobra.Command {
	var base, apiToken, apiTokenFile string
	c := &cobra.Command{
		Use:   "board",
		Short: "watch the work list on a kanban board",
		Long: `watches the orchestrator's work list as a live kanban board — a
column per state (Backlog, Running, Done, Failed), a card per item —
re-read from the work list API on a cadence, so cards move as the run
works.

The arrow keys move the selection, enter opens the item's detail (its full
instructions, its tags and record, and its kept output tailed live), esc
closes it. n opens a form that adds an item through the API — the same add
` + "`work add`" + ` sends — a stops a running item, x removes one that is not,
after asking. r reads again at once; q or Ctrl+C leaves. Keys are offered
only where they would do something.

Like every work command the board names the API with --url and presents
its token; the run's view of the items is the source of truth, and a
refusal reads the way the API states it. The board needs a terminal; to
report the same work into a pipe, use spinloop work list instead.`,
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(_ *cobra.Command, _ []string) error {
			b, token, err := workTarget("work board", base, apiToken, apiTokenFile)
			if err != nil {
				return err
			}
			return runWorkBoard(b, token)
		},
	}
	fs := c.Flags()
	workAPIFlags(fs, &base, &apiToken, &apiTokenFile)
	c.ValidArgsFunction = noPositionals
	return c
}

// runWorkBoard opens the view. The terminal check comes after the target
// resolves — a missing --url names the flag wherever the board is asked
// for — and before anything is drawn: a piped invocation never
// half-enters the view.
func runWorkBoard(base, token string) error {
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return fmt.Errorf("the work board needs an interactive terminal — " +
			"report the work into a pipe with spinloop work list instead")
	}
	return runWorkBoardProgram(newWorkBoardModel(base, token))
}

// runWorkBoardProgram runs the view on the alternate screen, the model
// held by pointer so the first round's answers survive — Bubble Tea
// restores the terminal on the way out, whatever key got here.
func runWorkBoardProgram(m *workBoardModel) error {
	prog := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := prog.Run(); err != nil {
		return fmt.Errorf("work board: %w", err)
	}
	return nil
}
