package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/quangkhaidam93/dev-digest/internal/config"
)

// field identifies a focusable row in the form.
type field int

const (
	fldType field = iota // cycled, not a text input
	fldName
	fldURL
	fldRepo
	fldKind // cycled
	fldSelector
)

var sourceTypes = []string{config.SourceRSS, config.SourceGitHub, config.SourceWebpage}
var githubKinds = []string{config.GitHubReleases, config.GitHubTags}

// formModel is the add/edit source screen.
type formModel struct {
	editIdx int // -1 for add, else index into cfg.Sources
	srcType string
	kind    string
	name    textinput.Model
	url     textinput.Model
	repo    textinput.Model
	selctr  textinput.Model
	focus   field
	err     string
}

func newForm(src config.Source, editIdx int) formModel {
	f := formModel{
		editIdx: editIdx,
		srcType: orDefault(src.Type, config.SourceRSS),
		kind:    orDefault(src.Kind, config.GitHubReleases),
		name:    newInput("Name", src.Name),
		url:     newInput("URL (https://…)", src.URL),
		repo:    newInput("Repo (owner/name)", src.Repo),
		selctr:  newInput("CSS selector (optional)", src.Selector),
		focus:   fldType,
	}
	return f
}

func newInput(placeholder, value string) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.SetValue(value)
	ti.Prompt = "› "
	return ti
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// rows returns the focusable fields for the current source type, in order.
func (f formModel) rows() []field {
	switch f.srcType {
	case config.SourceGitHub:
		return []field{fldType, fldName, fldRepo, fldKind}
	case config.SourceWebpage:
		return []field{fldType, fldName, fldURL, fldSelector}
	default: // rss
		return []field{fldType, fldName, fldURL}
	}
}

func (m *model) syncFormFocus() {
	// Blur all, focus the active text input (type/kind rows are not inputs).
	m.form.name.Blur()
	m.form.url.Blur()
	m.form.repo.Blur()
	m.form.selctr.Blur()
	switch m.form.focus {
	case fldName:
		m.form.name.Focus()
	case fldURL:
		m.form.url.Focus()
	case fldRepo:
		m.form.repo.Focus()
	case fldSelector:
		m.form.selctr.Focus()
	}
}

func (m model) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, isKey := msg.(tea.KeyMsg)
	if isKey {
		switch key.String() {
		case "esc":
			m.screen = screenSources
			return m, nil
		case "ctrl+c":
			return m, tea.Quit
		case "tab", "down":
			m.form.focus = m.nextRow(1)
			m.syncFormFocus()
			return m, nil
		case "shift+tab", "up":
			m.form.focus = m.nextRow(-1)
			m.syncFormFocus()
			return m, nil
		case "left", "right":
			if m.form.focus == fldType {
				m.form.srcType = cycle(sourceTypes, m.form.srcType, key.String() == "right")
				// Reset focus in case the row set changed.
				m.form.focus = fldType
				m.syncFormFocus()
				return m, nil
			}
			if m.form.focus == fldKind {
				m.form.kind = cycle(githubKinds, m.form.kind, key.String() == "right")
				return m, nil
			}
		case "enter":
			if err := m.saveForm(); err != "" {
				m.form.err = err
				return m, nil
			}
			m.screen = screenSources
			return m, nil
		}
	}

	// If the user types a character while a selector row (Type/Kind) is focused,
	// advance to the first editable field so the keystroke isn't silently lost.
	if isKey && (m.form.focus == fldType || m.form.focus == fldKind) &&
		(key.Type == tea.KeyRunes || key.Type == tea.KeySpace) {
		m.form.focus = fldName
		m.syncFormFocus()
	}

	// Route other key events to the focused text input.
	var cmd tea.Cmd
	switch m.form.focus {
	case fldName:
		m.form.name, cmd = m.form.name.Update(msg)
	case fldURL:
		m.form.url, cmd = m.form.url.Update(msg)
	case fldRepo:
		m.form.repo, cmd = m.form.repo.Update(msg)
	case fldSelector:
		m.form.selctr, cmd = m.form.selctr.Update(msg)
	}
	return m, cmd
}

// nextRow advances focus by dir (+1/-1) over the current row set, wrapping.
func (m model) nextRow(dir int) field {
	rows := m.form.rows()
	cur := 0
	for i, r := range rows {
		if r == m.form.focus {
			cur = i
			break
		}
	}
	cur = (cur + dir + len(rows)) % len(rows)
	return rows[cur]
}

// saveForm validates and writes the form into cfg. Returns "" on success or an
// error message.
func (m *model) saveForm() string {
	src := config.Source{
		Type: m.form.srcType,
		Name: strings.TrimSpace(m.form.name.Value()),
		URL:  strings.TrimSpace(m.form.url.Value()),
		Repo: strings.TrimSpace(m.form.repo.Value()),
		Kind: m.form.kind,
	}
	if m.form.srcType == config.SourceWebpage {
		src.Selector = strings.TrimSpace(m.form.selctr.Value())
	}
	if m.form.srcType != config.SourceGitHub {
		src.Repo, src.Kind = "", ""
	}
	if m.form.srcType == config.SourceGitHub {
		src.URL = ""
	}

	tmp := config.Config{Sources: []config.Source{src}}
	if err := tmp.Validate(); err != nil {
		return err.Error()
	}

	if m.form.editIdx >= 0 && m.form.editIdx < len(m.cfg.Sources) {
		m.cfg.Sources[m.form.editIdx] = src
	} else {
		m.cfg.Sources = append(m.cfg.Sources, src)
	}
	m.reloadList()
	m.save()
	return ""
}

func cycle(opts []string, cur string, forward bool) string {
	idx := 0
	for i, o := range opts {
		if o == cur {
			idx = i
			break
		}
	}
	if forward {
		idx = (idx + 1) % len(opts)
	} else {
		idx = (idx - 1 + len(opts)) % len(opts)
	}
	return opts[idx]
}

func (m model) viewForm() string {
	var b strings.Builder
	title := "Add source"
	if m.form.editIdx >= 0 {
		title = "Edit source"
	}
	b.WriteString(titleStyle.Render(title) + "\n\n")

	b.WriteString(m.formRow(fldType, "Type", "◂ "+m.form.srcType+" ▸") + "\n")
	b.WriteString(m.formRow(fldName, "Name", m.form.name.View()) + "\n")
	for _, r := range m.form.rows() {
		switch r {
		case fldURL:
			b.WriteString(m.formRow(fldURL, "URL", m.form.url.View()) + "\n")
		case fldRepo:
			b.WriteString(m.formRow(fldRepo, "Repo", m.form.repo.View()) + "\n")
		case fldKind:
			b.WriteString(m.formRow(fldKind, "Kind", "◂ "+m.form.kind+" ▸") + "\n")
		case fldSelector:
			b.WriteString(m.formRow(fldSelector, "Selector", m.form.selctr.View()) + "\n")
		}
	}

	if m.form.err != "" {
		b.WriteString("\n" + errStyle.Render(m.form.err) + "\n")
	}
	b.WriteString("\n" + helpStyle.Render("tab/↑↓ move · ←→ change type/kind · enter save · esc cancel"))
	return b.String()
}

func (m model) formRow(f field, label, value string) string {
	marker := "  "
	labelStyle := lipgloss.NewStyle().Width(12)
	if m.form.focus == f {
		marker = lipgloss.NewStyle().Foreground(lipgloss.Color("#c05621")).Render("➤ ")
		labelStyle = labelStyle.Bold(true)
	}
	return marker + labelStyle.Render(label) + value
}
