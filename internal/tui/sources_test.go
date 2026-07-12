package tui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/quangkhaidam93/dev-digest/internal/config"
)

func TestSourcesViewShowsCronStatus(t *testing.T) {
	dir := t.TempDir()
	m := newModel(config.Default(), filepath.Join(dir, "c.toml"), filepath.Join(dir, "s.json"))

	m.cronRegistered = false
	if got := m.viewSources(); !strings.Contains(got, "not registered") {
		t.Errorf("expected 'not registered' in view, got:\n%s", got)
	}

	m.cronRegistered = true
	m.cronSchedule = "0 8 * * *"
	got := m.viewSources()
	if !strings.Contains(got, "cron registered") {
		t.Errorf("expected 'cron registered' in view:\n%s", got)
	}
	if !strings.Contains(got, "0 8 * * *") {
		t.Errorf("expected the schedule shown in view:\n%s", got)
	}
}
