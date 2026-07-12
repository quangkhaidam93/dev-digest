package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/quangkhaidam93/dev-digest/internal/config"
)

// sourceItem adapts a config.Source to the list.Item interface.
type sourceItem struct{ src config.Source }

func (i sourceItem) Title() string { return i.src.Name }
func (i sourceItem) Description() string {
	switch i.src.Type {
	case config.SourceRSS:
		return "rss · " + i.src.URL
	case config.SourceGitHub:
		kind := i.src.Kind
		if kind == "" {
			kind = config.GitHubReleases
		}
		return fmt.Sprintf("github · %s (%s)", i.src.Repo, kind)
	case config.SourceWebpage:
		return "webpage · " + i.src.URL
	}
	return i.src.Type
}
func (i sourceItem) FilterValue() string { return i.src.Name }

func newSourceList(srcs []config.Source) list.Model {
	items := make([]list.Item, len(srcs))
	for i, s := range srcs {
		items[i] = sourceItem{src: s}
	}
	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "dev-digest — sources"
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	return l
}

func (m *model) reloadList() {
	items := make([]list.Item, len(m.cfg.Sources))
	for i, s := range m.cfg.Sources {
		items[i] = sourceItem{src: s}
	}
	m.list.SetItems(items)
}

func (m model) updateSources(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		// Ignore shortcuts while filtering text is being entered.
		if m.list.FilterState() != list.Filtering {
			switch key.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "a":
				m.form = newForm(config.Source{Type: config.SourceRSS}, -1)
				m.screen = screenForm
				return m, nil
			case "e":
				if it, ok := m.list.SelectedItem().(sourceItem); ok {
					m.form = newForm(it.src, m.list.Index())
					m.screen = screenForm
				}
				return m, nil
			case "d":
				if _, ok := m.list.SelectedItem().(sourceItem); ok {
					idx := m.list.Index()
					m.cfg.Sources = append(m.cfg.Sources[:idx], m.cfg.Sources[idx+1:]...)
					m.reloadList()
					m.save()
				}
				return m, nil
			case "s":
				m.settings = newSettings(m.cfg)
				m.screen = screenSettings
				return m, nil
			case "r":
				m.setStatus("sending…", false)
				return m, m.runCmd(true)
			case "p":
				m.setStatus("previewing…", false)
				return m, m.runCmd(false)
			case "c":
				m.installCron()
				return m, nil
			case "u":
				m.uninstallCron()
				return m, nil
			}
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) viewSources() string {
	help := helpStyle.Render("a add · e edit · d delete · r send now · p preview · s settings · c cron · u uncron · q quit")
	body := m.list.View()
	status := m.renderStatus()
	if status != "" {
		return body + "\n" + status + "\n" + help
	}
	return body + "\n" + help
}
