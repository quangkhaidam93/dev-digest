package tui

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/quangkhaidam93/dev-digest/internal/deliver"
	"github.com/quangkhaidam93/dev-digest/internal/pipeline"
	"github.com/quangkhaidam93/dev-digest/internal/store"
)

// runFinishedMsg carries the result of a run back to the event loop.
type runFinishedMsg struct {
	delivered bool   // true if this was a real send (not a preview)
	output    string // rendered digest markdown
	summary   string // per-channel delivery summary (send mode)
	newItems  int
	failed    bool // a delivery failed (send mode)
	err       error
}

// runCmd runs the pipeline from inside the TUI. When deliver is false it's a
// preview (no channels, no state change); when true it actually fetches,
// summarizes, delivers, and records state — i.e. sends the email/webhook.
func (m model) runCmd(deliver bool) tea.Cmd {
	cfg := m.cfg
	storePath := m.storePath
	return func() tea.Msg {
		st, err := store.Load(storePath)
		if err != nil {
			return runFinishedMsg{delivered: deliver, err: err}
		}
		var logBuf bytes.Buffer
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		res, err := pipeline.Run(ctx, pipeline.Options{
			Config:  cfg,
			Store:   st,
			Now:     time.Now(),
			Log:     &logBuf,
			Deliver: deliver,
		})
		if err != nil {
			return runFinishedMsg{delivered: deliver, err: err}
		}

		out := "(no new items — nothing to send)"
		if !res.Digest.Empty() {
			md, rerr := res.Digest.RenderMarkdown()
			if rerr != nil {
				return runFinishedMsg{delivered: deliver, err: rerr}
			}
			out = md
		}

		msg := runFinishedMsg{delivered: deliver, output: out, newItems: res.NewItemCount}
		if deliver {
			msg.summary, msg.failed = summarizeDeliveries(res.Deliveries)
		}
		return msg
	}
}

// summarizeDeliveries renders "email ✓ · webhook ✗ (reason)" and reports whether
// any channel failed.
func summarizeDeliveries(results []deliver.Result) (string, bool) {
	if len(results) == 0 {
		return "no delivery channels enabled", false
	}
	var parts []string
	failed := false
	for _, r := range results {
		if r.Err != nil {
			failed = true
			parts = append(parts, fmt.Sprintf("%s ✗ (%s)", r.Channel, r.Err))
		} else {
			parts = append(parts, r.Channel+" ✓")
		}
	}
	return strings.Join(parts, " · "), failed
}

func (m model) updateRun(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc", "q", "enter":
			m.screen = screenSources
			return m, nil
		case "ctrl+c":
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.view, cmd = m.view.Update(msg)
	return m, cmd
}

func (m model) viewRun() string {
	title := "Run preview"
	if m.runDelivered {
		title = "Send now"
	}
	header := titleStyle.Render(title) + "  " + m.renderStatus()
	help := helpStyle.Render("↑↓ scroll · esc back")
	return header + "\n" + m.view.View() + "\n" + help
}
