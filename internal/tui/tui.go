// Package tui implements the interactive terminal UI for managing sources,
// toggling settings, previewing a run, and installing the cron schedule. All
// edits persist back to the config.toml the app was launched with.
package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/quangkhaidam93/dev-digest/internal/config"
	"github.com/quangkhaidam93/dev-digest/internal/cron"
	"github.com/quangkhaidam93/dev-digest/internal/store"
)

type screen int

const (
	screenSources screen = iota
	screenForm
	screenSettings
	screenRun
	screenPrompt
)

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#c05621"))
	helpStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#8a8577"))
	statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#2f855a"))
	errStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#c53030"))
)

// Run loads config at path and starts the TUI event loop.
func Run(path string) error {
	cfg, _, err := config.Load(path)
	if err != nil {
		return err
	}
	sp, err := store.DefaultPath()
	if err != nil {
		return err
	}
	m := newModel(cfg, path, sp)
	m.refreshCronStatus()
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

// refreshCronStatus queries the crontab once and caches whether dev-digest's
// entry is registered plus its schedule fields.
func (m *model) refreshCronStatus() {
	installed, line, err := cron.Status()
	if err != nil || !installed {
		m.cronRegistered = false
		m.cronSchedule = ""
		return
	}
	m.cronRegistered = true
	if f := strings.Fields(line); len(f) >= 5 {
		m.cronSchedule = strings.Join(f[:5], " ")
	} else {
		m.cronSchedule = line
	}
}

type model struct {
	cfg       config.Config
	path      string
	storePath string

	screen       screen
	list         list.Model
	form         formModel
	settings     settingsModel
	view         viewport.Model
	prompt       textarea.Model
	runDelivered bool // whether the last run view was a real send (vs preview)

	width, height int
	status        string
	statusErr     bool

	cronRegistered bool
	cronSchedule   string // the "M H * * *" fields of the registered entry
}

func newModel(cfg config.Config, path, storePath string) model {
	m := model{cfg: cfg, path: path, storePath: storePath, screen: screenSources}
	m.list = newSourceList(cfg.Sources)
	m.view = viewport.New(0, 0)
	return m
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.list.SetSize(msg.Width, msg.Height-4)
		m.view.Width = msg.Width
		m.view.Height = msg.Height - 4
		if m.screen == screenPrompt {
			m.prompt.SetWidth(msg.Width - 4)
			m.prompt.SetHeight(msg.Height - 6)
		}
		return m, nil

	case runFinishedMsg:
		m.screen = screenRun
		m.runDelivered = msg.delivered
		switch {
		case msg.err != nil:
			m.setStatus("run failed: "+msg.err.Error(), true)
			m.view.SetContent(msg.err.Error())
		case msg.delivered:
			s := fmt.Sprintf("%d new item(s) · %s", msg.newItems, msg.summary)
			m.setStatus(s, msg.failed)
			m.view.SetContent(msg.output)
		default:
			m.setStatus(fmt.Sprintf("preview only (not sent): %d new item(s)", msg.newItems), false)
			m.view.SetContent(msg.output)
		}
		m.view.GotoTop()
		return m, nil
	}

	switch m.screen {
	case screenSources:
		return m.updateSources(msg)
	case screenForm:
		return m.updateForm(msg)
	case screenSettings:
		return m.updateSettings(msg)
	case screenRun:
		return m.updateRun(msg)
	case screenPrompt:
		return m.updatePrompt(msg)
	}
	return m, nil
}

func (m model) View() string {
	switch m.screen {
	case screenForm:
		return m.viewForm()
	case screenSettings:
		return m.viewSettings()
	case screenRun:
		return m.viewRun()
	case screenPrompt:
		return m.viewPrompt()
	default:
		return m.viewSources()
	}
}

func (m *model) setStatus(s string, isErr bool) {
	m.status = s
	m.statusErr = isErr
}

func (m model) renderStatus() string {
	if m.status == "" {
		return ""
	}
	if m.statusErr {
		return errStyle.Render(m.status)
	}
	return statusStyle.Render(m.status)
}

// save writes the current config to disk and sets a status message.
func (m *model) save() {
	if err := config.Save(m.path, m.cfg); err != nil {
		m.setStatus("save failed: "+err.Error(), true)
		return
	}
	m.setStatus("saved "+m.path, false)
}

// installCron installs the daily crontab entry pointing at this binary.
func (m *model) installCron() {
	bin, err := os.Executable()
	if err != nil {
		m.setStatus("cron: "+err.Error(), true)
		return
	}
	bin, _ = filepath.Abs(bin)
	expr, err := m.cfg.Schedule.CronExpr()
	if err != nil {
		m.setStatus("cron: "+err.Error(), true)
		return
	}
	entry := cron.Entry{Schedule: expr, Binary: bin, LogPath: cron.DefaultLogPath()}
	if def, _ := config.DefaultPath(); def != m.path {
		entry.Config = m.path
	}
	_ = os.MkdirAll(filepath.Dir(entry.LogPath), 0o755)
	if err := cron.Install(entry); err != nil {
		m.setStatus("cron install failed: "+err.Error(), true)
		return
	}
	m.refreshCronStatus()
	m.setStatus("installed cron: "+entry.Line(), false)
}

func (m *model) uninstallCron() {
	if err := cron.Uninstall(); err != nil {
		m.setStatus("cron uninstall failed: "+err.Error(), true)
		return
	}
	m.refreshCronStatus()
	m.setStatus("removed cron entry", false)
}
