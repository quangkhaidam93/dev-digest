// Package config defines the on-disk configuration for dev-digest and helpers
// to load and save it. Config is human-editable TOML; the TUI reads and rewrites
// the same file. Secrets (provider API keys, SMTP password) may be stored here —
// the file is written with 0600 permissions — or supplied via environment
// variables (ANTHROPIC_API_KEY / GEMINI_API_KEY / OPENROUTER_API_KEY,
// GITHUB_TOKEN, DEV_DIGEST_SMTP_PASSWORD), which act as fallbacks.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// Source types.
const (
	SourceRSS     = "rss"
	SourceGitHub  = "github"
	SourceWebpage = "webpage"
)

// GitHub source kinds.
const (
	GitHubReleases = "releases"
	GitHubTags     = "tags"
)

// Webhook kinds.
const (
	WebhookSlack   = "slack"
	WebhookDiscord = "discord"
	WebhookGeneric = "generic"
)

// LLM providers.
const (
	ProviderAnthropic  = "anthropic"
	ProviderGemini     = "gemini"
	ProviderOpenRouter = "openrouter"
)

// Providers lists the supported providers in display order.
var Providers = []string{ProviderAnthropic, ProviderGemini, ProviderOpenRouter}

// DefaultModel returns the default model id for a provider (empty for unknown).
func DefaultModel(provider string) string {
	switch provider {
	case ProviderAnthropic:
		return "claude-opus-4-8"
	case ProviderGemini:
		return "gemini-2.5-flash"
	case ProviderOpenRouter:
		return "google/gemini-2.5-flash"
	default:
		return ""
	}
}

// APIKeyEnv returns the environment variable a provider reads its key from.
func APIKeyEnv(provider string) string {
	switch provider {
	case ProviderAnthropic:
		return "ANTHROPIC_API_KEY"
	case ProviderGemini:
		return "GEMINI_API_KEY"
	case ProviderOpenRouter:
		return "OPENROUTER_API_KEY"
	default:
		return ""
	}
}

// Config is the root configuration document.
type Config struct {
	Digest   Digest   `toml:"digest"`
	Schedule Schedule `toml:"schedule"`
	Sources  []Source `toml:"sources"`
	Delivery Delivery `toml:"delivery"`
	// Keys holds provider API keys by provider name. Optional — a provider's key
	// may instead come from its environment variable (see APIKey). Because this
	// can hold secrets, the config file is written with 0600 permissions.
	Keys map[string]string `toml:"keys,omitempty"`
}

// APIKey resolves the API key for a provider: the value stored in [keys] if set,
// otherwise the provider's environment variable.
func (c Config) APIKey(provider string) string {
	if c.Keys != nil {
		if k := strings.TrimSpace(c.Keys[provider]); k != "" {
			return k
		}
	}
	if env := APIKeyEnv(provider); env != "" {
		if v := strings.TrimSpace(os.Getenv(env)); v != "" {
			return v
		}
	}
	if provider == ProviderGemini {
		return strings.TrimSpace(os.Getenv("GOOGLE_API_KEY"))
	}
	return ""
}

// SetKey stores (or clears, when key is empty) a provider's API key.
func (c *Config) SetKey(provider, key string) {
	key = strings.TrimSpace(key)
	if key == "" {
		delete(c.Keys, provider)
		return
	}
	if c.Keys == nil {
		c.Keys = map[string]string{}
	}
	c.Keys[provider] = key
}

