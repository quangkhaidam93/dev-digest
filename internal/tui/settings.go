package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/quangkhaidam93/dev-digest/internal/config"
)

// setField identifies a focusable row on the settings screen.
type setField int

const (
	setSummarize setField = iota
	setProvider
	setModel
	setAPIKey
	setEffort
	setMaxAge
	setQuestion
	setQuestionPrompt
	setFile
	setEmail
	setEmailHost
	setEmailPort
	setEmailUser
	setEmailPass
	setEmailFrom
	setEmailTo
	setWebhook
	setWebhookKind
	setWebhookURL
	setDailyTime
)

var (
	effortCycle  = []string{"low", "medium", "high"}
	webhookKinds = []string{config.WebhookSlack, config.WebhookDiscord, config.WebhookGeneric}

	// textFields are edited via a textinput; everything else is toggled/cycled.
	textFields = map[setField]bool{
		setModel: true, setAPIKey: true, setMaxAge: true,
		setEmailHost: true, setEmailPort: true, setEmailUser: true,
		setEmailPass: true, setEmailFrom: true, setEmailTo: true,
		setWebhookURL: true, setDailyTime: true,
	}
	maskedFields = map[setField]bool{setAPIKey: true, setEmailPass: true}
	// subFields render indented beneath their channel toggle.
	subFields = map[setField]bool{
		setEmailHost: true, setEmailPort: true, setEmailUser: true,
		setEmailPass: true, setEmailFrom: true, setEmailTo: true,
		setWebhookKind: true, setWebhookURL: true,
	}
)

// settingsModel holds the editable text inputs; toggles/cycles read/write m.cfg.
type settingsModel struct {
	focus setField
	ti    map[setField]textinput.Model
}

func newSettings(cfg config.Config) settingsModel {
	s := settingsModel{focus: setSummarize, ti: map[setField]textinput.Model{}}
	mk := func(f setField, placeholder, val string) {
		ti := textinput.New()
		ti.Placeholder = placeholder
		ti.Prompt = "› "
		ti.SetValue(val)
		if maskedFields[f] {
			ti.EchoMode = textinput.EchoPassword
			ti.EchoCharacter = '•'
		}
		s.ti[f] = ti
	}
	e := cfg.Delivery.Email
	mk(setModel, "model id", cfg.Digest.ResolvedModel())
	mk(setAPIKey, "API key (stored in config)", rawKey(cfg, cfg.Digest.ResolvedProvider()))
	mk(setMaxAge, "e.g. 24h, 48h — empty/0 disables", cfg.Digest.MaxAge)
	mk(setEmailHost, "smtp.example.com", e.SMTPHost)
	mk(setEmailPort, "587", portStr(e.SMTPPort))
	mk(setEmailUser, "username (blank = no auth)", e.Username)
	mk(setEmailPass, "SMTP password", e.Password)
	mk(setEmailFrom, "from address", e.From)
	mk(setEmailTo, "comma-separated recipients", strings.Join(e.To, ", "))
	mk(setWebhookURL, "https://hooks.slack.com/…", cfg.Delivery.Webhook.URL)
	mk(setDailyTime, "HH:MM (24h)", cfg.Schedule.ResolvedDailyTime())
	return s
}

func portStr(n int) string {
	if n == 0 {
		return ""
	}
	return strconv.Itoa(n)
}

func rawKey(cfg config.Config, provider string) string {
	if cfg.Keys == nil {
		return ""
	}
	return cfg.Keys[provider]
}

// visibleRows returns the focus order, revealing a channel's fields only when
// that channel is enabled.
func (m model) visibleRows() []setField {
	rows := []setField{setSummarize, setProvider, setModel, setAPIKey, setEffort, setMaxAge, setQuestion, setQuestionPrompt, setFile, setEmail}
	if m.cfg.Delivery.Email.Enabled {
		rows = append(rows, setEmailHost, setEmailPort, setEmailUser, setEmailPass, setEmailFrom, setEmailTo)
	}
	rows = append(rows, setWebhook)
	if m.cfg.Delivery.Webhook.Enabled {
		rows = append(rows, setWebhookKind, setWebhookURL)
	}
	rows = append(rows, setDailyTime)
	return rows
}

func (s *settingsModel) syncFocus() {
	for f, ti := range s.ti {
		ti.Blur()
		s.ti[f] = ti
	}
	if textFields[s.focus] {
		ti := s.ti[s.focus]
		ti.Focus()
		s.ti[s.focus] = ti
	}
}

