package digest

import (
	"fmt"

	"github.com/quangkhaidam93/dev-digest/internal/config"
)

// Base URLs for the OpenAI-compatible providers.
const (
	geminiBaseURL     = "https://generativelanguage.googleapis.com/v1beta/openai"
	openRouterBaseURL = "https://openrouter.ai/api/v1"
)

// newCompleter selects and configures the completer for the digest's provider.
// apiKey is the resolved key (config value or env fallback), which the caller
// supplies; a missing key for a provider that requires one is a clear error.
func newCompleter(cfg config.Digest, apiKey string) (completer, error) {
	provider := cfg.ResolvedProvider()
	model := cfg.ResolvedModel()
	if model == "" {
		return nil, fmt.Errorf("%s: no model configured (set [digest].model)", provider)
	}

	switch provider {
	case config.ProviderAnthropic:
		// apiKey may be empty; the Anthropic SDK then resolves env or ant profile.
		return &anthropicCompleter{model: model, effort: cfg.Effort, apiKey: apiKey}, nil

	case config.ProviderGemini:
		if apiKey == "" {
			return nil, fmt.Errorf("gemini: no API key (set it in settings or $GEMINI_API_KEY)")
		}
		return &openAICompleter{
			name:    "gemini",
			baseURL: orDefault(cfg.BaseURL, geminiBaseURL),
			apiKey:  apiKey,
			model:   model,
		}, nil

	case config.ProviderOpenRouter:
		if apiKey == "" {
			return nil, fmt.Errorf("openrouter: no API key (set it in settings or $OPENROUTER_API_KEY)")
		}
		return &openAICompleter{
			name:    "openrouter",
			baseURL: orDefault(cfg.BaseURL, openRouterBaseURL),
			apiKey:  apiKey,
			model:   model,
			// OpenRouter recommends (optional) attribution headers.
			extraHeaders: map[string]string{
				"HTTP-Referer": "https://github.com/quangkhaidam93/dev-digest",
				"X-Title":      "dev-digest",
			},
		}, nil

	default:
		return nil, fmt.Errorf("unknown provider %q", provider)
	}
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
