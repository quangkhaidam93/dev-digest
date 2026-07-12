package digest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/quangkhaidam93/dev-digest/internal/config"
)

// captureSystemPrompt starts a mock OpenAI-compatible server that records the
// system prompt it receives and returns a valid question.
func captureSystemPrompt(t *testing.T, got *string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []struct {
				Role, Content string
			} `json:"messages"`
		}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &body)
		for _, msg := range body.Messages {
			if msg.Role == "system" {
				*got = msg.Content
			}
		}
		io.WriteString(w, `{"choices":[{"message":{"content":"{\"topic\":\"t\",\"question\":\"q\",\"answer\":\"a\"}"}}]}`)
	}))
}

func TestQuestionUsesCustomPrompt(t *testing.T) {
	var gotSystem string
	srv := captureSystemPrompt(t, &gotSystem)
	defer srv.Close()

	cfg := config.Digest{
		Provider:       config.ProviderOpenRouter,
		Model:          "x",
		BaseURL:        srv.URL,
		QuestionPrompt: "MY CUSTOM PROMPT",
	}
	d := GenerateQuestion(context.Background(), cfg, "key", time.Now())
	if gotSystem != "MY CUSTOM PROMPT" {
		t.Errorf("system prompt = %q, want the custom prompt", gotSystem)
	}
	if d.Empty() {
		t.Error("digest should not be empty")
	}
}

func TestQuestionUsesDefaultPromptWhenEmpty(t *testing.T) {
	var gotSystem string
	srv := captureSystemPrompt(t, &gotSystem)
	defer srv.Close()

	cfg := config.Digest{Provider: config.ProviderOpenRouter, Model: "x", BaseURL: srv.URL}
	GenerateQuestion(context.Background(), cfg, "key", time.Now())
	if gotSystem != DefaultQuestionPrompt {
		t.Errorf("empty QuestionPrompt should use the default; got %q", gotSystem)
	}
}

// When the LLM is unavailable (no key for a non-anthropic provider), a built-in
// question is used so a notification still goes out.
func TestGenerateQuestionFallback(t *testing.T) {
	cfg := config.Digest{Provider: config.ProviderGemini, Title: "Dev Digest"} // no key
	d := GenerateQuestion(context.Background(), cfg, "", time.Date(2026, 7, 12, 8, 0, 0, 0, time.UTC))

	if d.Empty() {
		t.Fatal("fallback question digest should not be empty")
	}
	if len(d.Sections) != 1 || len(d.Sections[0].Items) != 1 {
		t.Fatalf("expected one section/item, got %+v", d.Sections)
	}
	it := d.Sections[0].Items[0]
	if it.Headline == "" || it.Summary == "" {
		t.Errorf("question/answer should be populated: %+v", it)
	}
	if it.URL != "" {
		t.Errorf("question item should have no URL, got %q", it.URL)
	}
}

func TestFallbackQuestionRotatesInRange(t *testing.T) {
	seen := map[string]bool{}
	for day := 1; day <= 366; day++ {
		q := fallbackQuestions[day%len(fallbackQuestions)]
		seen[q.Topic] = true
	}
	if len(seen) < 2 {
		t.Errorf("fallback pool should rotate across topics, saw %d", len(seen))
	}
}

// A URL-less question item must render cleanly (no empty markdown link "()").
func TestQuestionDigestRendersCleanly(t *testing.T) {
	q := question{Topic: "System design", Question: "Why use a queue?", Answer: "To decouple services."}
	d := questionDigest("Dev Digest", time.Now(), q)

	md, err := d.RenderMarkdown()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(md, "]()") || strings.Contains(md, "[Why use a queue?](") {
		t.Errorf("URL-less item should not render a link:\n%s", md)
	}
	if !strings.Contains(md, "Why use a queue?") || !strings.Contains(md, "To decouple services.") {
		t.Errorf("question/answer missing from markdown:\n%s", md)
	}
	if !strings.Contains(md, "Question of the day · System design") {
		t.Errorf("topic heading missing:\n%s", md)
	}

	html, err := d.RenderHTML()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(html, `href=""`) {
		t.Errorf("URL-less item should not render an empty anchor")
	}
	if !strings.Contains(html, "Why use a queue?") {
		t.Errorf("question missing from html")
	}
}

// Items that DO have a URL still render as links (regression guard).
func TestItemWithURLStillLinks(t *testing.T) {
	md, err := sampleDigest().RenderMarkdown()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(md, "[React 19.2 RC](https://example.com/react)") {
		t.Errorf("dated item should still render a link:\n%s", md)
	}
}