func (m model) nextSet(dir int) setField {
	rows := m.visibleRows()
	cur := 0
	for i, f := range rows {
		if f == m.settings.focus {
			cur = i
			break
		}
	}
	cur = (cur + dir + len(rows)) % len(rows)
	return rows[cur]
}

func (m model) updateSettings(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, isKey := msg.(tea.KeyMsg); isKey {
		switch key.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			m.syncTextIntoCfg()
			if err := m.validateSettings(); err != nil {
				m.setStatus(err.Error(), true)
				return m, nil // stay so the user can fix it
			}
			m.save()
			m.screen = screenSources
			return m, nil
		case "up", "shift+tab":
			m.settings.focus = m.nextSet(-1)
			m.settings.syncFocus()
			return m, nil
		case "down", "tab":
			m.settings.focus = m.nextSet(1)
			m.settings.syncFocus()
			return m, nil
		}

		// The prompt row opens a full-screen editor rather than toggling.
		if m.settings.focus == setQuestionPrompt {
			switch key.String() {
			case "enter", " ", "right":
				m.openPromptEditor()
				return m, nil
			}
			return m, nil
		}

		if !textFields[m.settings.focus] {
			switch key.String() {
			case "enter", " ", "left", "right":
				m.toggleSetting(key.String() == "left")
				return m, nil
			}
			return m, nil
		}
	}

	// Route to the focused text input, then mirror into cfg.
	if textFields[m.settings.focus] {
		ti := m.settings.ti[m.settings.focus]
		var cmd tea.Cmd
		ti, cmd = ti.Update(msg)
		m.settings.ti[m.settings.focus] = ti
		m.syncTextIntoCfg()
		return m, cmd
	}
	return m, nil
}

func (m *model) val(f setField) string { return strings.TrimSpace(m.settings.ti[f].Value()) }

// syncTextIntoCfg copies the text inputs into cfg (in memory).
func (m *model) syncTextIntoCfg() {
	m.cfg.Digest.Model = m.val(setModel)
	m.cfg.Digest.MaxAge = m.val(setMaxAge)
	m.cfg.SetKey(m.cfg.Digest.ResolvedProvider(), m.settings.ti[setAPIKey].Value())

	e := &m.cfg.Delivery.Email
	e.SMTPHost = m.val(setEmailHost)
	if p := m.val(setEmailPort); p == "" {
		e.SMTPPort = 0
	} else if n, err := strconv.Atoi(p); err == nil {
		e.SMTPPort = n
	}
	e.Username = m.val(setEmailUser)
	e.Password = m.settings.ti[setEmailPass].Value()
	e.From = m.val(setEmailFrom)
	e.To = splitList(m.val(setEmailTo))

	m.cfg.Delivery.Webhook.URL = m.val(setWebhookURL)

	m.cfg.Schedule.DailyTime = m.val(setDailyTime)
}

