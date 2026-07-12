package digest

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/quangkhaidam93/dev-digest/internal/config"
	"github.com/quangkhaidam93/dev-digest/internal/sources"
)

const systemPrompt = `You are the editor of a developer newsletter in the style of "Bytes" — sharp, witty, and genuinely useful.
You are given a list of new items collected from RSS feeds, GitHub releases, and dev pages.
Write:
- "intro": one short, punchy paragraph in the voice "Today's issue: …" that teases the 2-3 most interesting items with light humor. One or two sentences.
- "sections": group the items into a few themed sections with concise, lowercase-ish punchy titles. For each item write a "headline" (rewrite the raw title to be crisp) and a 1-2 sentence "summary" that tells the reader why it matters. Keep the original "url" and "source".
Do not invent items, facts, or URLs. Only use what you are given. Keep summaries tight and skimmable.
Respond with a single JSON object matching the requested schema and nothing else.`

// completer performs one LLM completion: given a system and user prompt, it
// returns the model's raw text response (expected to be JSON matching
// digestSchema). Each provider implements it.
type completer interface {
	// complete sends the system + user prompts constrained to schema and returns
	// the model's raw text (expected JSON matching schema).
	complete(ctx context.Context, system, user string, schema map[string]any) (string, error)
	label() string
}

// Summarize builds a structured Digest from the new items using the configured
// provider (anthropic, gemini, or openrouter). apiKey is the resolved provider
// key (may be empty for anthropic, which falls back to the SDK's own
// resolution). The response is constrained to a JSON schema so rendering is
// deterministic.
func Summarize(ctx context.Context, cfg config.Digest, apiKey string, now time.Time, items []sources.Item) (Digest, error) {
	c, err := newCompleter(cfg, apiKey)
	if err != nil {
		return Digest{}, err
	}

	raw, err := c.complete(ctx, systemPrompt, buildPrompt(items), digestSchema)
	if err != nil {
		return Digest{}, fmt.Errorf("%s: %w", c.label(), err)
	}

	return parseDigest(raw, cfg.Title, now)
}

// parseDigest parses the model's JSON output into a Digest, tolerating markdown
// code fences some models wrap around JSON.
func parseDigest(raw, title string, now time.Time) (Digest, error) {
	raw = stripCodeFence(strings.TrimSpace(raw))
	if raw == "" {
		return Digest{}, fmt.Errorf("model returned no output")
	}

	var out struct {
		Intro    string    `json:"intro"`
		Sections []Section `json:"sections"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return Digest{}, fmt.Errorf("parse structured output: %w", err)
	}
	return Digest{
		Title:    title,
		Date:     now,
		Intro:    out.Intro,
		Sections: out.Sections,
	}, nil
}

// stripCodeFence removes a surrounding ```json … ``` (or ``` … ```) fence.
func stripCodeFence(s string) string {
	if !strings.HasPrefix(s, "```") {
		return s
	}
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimPrefix(s, "json")
	s = strings.TrimPrefix(s, "\n")
	if i := strings.LastIndex(s, "```"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

func buildPrompt(items []sources.Item) string {
	var b strings.Builder
	b.WriteString("Here are the new items. Summarize them into the newsletter.\n\n")
	for i, it := range items {
		fmt.Fprintf(&b, "%d. Source: %s\n   Title: %s\n   URL: %s\n", i+1, it.SourceName, it.Title, it.URL)
		if !it.Published.IsZero() {
			fmt.Fprintf(&b, "   Published: %s\n", it.Published.Format(time.RFC3339))
		}
		if it.Excerpt != "" {
			fmt.Fprintf(&b, "   Content: %s\n", it.Excerpt)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// digestSchema constrains the model's output. It satisfies both Anthropic's and
// OpenAI's strict structured-output rules (additionalProperties:false and
// required on every object).
var digestSchema = map[string]any{
	"type":                 "object",
	"additionalProperties": false,
	"required":             []string{"intro", "sections"},
	"properties": map[string]any{
		"intro": map[string]any{"type": "string"},
		"sections": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"title", "items"},
				"properties": map[string]any{
					"title": map[string]any{"type": "string"},
					"items": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type":                 "object",
							"additionalProperties": false,
							"required":             []string{"headline", "summary", "url", "source"},
							"properties": map[string]any{
								"headline": map[string]any{"type": "string"},
								"summary":  map[string]any{"type": "string"},
								"url":      map[string]any{"type": "string"},
								"source":   map[string]any{"type": "string"},
							},
						},
					},
				},
			},
		},
	},
}
