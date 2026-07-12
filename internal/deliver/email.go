package deliver

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"os"
	"strconv"
	"strings"

	"github.com/quangkhaidam93/dev-digest/internal/config"
	"github.com/quangkhaidam93/dev-digest/internal/digest"
)

// smtpPasswordEnv is the environment variable holding the SMTP password.
const smtpPasswordEnv = "DEV_DIGEST_SMTP_PASSWORD"

// deliverEmail sends the HTML digest over SMTP using STARTTLS. The password is
// read from DEV_DIGEST_SMTP_PASSWORD; if a username is configured the
// connection authenticates with PLAIN auth.
func deliverEmail(cfg config.EmailDelivery, d digest.Digest, html string) error {
	if cfg.SMTPHost == "" {
		return fmt.Errorf("email: smtp_host not configured")
	}
	if len(cfg.To) == 0 {
		return fmt.Errorf("email: no recipients configured")
	}
	from := cfg.From
	if from == "" {
		from = cfg.Username
	}
	if from == "" {
		return fmt.Errorf("email: from address not configured")
	}

	port := cfg.SMTPPort
	if port == 0 {
		port = 587
	}
	addr := net.JoinHostPort(cfg.SMTPHost, strconv.Itoa(port))

	msg := buildMIME(from, cfg.To, subject(d), html)

	var auth smtp.Auth
	if cfg.Username != "" {
		pass := cfg.Password
		if pass == "" {
			pass = os.Getenv(smtpPasswordEnv)
		}
		if pass == "" {
			return fmt.Errorf("email: no password (set it in settings or $%s)", smtpPasswordEnv)
		}
		auth = smtp.PlainAuth("", cfg.Username, pass, cfg.SMTPHost)
	}

	return sendSTARTTLS(addr, cfg.SMTPHost, auth, from, cfg.To, msg)
}

func buildMIME(from string, to []string, subj, html string) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", strings.Join(to, ", "))
	fmt.Fprintf(&b, "Subject: %s\r\n", subj)
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	b.WriteString("\r\n")
	b.WriteString(html)
	return []byte(b.String())
}

// sendSTARTTLS connects, upgrades to TLS via STARTTLS when the server offers
// it, authenticates if auth is non-nil, and sends the message.
func sendSTARTTLS(addr, host string, auth smtp.Auth, from string, to []string, msg []byte) error {
	c, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("email: dial %s: %w", addr, err)
	}
	defer c.Close()

	if ok, _ := c.Extension("STARTTLS"); ok {
		if err := c.StartTLS(&tls.Config{ServerName: host}); err != nil {
			return fmt.Errorf("email: starttls: %w", err)
		}
	}
	if auth != nil {
		if ok, _ := c.Extension("AUTH"); ok {
			if err := c.Auth(auth); err != nil {
				return fmt.Errorf("email: auth: %w", err)
			}
		}
	}
	if err := c.Mail(from); err != nil {
		return fmt.Errorf("email: MAIL FROM: %w", err)
	}
	for _, rcpt := range to {
		if err := c.Rcpt(rcpt); err != nil {
			return fmt.Errorf("email: RCPT TO %s: %w", rcpt, err)
		}
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("email: DATA: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("email: write body: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("email: close body: %w", err)
	}
	return c.Quit()
}
