// Package deliver sends a rendered digest to the configured output channels:
// file, email (SMTP), and chat webhook. Channels are independent — a failure in
// one is reported but does not stop the others.
package deliver

import (
	"context"

	"github.com/quangkhaidam93/dev-digest/internal/config"
	"github.com/quangkhaidam93/dev-digest/internal/digest"
)

// Result reports the outcome of one channel.
type Result struct {
	Channel string
	Err     error
}

// Deliver renders the digest and sends it to every enabled channel. It returns
// one Result per attempted channel (in file, email, webhook order).
func Deliver(ctx context.Context, cfg config.Delivery, d digest.Digest) []Result {
	md, mdErr := d.RenderMarkdown()
	html, htmlErr := d.RenderHTML()

	var results []Result

	if cfg.File.Enabled {
		err := firstErr(mdErr, htmlErr)
		if err == nil {
			err = deliverFile(cfg.File, d.Date, md, html)
		}
		results = append(results, Result{Channel: "file", Err: err})
	}

	if cfg.Email.Enabled {
		err := htmlErr
		if err == nil {
			err = deliverEmail(cfg.Email, d, html)
		}
		results = append(results, Result{Channel: "email", Err: err})
	}

	if cfg.Webhook.Enabled {
		err := mdErr
		if err == nil {
			err = deliverWebhook(ctx, cfg.Webhook, md)
		}
		results = append(results, Result{Channel: "webhook", Err: err})
	}

	return results
}

func firstErr(errs ...error) error {
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}

// subject returns the email/webhook subject line for the digest's date.
func subject(d digest.Digest) string {
	title := d.Title
	if title == "" {
		title = "Dev Digest"
	}
	return title + " — " + d.Date.Format("Jan 2")
}
