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

func TestNewCompleterSelection(t *testing.T) {
	tests := []struct {
		provider  string
		wantLabel string
	}{
		{config.ProviderAnthropic, "anthropic"},
		{config.ProviderGemini, "gemini"},
		{config.ProviderOpenRouter, "openrouter"},
		{"", "anthropic"}, // default
	}
	for _, tt := range tests {
		c, err := newCompleter(config.Digest{Provider: tt.provider}, "some-key")
		if err != nil {
			t.Fatalf("provider %q: %v", tt.provider, err)
		}
		if c.label() != tt.wantLabel {
			t.Errorf("provider %q: label = %q, want %q", tt.provider, c.label(), tt.wantLabel)
		}
	}
}

func TestNewCompleterMissingKey(t *testing.T) {
	// A non-anthropic provider with no resolved key is an error.
	if _, err := newCompleter(config.Digest{Provider: config.ProviderGemini}, ""); err == nil {
		t.Error("expected error when gemini has no API key")
	}
	// Anthropic tolerates an empty key (SDK resolves env/profile at call time).
	if _, err := newCompleter(config.Digest{Provider: config.ProviderAnthropic}, ""); err != nil {
		t.Errorf("anthropic with empty key should construct: %v", err)
	}
}

func TestOpenAICompleterRequestAndParse(t *testing.T) {
	var gotAuth, gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		// Return an OpenAI-shaped response whose content is our digest JSON.
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"choices":[{"message":{"content":"{\"intro\":\"hi\",\"sections\":[]}"}}]}`)
	}))
	defer srv.Close()

	c := &openAICompleter{name: "gemini", baseURL: srv.URL, apiKey: "secret", model: "gemini-2.5-flash"}
	raw, err := c.complete(context.Background(), "sys", "usr", digestSchema)
	if err != nil {
		t.Fatalf("complete: %v", err)
	}

	if gotAuth != "Bearer secret" {
		t.Errorf("auth header = %q", gotAuth)
	}
	if gotPath != "/chat/completions" {
		t.Errorf("path = %q", gotPath)
	}
	if gotBody["model"] != "gemini-2.5-flash" {
		t.Errorf("model in body = %v", gotBody["model"])
	}
	if _, ok := gotBody["response_format"]; !ok {
		t.Error("response_format missing from request body")
	}

	// The raw content parses into a Digest.
	d, err := parseDigest(raw, "T", time.Now())
	if err != nil {
		t.Fatalf("parseDigest: %v", err)
	}
	if d.Intro != "hi" {
		t.Errorf("intro = %q", d.Intro)
	}
}

func TestOpenAICompleterErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, `{"error":{"message":"invalid key"}}`)
	}))
	defer srv.Close()

	c := &openAICompleter{name: "openrouter", baseURL: srv.URL, apiKey: "bad", model: "x"}
	if _, err := c.complete(context.Background(), "s", "u", digestSchema); err == nil {
		t.Error("expected error on 401")
	}
}

func TestStripCodeFence(t *testing.T) {
	cases := map[string]string{
		"{\"a\":1}":                     "{\"a\":1}",
		"```json\n{\"a\":1}\n```":       "{\"a\":1}",
		"```\n{\"a\":1}\n```":           "{\"a\":1}",
		"  ```json\n{\"a\":1}\n```  \n": "{\"a\":1}",
	}
	for in, want := range cases {
		// parseDigest trims before stripping; mirror that ordering here.
		if got := stripCodeFence(strings.TrimSpace(in)); got != want {
			t.Errorf("stripCodeFence(%q) = %q, want %q", in, got, want)
		}
	}
}
