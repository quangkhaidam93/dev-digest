package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/quangkhaidam93/dev-digest/internal/config"
)

// buildModel returns a model whose config has no sources (so a run produces no
// news and the offline fallback question fires) and file delivery to dir.
func buildModel(t *testing.T, dir string) model {
	t.Helper()
	cfg := config.Default()
	cfg.Sources = nil
	cfg.Digest.Provider = config.ProviderGemini // no key -> fallback question, no network
	cfg.Digest.QuestionWhenEmpty = true
	cfg.Delivery.File = config.FileDelivery{Enabled: true, Dir: dir, Formats: []string{"md"}}
	cfg.Delivery.Email.Enabled = false
	cfg.Delivery.Webhook.Enabled = false
	return newModel(cfg, filepath.Join(dir, "config.toml"), filepath.Join(dir, "state.json"))
}

func TestRunSendDelivers(t *testing.T) {
	dir := t.TempDir()
	m := buildModel(t, dir)

	msg, ok := m.runCmd(true)().(runFinishedMsg)
	if !ok {
		t.Fatal("expected runFinishedMsg")
	}
	if msg.err != nil {
		t.Fatalf("run error: %v", msg.err)
	}
	if !msg.delivered {
		t.Error("send run should report delivered=true")
	}
	if msg.failed {
		t.Errorf("delivery should have succeeded: %s", msg.summary)
	}
	// A file should have been written (the question-of-the-day).
	entries, _ := os.ReadDir(dir)
	found := false
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".md" {
			found = true
		}
	}
	if !found {
		t.Error("send run should have written a digest file")
	}
}

func TestRunPreviewDoesNotDeliver(t *testing.T) {
	dir := t.TempDir()
	m := buildModel(t, dir)

	msg, ok := m.runCmd(false)().(runFinishedMsg)
	if !ok {
		t.Fatal("expected runFinishedMsg")
	}
	if msg.err != nil {
		t.Fatalf("preview error: %v", msg.err)
	}
	if msg.delivered {
		t.Error("preview should report delivered=false")
	}
	// No file should have been written.
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".md" {
			t.Errorf("preview must not write files, found %s", e.Name())
		}
	}
	// But the preview should still show the question content.
	if msg.output == "" || msg.output == "(no new items — nothing to send)" {
		t.Errorf("preview should render the question, got %q", msg.output)
	}
}