// validateSettings checks provider/max_age plus the fields required by any
// enabled delivery channel.
func (m model) validateSettings() error {
	if err := m.cfg.Validate(); err != nil {
		return err
	}
	if m.cfg.Delivery.Email.Enabled {
		if m.cfg.Delivery.Email.SMTPHost == "" {
			return fmt.Errorf("email: SMTP host is required")
		}
		if len(m.cfg.Delivery.Email.To) == 0 {
			return fmt.Errorf("email: at least one recipient is required")
		}
		if p := m.val(setEmailPort); p != "" {
			if _, err := strconv.Atoi(p); err != nil {
				return fmt.Errorf("email: port must be a number")
			}
		}
	}
	if m.cfg.Delivery.Webhook.Enabled && m.cfg.Delivery.Webhook.URL == "" {
		return fmt.Errorf("webhook: URL is required")
	}
	return nil
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func (m *model) toggleSetting(backward bool) {
	fwd := !backward
	switch m.settings.focus {
	case setSummarize:
		m.cfg.Digest.Summarize = !m.cfg.Digest.Summarize
	case setProvider:
		m.syncTextIntoCfg() // preserve current provider's key/model before switching
		next := cycle(config.Providers, m.cfg.Digest.ResolvedProvider(), fwd)
		m.cfg.Digest.Provider = next
		m.cfg.Digest.Model = config.DefaultModel(next)
		m.setInput(setModel, m.cfg.Digest.Model)
		m.setInput(setAPIKey, rawKey(m.cfg, next))
	case setEffort:
		m.cfg.Digest.Effort = cycle(effortCycle, orDefault(m.cfg.Digest.Effort, "medium"), fwd)
	case setQuestion:
		m.cfg.Digest.QuestionWhenEmpty = !m.cfg.Digest.QuestionWhenEmpty
	case setFile:
		m.cfg.Delivery.File.Enabled = !m.cfg.Delivery.File.Enabled
	case setEmail:
		m.cfg.Delivery.Email.Enabled = !m.cfg.Delivery.Email.Enabled
	case setWebhook:
		m.cfg.Delivery.Webhook.Enabled = !m.cfg.Delivery.Webhook.Enabled
	case setWebhookKind:
		m.cfg.Delivery.Webhook.Kind = cycle(webhookKinds, orDefault(m.cfg.Delivery.Webhook.Kind, config.WebhookSlack), fwd)
	}
	m.save()
}

func (m *model) setInput(f setField, val string) {
	ti := m.settings.ti[f]
	ti.SetValue(val)
	m.settings.ti[f] = ti
}

var (
	groupStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#c05621")).MarginTop(1)
	onStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#2f855a"))
	offStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#8a8577"))
)

func (m model) viewSettings() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Settings") + "\n")
	for _, f := range m.visibleRows() {
		switch f {
		case setSummarize:
			b.WriteString(groupStyle.Render("Summarization") + "\n")
		case setFile:
			b.WriteString(groupStyle.Render("Delivery") + "\n")
		case setDailyTime:
			b.WriteString(groupStyle.Render("Schedule") + "\n")
		}
		b.WriteString(m.renderSetRow(f) + "\n")
	}

	provider := m.cfg.Digest.ResolvedProvider()
	b.WriteString(helpStyle.Render("\n↑↓ move · space/←→ toggle · type to edit fields · esc save & back") + "\n")
	b.WriteString(helpStyle.Render("Secrets are stored in config (chmod 0600); $" + config.APIKeyEnv(provider) + " / $DEV_DIGEST_SMTP_PASSWORD also work."))
	if s := m.renderStatus(); s != "" {
		b.WriteString("\n" + s)
	}
	return b.String()
}

func (m model) renderSetRow(f setField) string {
	label, value := m.setRowContent(f)

	indent := "  "
	if subFields[f] {
		indent = "      "
	}
	marker := "  "
	labelStyle := lipgloss.NewStyle().Width(18)
	if m.settings.focus == f {
		marker = lipgloss.NewStyle().Foreground(lipgloss.Color("#c05621")).Render("➤ ")
		labelStyle = labelStyle.Bold(true)
	}
	return indent + marker + labelStyle.Render(label) + value
}

func (m model) setRowContent(f setField) (label, value string) {
	onOff := func(b bool) string {
		if b {
			return onStyle.Render("on")
		}
		return offStyle.Render("off")
	}
	if textFields[f] {
		view := m.settings.ti[f].View()
		switch f {
		case setModel:
			return "Model", view
		case setAPIKey:
			return "API key", view
		case setMaxAge:
			return "Max age", view
		case setEmailHost:
			return "SMTP host", view
		case setEmailPort:
			return "SMTP port", view
		case setEmailUser:
			return "Username", view
		case setEmailPass:
			return "Password", view
		case setEmailFrom:
			return "From", view
		case setEmailTo:
			return "Recipients", view
		case setWebhookURL:
			return "URL", view
		case setDailyTime:
			suffix := "  (invalid time)"
			if expr, err := (config.Schedule{DailyTime: m.val(setDailyTime)}).CronExpr(); err == nil {
				suffix = "  → cron: " + expr
			}
			return "Daily time (HH:MM)", view + helpStyle.Render(suffix)
		}
	}
	switch f {
	case setSummarize:
		return "Summarize", onOff(m.cfg.Digest.Summarize)
	case setProvider:
		return "Provider", m.cfg.Digest.ResolvedProvider()
	case setEffort:
		return "Effort (anthropic)", orDefault(m.cfg.Digest.Effort, "medium")
	case setQuestion:
		return "Question on empty days", onOff(m.cfg.Digest.QuestionWhenEmpty)
	case setQuestionPrompt:
		state := "default"
		if strings.TrimSpace(m.cfg.Digest.QuestionPrompt) != "" {
			state = "custom"
		}
		return "Question prompt", state + " · enter to edit"
	case setFile:
		return "File delivery", onOff(m.cfg.Delivery.File.Enabled)
	case setEmail:
		return "Email delivery", onOff(m.cfg.Delivery.Email.Enabled)
	case setWebhook:
		return "Webhook delivery", onOff(m.cfg.Delivery.Webhook.Enabled)
	case setWebhookKind:
		return "Kind", orDefault(m.cfg.Delivery.Webhook.Kind, config.WebhookSlack)
	}
	return "", ""
}
