package config

import (
	"path/filepath"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	want := Default()
	want.Digest.Title = "My Digest"
	want.Sources = []Source{
		{Type: SourceRSS, Name: "Bytes", URL: "https://bytes.dev/rss"},
		{Type: SourceGitHub, Name: "React", Repo: "facebook/react", Kind: GitHubReleases},
		{Type: SourceWebpage, Name: "Go", URL: "https://go.dev/doc/devel/release", Selector: "#content"},
	}
	want.Delivery.Webhook = WebhookDelivery{Enabled: true, Kind: WebhookSlack, URL: "https://hooks.slack.com/x"}

	if err := Save(path, want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, exists, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !exists {
		t.Fatal("expected exists=true after Save")
	}
	if got.Digest.Title != want.Digest.Title {
		t.Errorf("title: got %q want %q", got.Digest.Title, want.Digest.Title)
	}
	if len(got.Sources) != 3 {
		t.Fatalf("sources: got %d want 3", len(got.Sources))
	}
	if got.Sources[1].Repo != "facebook/react" {
		t.Errorf("repo: got %q", got.Sources[1].Repo)
	}
	if !got.Delivery.Webhook.Enabled || got.Delivery.Webhook.URL != "https://hooks.slack.com/x" {
		t.Errorf("webhook not preserved: %+v", got.Delivery.Webhook)
	}
}

func TestLoadMissingReturnsDefault(t *testing.T) {
	got, exists, err := Load(filepath.Join(t.TempDir(), "nope.toml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if exists {
		t.Error("expected exists=false for missing file")
	}
	if got.Digest.Model != "claude-opus-4-8" {
		t.Errorf("default model: got %q", got.Digest.Model)
	}
}

func TestAPIKeyResolution(t *testing.T) {
	// Config value wins over env.
	t.Setenv("GEMINI_API_KEY", "env-key")
	cfg := Config{Keys: map[string]string{ProviderGemini: "cfg-key"}}
	if got := cfg.APIKey(ProviderGemini); got != "cfg-key" {
		t.Errorf("config key should win: got %q", got)
	}

	// Env fallback when no config key.
	if got := (Config{}).APIKey(ProviderGemini); got != "env-key" {
		t.Errorf("env fallback: got %q", got)
	}

	// GOOGLE_API_KEY fallback for gemini.
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("GOOGLE_API_KEY", "google-key")
	if got := (Config{}).APIKey(ProviderGemini); got != "google-key" {
		t.Errorf("GOOGLE_API_KEY fallback: got %q", got)
	}

	// SetKey stores and clears.
	var c Config
	c.SetKey(ProviderOpenRouter, "  or-key  ")
	if c.Keys[ProviderOpenRouter] != "or-key" {
		t.Errorf("SetKey should trim and store: %q", c.Keys[ProviderOpenRouter])
	}
	c.SetKey(ProviderOpenRouter, "")
	if _, ok := c.Keys[ProviderOpenRouter]; ok {
		t.Error("SetKey with empty should delete the entry")
	}
}

func TestMaxAgeDuration(t *testing.T) {
	if d := (Digest{MaxAge: "24h"}).MaxAgeDuration(); d.Hours() != 24 {
		t.Errorf("24h -> %v", d)
	}
	if d := (Digest{MaxAge: ""}).MaxAgeDuration(); d != 0 {
		t.Errorf("empty -> %v, want 0", d)
	}
	if d := (Digest{MaxAge: "0"}).MaxAgeDuration(); d != 0 {
		t.Errorf("0 -> %v, want 0", d)
	}
	if d := (Digest{MaxAge: "garbage"}).MaxAgeDuration(); d != 0 {
		t.Errorf("garbage -> %v, want 0", d)
	}
}

func TestScheduleCronExpr(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"08:00", "0 8 * * *", false},
		{"", "0 8 * * *", false}, // default
		{"9:5", "5 9 * * *", false},
		{"23:59", "59 23 * * *", false},
		{"00:00", "0 0 * * *", false},
		{"24:00", "", true},
		{"08:60", "", true},
		{"8", "", true},
		{"aa:bb", "", true},
	}
	for _, tt := range tests {
		got, err := Schedule{DailyTime: tt.in}.CronExpr()
		if (err != nil) != tt.wantErr {
			t.Errorf("CronExpr(%q) err=%v wantErr=%v", tt.in, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && got != tt.want {
			t.Errorf("CronExpr(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestValidateRejectsBadSchedule(t *testing.T) {
	c := Default()
	c.Schedule.DailyTime = "99:99"
	if err := c.Validate(); err == nil {
		t.Error("expected validation error for bad schedule time")
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		src     Source
		wantErr bool
	}{
		{"rss ok", Source{Type: SourceRSS, Name: "a", URL: "http://x"}, false},
		{"rss no url", Source{Type: SourceRSS, Name: "a"}, true},
		{"github ok", Source{Type: SourceGitHub, Name: "a", Repo: "o/r"}, false},
		{"github no repo", Source{Type: SourceGitHub, Name: "a"}, true},
		{"github bad kind", Source{Type: SourceGitHub, Name: "a", Repo: "o/r", Kind: "commits"}, true},
		{"webpage ok", Source{Type: SourceWebpage, Name: "a", URL: "http://x"}, false},
		{"unknown type", Source{Type: "ftp", Name: "a"}, true},
		{"no name", Source{Type: SourceRSS, URL: "http://x"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := Config{Sources: []Source{tt.src}}
			err := c.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() err=%v, wantErr=%v", err, tt.wantErr)
			}
		})
	}
}
