package deliver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/quangkhaidam93/dev-digest/internal/config"
)

// deliverWebhook posts the Markdown digest to a chat webhook. Slack and Discord
// both accept a JSON body with the message text under different keys; "generic"
// posts {"text": <markdown>}.
func deliverWebhook(ctx context.Context, cfg config.WebhookDelivery, md string) error {
	if cfg.URL == "" {
		return fmt.Errorf("webhook: url not configured")
	}

	var payload map[string]string
	switch cfg.Kind {
	case config.WebhookDiscord:
		payload = map[string]string{"content": md}
	case config.WebhookSlack, config.WebhookGeneric, "":
		payload = map[string]string{"text": md}
	default:
		return fmt.Errorf("webhook: unknown kind %q", cfg.Kind)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("webhook: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("webhook: post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("webhook: %s returned %s: %s", cfg.URL, resp.Status, bytes.TrimSpace(snippet))
	}
	return nil
}
