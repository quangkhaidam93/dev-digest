package tui

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/quangkhaidam93/dev-digest/internal/config"
)

func feedSet(m model, msgs ...tea.Msg) model {
	for _, msg := range msgs {
		tm, _ := m.updateSettings(msg)
		m = tm.(model)
	}
	return m
}

func settingsModelAt(t *testing.T, cfg config.Config) (model, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	m := newModel(cfg, path, filepath.Join(dir, "state.json"))
	m.settings = newSettings(cfg)
	m.screen = screenSettings
	return m, path
}

// Editing the Max age field persists to config.
func TestSettingsEditMaxAge(t *testing.T) {
	cfg := config.Default()
	cfg.Digest.MaxAge = ""
	m, path := settingsModelAt(t, cfg)

	// Summarize(0) -> Provider -> Model -> APIKey -> Effort -> MaxAge (5 downs).
	m = feedSet(m, key("down"), key("down"), key("down"), key("down"), key("down"))
	if m.settings.focus != setMaxAge {
		t.Fatalf("focus=%v want setMaxAge", m.settings.focus)
	}
	for _, r := range "48h" {
		m = feedSet(m, key(string(r)))
	}
	m = feedSet(m, tea.KeyMsg{Type: tea.KeyEsc})

	if m.cfg.Digest.MaxAge != "48h" {
		t.Errorf("in-memory max_age = %q, want 48h", m.cfg.Digest.MaxAge)
	}
	got, _, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Digest.MaxAge != "48h" {
		t.Errorf("persisted max_age = %q, want 48h", got.Digest.MaxAge)
	}
}

// Setting the API key for the active provider persists under [keys], and the
// file is written 0600.
func TestSettingsEditAPIKey(t *testing.T) {
	m, path := settingsModelAt(t, config.Default())

	// Move to Provider and cycle anthropic -> gemini.
	m = feedSet(m, key("down"))
	if m.settings.focus != setProvider {
		t.Fatalf("focus=%v want setProvider", m.settings.focus)
	}
	m = feedSet(m, key("right"))
	if m.cfg.Digest.Provider != config.ProviderGemini {
		t.Fatalf("provider=%q want gemini", m.cfg.Digest.Provider)
	}

	// Provider -> Model -> APIKey, then type the key.
	m = feedSet(m, key("down"), key("down"))
	if m.settings.focus != setAPIKey {
		t.Fatalf("focus=%v want setAPIKey", m.settings.focus)
	}
	for _, r := range "gkey-123" {
		m = feedSet(m, key(string(r)))
	}
	m = feedSet(m, tea.KeyMsg{Type: tea.KeyEsc})

	if got := m.cfg.Keys[config.ProviderGemini]; got != "gkey-123" {
		t.Errorf("in-memory gemini key = %q, want gkey-123", got)
	}
	loaded, _, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded.APIKey(config.ProviderGemini); got != "gkey-123" {
		t.Errorf("persisted gemini key = %q, want gkey-123", got)
	}
	if fi, err := os.Stat(path); err != nil {
		t.Fatal(err)
	} else if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Errorf("config perms = %o, want 600", perm)
	}
}

// Cycling the provider resets the model to that provider's default and loads
// its stored key into the API-key field.
func TestSettingsProviderCycleLoadsKeyAndModel(t *testing.T) {
	cfg := config.Default()
	cfg.SetKey(config.ProviderGemini, "stored-gemini-key")
	m, _ := settingsModelAt(t, cfg)

	m = feedSet(m, key("down"), key("right")) // Provider -> gemini
	if m.cfg.Digest.Model != config.DefaultModel(config.ProviderGemini) {
		t.Errorf("model = %q, want gemini default", m.cfg.Digest.Model)
	}
	if m.settings.ti[setModel].Value() != config.DefaultModel(config.ProviderGemini) {
		t.Errorf("model input = %q, want gemini default", m.settings.ti[setModel].Value())
	}
	if m.settings.ti[setAPIKey].Value() != "stored-gemini-key" {
		t.Errorf("apiKey input = %q, want stored-gemini-key", m.settings.ti[setAPIKey].Value())
	}
}

