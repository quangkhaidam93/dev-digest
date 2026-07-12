package deliver

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/quangkhaidam93/dev-digest/internal/config"
	"github.com/quangkhaidam93/dev-digest/internal/digest"
)

func testDigest() digest.Digest {
	return digest.Digest{
		Title: "Dev Digest",
		Date:  time.Date(2026, 7, 11, 8, 0, 0, 0, time.UTC),
		Intro: "Today's issue: testing.",
		Sections: []digest.Section{{
			Title: "news",
			Items: []digest.Item{{Headline: "H", Summary: "S", URL: "https://x", Source: "src"}},
		}},
	}
}

func TestDeliverFile(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Delivery{File: config.FileDelivery{Enabled: true, Dir: dir, Formats: []string{"md", "html"}}}

	results := Deliver(context.Background(), cfg, testDigest())
	if len(results) != 1 || results[0].Err != nil {
		t.Fatalf("file delivery: %+v", results)
	}
	for _, ext := range []string{"md", "html"} {
		p := filepath.Join(dir, "2026-07-11."+ext)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("missing %s: %v", p, err)
		}
	}
}

func TestDeliverWebhookSlack(t *testing.T) {
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := config.Delivery{Webhook: config.WebhookDelivery{Enabled: true, Kind: config.WebhookSlack, URL: srv.URL}}
	results := Deliver(context.Background(), cfg, testDigest())
	if len(results) != 1 || results[0].Err != nil {
		t.Fatalf("webhook delivery: %+v", results)
	}
	if _, ok := gotBody["text"]; !ok {
		t.Errorf("slack payload missing 'text' key: %+v", gotBody)
	}
}

func TestDeliverWebhookDiscordKey(t *testing.T) {
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
	}))
	defer srv.Close()

	cfg := config.Delivery{Webhook: config.WebhookDelivery{Enabled: true, Kind: config.WebhookDiscord, URL: srv.URL}}
	Deliver(context.Background(), cfg, testDigest())
	if _, ok := gotBody["content"]; !ok {
		t.Errorf("discord payload missing 'content' key: %+v", gotBody)
	}
}

// With a username but no password (config or env), email delivery fails with a
// clear error before ever dialing.
func TestEmailMissingPassword(t *testing.T) {
	t.Setenv("DEV_DIGEST_SMTP_PASSWORD", "")
	cfg := config.Delivery{Email: config.EmailDelivery{
		Enabled: true, SMTPHost: "localhost", SMTPPort: 2525,
		Username: "u", From: "f@x.com", To: []string{"t@x.com"},
	}}
	results := Deliver(context.Background(), cfg, testDigest())
	if len(results) != 1 || results[0].Err == nil {
		t.Fatalf("expected email error, got %+v", results)
	}
	if got := results[0].Err.Error(); !strings.Contains(got, "password") {
		t.Errorf("error = %q, want it to mention the missing password", got)
	}
}

func TestDeliverIndependentChannels(t *testing.T) {
	// File delivery succeeds; webhook points at a dead URL and fails. Both
	// results are returned, and the file still lands.
	dir := t.TempDir()
	cfg := config.Delivery{
		File:    config.FileDelivery{Enabled: true, Dir: dir, Formats: []string{"md"}},
		Webhook: config.WebhookDelivery{Enabled: true, Kind: config.WebhookGeneric, URL: "http://127.0.0.1:0/nope"},
	}
	results := Deliver(context.Background(), cfg, testDigest())
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	byChannel := map[string]error{}
	for _, r := range results {
		byChannel[r.Channel] = r.Err
	}
	if byChannel["file"] != nil {
		t.Errorf("file should have succeeded: %v", byChannel["file"])
	}
	if byChannel["webhook"] == nil {
		t.Error("webhook to dead URL should have failed")
	}
	if _, err := os.Stat(filepath.Join(dir, "2026-07-11.md")); err != nil {
		t.Errorf("file not written despite webhook failure: %v", err)
	}
}
