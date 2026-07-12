package digest

import (
	"context"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// anthropicCompleter uses the official Anthropic SDK. When apiKey is empty the
// SDK resolves credentials itself (ANTHROPIC_API_KEY or an `ant auth login`
// profile); otherwise the given key is used.
type anthropicCompleter struct {
	model  string
	effort string
	apiKey string
}

func (c *anthropicCompleter) label() string { return "anthropic" }

func (c *anthropicCompleter) complete(ctx context.Context, system, user string, schema map[string]any) (string, error) {
	var opts []option.RequestOption
	if c.apiKey != "" {
		opts = append(opts, option.WithAPIKey(c.apiKey))
	}
	client := anthropic.NewClient(opts...)

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(c.model),
		MaxTokens: 16000,
		System:    []anthropic.TextBlockParam{{Text: system}},
		Thinking: anthropic.ThinkingConfigParamUnion{
			OfAdaptive: &anthropic.ThinkingConfigAdaptiveParam{},
		},
		OutputConfig: anthropic.OutputConfigParam{
			Effort: effortFor(c.effort),
			Format: anthropic.JSONOutputFormatParam{Schema: schema},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(user)),
		},
	}

	stream := client.Messages.NewStreaming(ctx, params)
	message := anthropic.Message{}
	for stream.Next() {
		if err := message.Accumulate(stream.Current()); err != nil {
			return "", fmt.Errorf("accumulate stream: %w", err)
		}
	}
	if err := stream.Err(); err != nil {
		return "", err
	}
	if message.StopReason == anthropic.StopReasonRefusal {
		return "", fmt.Errorf("request refused: %s", message.StopDetails.Explanation)
	}
	return firstText(message), nil
}

func firstText(m anthropic.Message) string {
	for _, block := range m.Content {
		if t, ok := block.AsAny().(anthropic.TextBlock); ok {
			return t.Text
		}
	}
	return ""
}

func effortFor(s string) anthropic.OutputConfigEffort {
	switch s {
	case "low":
		return anthropic.OutputConfigEffortLow
	case "high":
		return anthropic.OutputConfigEffortHigh
	default:
		return anthropic.OutputConfigEffortMedium
	}
}
