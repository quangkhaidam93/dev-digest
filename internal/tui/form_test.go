package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/quangkhaidam93/dev-digest/internal/config"
)

func key(s string) tea.KeyMsg {
	switch s {
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func feed(m model, msgs ...tea.Msg) model {
	for _, msg := range msgs {
		var tm tea.Model
		tm, _ = m.updateForm(msg)
		m = tm.(model)
	}
	return m
}

// Reproduces "cannot edit repo field": add a github source and try to type into
// the repo input.
func TestFormEditRepo(t *testing.T) {
	m := newModel(config.Default(), t.TempDir()+"/c.toml", t.TempDir()+"/s.json")
	m.screen = screenForm
	m.form = newForm(config.Source{Type: config.SourceGitHub}, -1)

	// focus starts on the Type row. Tab to Name, tab to Repo.
	m = feed(m, key("tab")) // -> Name
	if m.form.focus != fldName {
		t.Fatalf("after 1 tab focus=%v want fldName", m.form.focus)
	}
	m = feed(m, key("tab")) // -> Repo
	if m.form.focus != fldRepo {
		t.Fatalf("after 2 tabs focus=%v want fldRepo", m.form.focus)
	}
	if !m.form.repo.Focused() {
		t.Errorf("repo input is not Focused() while focus==fldRepo")
	}

	// Type "flutter/flutter" into the repo field.
	for _, r := range "flutter/flutter" {
		m = feed(m, key(string(r)))
	}
	if got := m.form.repo.Value(); got != "flutter/flutter" {
		t.Errorf("repo value = %q, want %q", got, "flutter/flutter")
	}
}

// Realistic flow: press `a` (rss default), cycle type to github, then edit repo.
func TestFormCycleToGithubThenEditRepo(t *testing.T) {
	m := newModel(config.Default(), t.TempDir()+"/c.toml", t.TempDir()+"/s.json")
	m.screen = screenForm
	m.form = newForm(config.Source{Type: config.SourceRSS}, -1) // what `a` does

	m = feed(m, key("right")) // rss -> github (on the Type row)
	if m.form.srcType != config.SourceGitHub {
		t.Fatalf("srcType=%q want github", m.form.srcType)
	}
	m = feed(m, key("down"), key("down")) // Type -> Name -> Repo
	if m.form.focus != fldRepo {
		t.Fatalf("focus=%v want fldRepo", m.form.focus)
	}
	for _, r := range "flutter/flutter" {
		m = feed(m, key(string(r)))
	}
	if got := m.form.repo.Value(); got != "flutter/flutter" {
		t.Errorf("repo value = %q, want %q", got, "flutter/flutter")
	}
}

// Typing while the Type selector row is focused must not be silently dropped —
// it should advance focus to the first editable field and capture the keystroke.
func TestFormTypingOnSelectorRowAdvances(t *testing.T) {
	m := newModel(config.Default(), t.TempDir()+"/c.toml", t.TempDir()+"/s.json")
	m.screen = screenForm
	m.form = newForm(config.Source{Type: config.SourceRSS}, -1)

	m = feed(m, key("right")) // rss -> github; focus still on Type row
	// User immediately types without tabbing down.
	for _, r := range "Flutter" {
		m = feed(m, key(string(r)))
	}
	if m.form.focus != fldName {
		t.Fatalf("focus=%v want fldName after typing on Type row", m.form.focus)
	}
	if got := m.form.name.Value(); got != "Flutter" {
		t.Errorf("name value = %q, want %q (keystrokes should not be lost)", got, "Flutter")
	}
}

// Flow where the user tabs into an rss-only field first, then switches type.
func TestFormSwitchTypeAfterFocusingURL(t *testing.T) {
	m := newModel(config.Default(), t.TempDir()+"/c.toml", t.TempDir()+"/s.json")
	m.screen = screenForm
	m.form = newForm(config.Source{Type: config.SourceRSS}, -1)

	m = feed(m, key("down"), key("down")) // Type -> Name -> URL (rss)
	if m.form.focus != fldURL {
		t.Fatalf("focus=%v want fldURL", m.form.focus)
	}
	// Go back to the Type row and switch to github.
	m = feed(m, key("up"), key("up")) // URL -> Name -> Type
	if m.form.focus != fldType {
		t.Fatalf("focus=%v want fldType", m.form.focus)
	}
	m = feed(m, key("right"))             // -> github
	m = feed(m, key("down"), key("down")) // Type -> Name -> Repo
	if m.form.focus != fldRepo {
		t.Fatalf("focus=%v want fldRepo", m.form.focus)
	}
	if !m.form.repo.Focused() {
		t.Errorf("repo not focused")
	}
	for _, r := range "flutter/flutter" {
		m = feed(m, key(string(r)))
	}
	if got := m.form.repo.Value(); got != "flutter/flutter" {
		t.Errorf("repo value = %q, want %q", got, "flutter/flutter")
	}
}
