package tui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/quangkhaidam93/dev-digest/internal/config"
	"github.com/quangkhaidam93/dev-digest/internal/digest"
)

// feedApp routes messages through the top-level Update so screen dispatch (and
// the settings→prompt-editor transition) is exercised realistically.
func feedApp(m model, msgs ...tea.Msg) model {
	for _, msg := range msgs {
		tm, _ := m.Update(msg)
		m = tm.(model)
	}
	return m
}

func TestPromptEditorSaveAndReset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	m := newModel(config.Default(), path, filepath.Join(dir, "state.json"))
	m.settings = newSettings(m.cfg)
	m.screen = screenSettings

	// Navigate to the prompt row and open the editor.
	for m.settings.focus != setQuestionPrompt {
		m = feedApp(m, tea.KeyMsg{Type: tea.KeyDown})
	}
	m = feedApp(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != screenPrompt {
		t.Fatalf("enter on prompt row should open the editor, screen=%v", m.screen)
	}

	// Append custom text, then save & back.
	for _, r := range " EXTRA RULE" {
		m = feedApp(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m = feedApp(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.screen != screenSettings {
		t.Fatalf("esc should return to settings, screen=%v", m.screen)
	}
	if !strings.Contains(m.cfg.Digest.QuestionPrompt, "EXTRA RULE") {
		t.Errorf("custom prompt not stored: %q", m.cfg.Digest.QuestionPrompt)
	}
	// Persisted to disk.
	loaded, _, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(loaded.Digest.QuestionPrompt, "EXTRA RULE") {
		t.Errorf("custom prompt not persisted: %q", loaded.Digest.QuestionPrompt)
	}

	// Reopen, reset to default, save → stored as empty (tracks the default).
	m = feedApp(m, tea.KeyMsg{Type: tea.KeyEnter}) // still on the prompt row
	if m.screen != screenPrompt {
		t.Fatalf("expected editor screen again, got %v", m.screen)
	}
	m = feedApp(m, tea.KeyMsg{Type: tea.KeyCtrlR}) // reset to default
	if m.prompt.Value() != digest.DefaultQuestionPrompt {
		t.Errorf("ctrl+r should load the default prompt")
	}
	m = feedApp(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.cfg.Digest.QuestionPrompt != "" {
		t.Errorf("resetting to default should clear the stored prompt, got %q", m.cfg.Digest.QuestionPrompt)
	}
}
