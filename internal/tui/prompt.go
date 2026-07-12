package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/quangkhaidam93/dev-digest/internal/digest"
)

// effectiveQuestionPrompt returns the custom prompt if set, else the built-in
// default (so the editor opens pre-filled with something to edit).
func (m model) effectiveQuestionPrompt() string {
	if strings.TrimSpace(m.cfg.Digest.QuestionPrompt) != "" {
		return m.cfg.Digest.QuestionPrompt
	}
	return digest.DefaultQuestionPrompt
}

// openPromptEditor initializes the textarea and switches to the editor screen.
func (m *model) openPromptEditor() {
	ta := textarea.New()
	ta.SetValue(m.effectiveQuestionPrompt())
	ta.CharLimit = 4000
	if m.width > 0 {
		ta.SetWidth(m.width - 4)
	}
	if m.height > 6 {
		ta.SetHeight(m.height - 6)
	}
	ta.Focus()
	m.prompt = ta
	m.screen = screenPrompt
}

func (m model) updatePrompt(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			// Store the edited prompt; if it's blank or matches the default,
			// store empty so config stays clean and tracks future default changes.
			v := m.prompt.Value()
			if strings.TrimSpace(v) == "" || v == digest.DefaultQuestionPrompt {
				m.cfg.Digest.QuestionPrompt = ""
			} else {
				m.cfg.Digest.QuestionPrompt = v
			}
			m.save()
			m.screen = screenSettings
			return m, nil
		case "ctrl+r":
			m.prompt.SetValue(digest.DefaultQuestionPrompt)
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.prompt, cmd = m.prompt.Update(msg)
	return m, cmd
}

func (m model) viewPrompt() string {
	header := titleStyle.Render("Edit question prompt")
	sub := helpStyle.Render("This system prompt tells the model how to generate the no-news question.")
	help := helpStyle.Render("edit freely · ctrl+r reset to default · esc save & back")
	return header + "\n" + sub + "\n\n" + m.prompt.View() + "\n" + help
}