// Digest holds summarization settings.
type Digest struct {
	Title     string `toml:"title"`
	Summarize bool   `toml:"summarize"`
	Provider  string `toml:"provider"` // anthropic | gemini | openrouter
	Model     string `toml:"model"`
	Effort    string `toml:"effort"`             // low | medium | high (anthropic only)
	BaseURL   string `toml:"base_url,omitempty"` // optional override for OpenAI-compatible providers
	// MaxAge only includes items published within this window (Go duration, e.g.
	// "24h", "48h"). Items without a publish date aren't age-filtered. Empty or
	// "0" disables the filter. Defaults to "24h" for a daily run.
	MaxAge string `toml:"max_age"`
	// QuestionWhenEmpty sends an AI-generated learning question (SE fact, code
	// smell, data structure/algorithm, system design, …) when a run finds no new
	// items, so a daily notification always goes out.
	QuestionWhenEmpty bool `toml:"question_when_empty"`
	// QuestionPrompt overrides the system prompt used to generate that question.
	// Empty uses the built-in default (digest.DefaultQuestionPrompt).
	QuestionPrompt string `toml:"question_prompt,omitempty"`
}

// MaxAgeDuration parses MaxAge into a duration; returns 0 (no filter) when unset,
// "0", or unparseable.
func (d Digest) MaxAgeDuration() time.Duration {
	if d.MaxAge == "" || d.MaxAge == "0" {
		return 0
	}
	dur, err := time.ParseDuration(d.MaxAge)
	if err != nil || dur < 0 {
		return 0
	}
	return dur
}

// ResolvedProvider returns the configured provider, defaulting to anthropic.
func (d Digest) ResolvedProvider() string {
	if d.Provider == "" {
		return ProviderAnthropic
	}
	return d.Provider
}

// ResolvedModel returns the configured model, falling back to the provider's
// default when unset.
func (d Digest) ResolvedModel() string {
	if d.Model != "" {
		return d.Model
	}
	return DefaultModel(d.ResolvedProvider())
}

// Schedule controls the daily cron entry that `dev-digest cron install` writes.
type Schedule struct {
	// DailyTime is the local time of day to run, "HH:MM" (24-hour). Converted to
	// a cron expression. Defaults to "08:00".
	DailyTime string `toml:"daily_time"`
}

// ResolvedDailyTime returns the configured time or the "08:00" default.
func (s Schedule) ResolvedDailyTime() string {
	if strings.TrimSpace(s.DailyTime) == "" {
		return "08:00"
	}
	return strings.TrimSpace(s.DailyTime)
}

// CronExpr converts DailyTime into a cron expression "M H * * *" (every day at
// that minute/hour). It errors if the time isn't a valid HH:MM.
func (s Schedule) CronExpr() (string, error) {
	t := s.ResolvedDailyTime()
	parts := strings.SplitN(t, ":", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("time must be HH:MM, got %q", t)
	}
	h, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || h < 0 || h > 23 {
		return "", fmt.Errorf("hour must be 0-23, got %q", parts[0])
	}
	m, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || m < 0 || m > 59 {
		return "", fmt.Errorf("minute must be 0-59, got %q", parts[1])
	}
	return fmt.Sprintf("%d %d * * *", m, h), nil
}

// Source is a single content source. Fields used depend on Type.
type Source struct {
	Type string `toml:"type"` // rss | github | webpage
	Name string `toml:"name"`

	// rss, webpage
	URL string `toml:"url,omitempty"`

	// github
	Repo string `toml:"repo,omitempty"` // owner/name
	Kind string `toml:"kind,omitempty"` // releases | tags

	// webpage
	Selector string `toml:"selector,omitempty"` // optional CSS selector
}

// Delivery groups the output channels.
type Delivery struct {
	File    FileDelivery    `toml:"file"`
	Email   EmailDelivery   `toml:"email"`
	Webhook WebhookDelivery `toml:"webhook"`
}

// FileDelivery writes rendered digests to disk.
type FileDelivery struct {
	Enabled bool     `toml:"enabled"`
	Dir     string   `toml:"dir"`
	Formats []string `toml:"formats"` // "md", "html"
}