// Enabling email reveals its fields; editing them persists, and the SMTP
// password is stored in config.
func TestSettingsEditEmailFields(t *testing.T) {
	m, path := settingsModelAt(t, config.Default())

	// Navigate to Email toggle and enable it.
	for m.settings.focus != setEmail {
		m = feedSet(m, key("down"))
	}
	m = feedSet(m, key(" "))
	if !m.cfg.Delivery.Email.Enabled {
		t.Fatal("email not enabled after toggle")
	}
	// Sub-fields must now be visible.
	if !contains(m.visibleRows(), setEmailHost) {
		t.Fatal("email sub-fields not revealed after enabling")
	}

	// Fill each email field (clearing any prefilled value first).
	edit := func(f setField, text string) {
		for m.settings.focus != f {
			m = feedSet(m, key("down"))
		}
		for range m.settings.ti[f].Value() {
			m = feedSet(m, tea.KeyMsg{Type: tea.KeyBackspace})
		}
		for _, r := range text {
			m = feedSet(m, key(string(r)))
		}
	}
	edit(setEmailHost, "smtp.example.com")
	edit(setEmailPort, "465")
	edit(setEmailUser, "me@example.com")
	edit(setEmailPass, "s3cret")
	edit(setEmailFrom, "me@example.com")
	edit(setEmailTo, "a@x.com, b@y.com")

	m = feedSet(m, tea.KeyMsg{Type: tea.KeyEsc})

	loaded, _, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	e := loaded.Delivery.Email
	if e.SMTPHost != "smtp.example.com" || e.SMTPPort != 465 || e.Username != "me@example.com" ||
		e.Password != "s3cret" || e.From != "me@example.com" {
		t.Errorf("email persisted wrong: %+v", e)
	}
	if len(e.To) != 2 || e.To[0] != "a@x.com" || e.To[1] != "b@y.com" {
		t.Errorf("recipients = %v, want [a@x.com b@y.com]", e.To)
	}
}

// esc-time validation blocks leaving with an enabled-but-incomplete email config.
func TestSettingsEmailValidation(t *testing.T) {
	cfg := config.Default()
	cfg.Delivery.Email.Enabled = true // enabled but no host/recipients
	m, _ := settingsModelAt(t, cfg)

	m = feedSet(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.screen != screenSettings {
		t.Error("should stay on settings when email config is incomplete")
	}
	if !m.statusErr {
		t.Error("expected an error status for incomplete email config")
	}
}

// Editing the daily schedule time persists and converts to the right cron.
func TestSettingsEditScheduleTime(t *testing.T) {
	m, path := settingsModelAt(t, config.Default())

	for m.settings.focus != setDailyTime {
		m = feedSet(m, key("down"))
	}
	// Clear the prefilled "08:00" and type a new time.
	for range m.settings.ti[setDailyTime].Value() {
		m = feedSet(m, tea.KeyMsg{Type: tea.KeyBackspace})
	}
	for _, r := range "21:30" {
		m = feedSet(m, key(string(r)))
	}
	m = feedSet(m, tea.KeyMsg{Type: tea.KeyEsc})

	loaded, _, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Schedule.DailyTime != "21:30" {
		t.Errorf("daily_time = %q, want 21:30", loaded.Schedule.DailyTime)
	}
	expr, err := loaded.Schedule.CronExpr()
	if err != nil || expr != "30 21 * * *" {
		t.Errorf("CronExpr = %q (err %v), want 30 21 * * *", expr, err)
	}
}

// An invalid time keeps you on the settings screen with an error.
func TestSettingsRejectsBadScheduleTime(t *testing.T) {
	m, _ := settingsModelAt(t, config.Default())
	for m.settings.focus != setDailyTime {
		m = feedSet(m, key("down"))
	}
	for range m.settings.ti[setDailyTime].Value() {
		m = feedSet(m, tea.KeyMsg{Type: tea.KeyBackspace})
	}
	for _, r := range "99:99" {
		m = feedSet(m, key(string(r)))
	}
	m = feedSet(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.screen != screenSettings || !m.statusErr {
		t.Error("bad schedule time should block leaving settings with an error")
	}
}

func contains(rows []setField, f setField) bool {
	for _, r := range rows {
		if r == f {
			return true
		}
	}
	return false
}