// EmailDelivery sends the HTML digest over SMTP. Password may be set here (the
// config file is written 0600) or via DEV_DIGEST_SMTP_PASSWORD.
type EmailDelivery struct {
	Enabled  bool     `toml:"enabled"`
	SMTPHost string   `toml:"smtp_host"`
	SMTPPort int      `toml:"smtp_port"`
	Username string   `toml:"username"`
	Password string   `toml:"password,omitempty"`
	From     string   `toml:"from"`
	To       []string `toml:"to"`
}

// WebhookDelivery posts the Markdown digest to a chat webhook.
type WebhookDelivery struct {
	Enabled bool   `toml:"enabled"`
	Kind    string `toml:"kind"` // slack | discord | generic
	URL     string `toml:"url"`
}

// Default returns a config with sensible starting values and no sources.
func Default() Config {
	return Config{
		Digest: Digest{
			Title:             "Dev Digest",
			Summarize:         true,
			Provider:          ProviderAnthropic,
			Model:             "claude-opus-4-8",
			Effort:            "medium",
			MaxAge:            "24h",
			QuestionWhenEmpty: true,
		},
		Schedule: Schedule{DailyTime: "08:00"},
		Sources:  nil,
		Delivery: Delivery{
			File: FileDelivery{
				Enabled: true,
				Dir:     "./out",
				Formats: []string{"md", "html"},
			},
			Email:   EmailDelivery{Enabled: false, SMTPPort: 587},
			Webhook: WebhookDelivery{Enabled: false, Kind: WebhookSlack},
		},
	}
}

// DefaultPath returns the default config file location
// (~/.config/dev-digest/config.toml), honoring XDG_CONFIG_HOME.
func DefaultPath() (string, error) {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "dev-digest", "config.toml"), nil
}

// Load reads and parses the config at path. If the file does not exist, it
// returns Default() and reports exists=false so callers can decide whether to
// write a starter file.
func Load(path string) (cfg Config, exists bool, err error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Default(), false, nil
	}
	if err != nil {
		return Config{}, false, fmt.Errorf("read config: %w", err)
	}
	cfg = Default()
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return Config{}, true, fmt.Errorf("parse config %s: %w", path, err)
	}
	return cfg, true, nil
}

// Save writes cfg to path atomically, creating parent directories as needed.
func Save(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	tmp := path + ".tmp"
	// 0600: the config may contain provider API keys.
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("replace config: %w", err)
	}
	return nil
}

// Validate checks the digest provider and that each source has the fields its
// type requires.
func (c Config) Validate() error {
	switch c.Digest.Provider {
	case "", ProviderAnthropic, ProviderGemini, ProviderOpenRouter:
		// ok
	default:
		return fmt.Errorf("digest: unknown provider %q (want anthropic, gemini, or openrouter)", c.Digest.Provider)
	}

	if ma := c.Digest.MaxAge; ma != "" && ma != "0" {
		if _, err := time.ParseDuration(ma); err != nil {
			return fmt.Errorf("digest: invalid max_age %q (want a Go duration like \"24h\"): %w", ma, err)
		}
	}

	if _, err := c.Schedule.CronExpr(); err != nil {
		return fmt.Errorf("schedule: %w", err)
	}

	for i, s := range c.Sources {
		if s.Name == "" {
			return fmt.Errorf("source #%d: name is required", i+1)
		}
		switch s.Type {
		case SourceRSS:
			if s.URL == "" {
				return fmt.Errorf("source %q: rss requires url", s.Name)
			}
		case SourceGitHub:
			if s.Repo == "" {
				return fmt.Errorf("source %q: github requires repo (owner/name)", s.Name)
			}
			if s.Kind != "" && s.Kind != GitHubReleases && s.Kind != GitHubTags {
				return fmt.Errorf("source %q: github kind must be releases or tags", s.Name)
			}
		case SourceWebpage:
			if s.URL == "" {
				return fmt.Errorf("source %q: webpage requires url", s.Name)
			}
		default:
			return fmt.Errorf("source %q: unknown type %q", s.Name, s.Type)
		}
	}
	return nil
}
