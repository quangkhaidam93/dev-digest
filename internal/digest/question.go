package digest

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/quangkhaidam93/dev-digest/internal/config"
)

// DefaultQuestionPrompt is the built-in system prompt used to generate the
// question of the day. Users can override it via config ([digest].question_prompt).
const DefaultQuestionPrompt = `You write a short daily learning prompt for software engineers.
Pick ONE topic at random from this set (vary it day to day):
- a software-engineering fact or best practice
- a code smell (name it and how to refactor)
- a data structure or algorithm
- system design
- concurrency, databases, testing, or security
Produce a concise, genuinely interesting question and a 2-4 sentence answer.
Return a single JSON object matching the schema and nothing else.`

// questionSchema constrains the model output for the daily question.
var questionSchema = map[string]any{
	"type":                 "object",
	"additionalProperties": false,
	"required":             []string{"topic", "question", "answer"},
	"properties": map[string]any{
		"topic":    map[string]any{"type": "string"},
		"question": map[string]any{"type": "string"},
		"answer":   map[string]any{"type": "string"},
	},
}

// GenerateQuestion builds a "question of the day" Digest to send when a run has
// no news. It uses the configured LLM provider; if that fails (or no key is
// configured) it falls back to a built-in question so a notification still goes
// out.
func GenerateQuestion(ctx context.Context, cfg config.Digest, apiKey string, now time.Time) Digest {
	q, err := generateQuestionLLM(ctx, cfg, apiKey)
	if err != nil {
		q = fallbackQuestion(now)
	}
	return questionDigest(cfg.Title, now, q)
}

type question struct {
	Topic    string `json:"topic"`
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

func generateQuestionLLM(ctx context.Context, cfg config.Digest, apiKey string) (question, error) {
	c, err := newCompleter(cfg, apiKey)
	if err != nil {
		return question{}, err
	}
	sys := strings.TrimSpace(cfg.QuestionPrompt)
	if sys == "" {
		sys = DefaultQuestionPrompt
	}
	raw, err := c.complete(ctx, sys,
		"Give me today's question. Choose a fresh topic and a non-obvious question.",
		questionSchema)
	if err != nil {
		return question{}, err
	}
	raw = stripCodeFence(strings.TrimSpace(raw))
	var q question
	if err := json.Unmarshal([]byte(raw), &q); err != nil {
		return question{}, fmt.Errorf("parse question: %w", err)
	}
	if q.Question == "" || q.Answer == "" {
		return question{}, fmt.Errorf("empty question")
	}
	return q, nil
}

func questionDigest(title string, now time.Time, q question) Digest {
	sectionTitle := "Question of the day"
	if q.Topic != "" {
		sectionTitle += " · " + q.Topic
	}
	return Digest{
		Title: title,
		Date:  now,
		Intro: "No new updates today — here's something to sharpen your skills.",
		Sections: []Section{{
			Title: sectionTitle,
			Items: []Item{{Headline: q.Question, Summary: q.Answer}},
		}},
	}
}

// fallbackQuestions is a small built-in pool used when the LLM is unavailable.
var fallbackQuestions = []question{
	{"Data structures", "When would a hash map's O(1) lookup degrade to O(n)?",
		"When many keys collide into the same bucket — e.g. a weak hash or adversarial keys. Good implementations mitigate this by resizing on load factor and, in some languages, switching a bucket to a balanced tree."},
	{"Code smell", "What is a \"primitive obsession\" code smell and how do you fix it?",
		"Using raw primitives (strings, ints) to represent domain concepts like money or email. Replace them with small value types that carry validation and behavior, so invalid states become unrepresentable."},
	{"System design", "Why add a message queue between two services that could call each other directly?",
		"To decouple producer and consumer speeds, absorb traffic spikes, and survive downstream outages by buffering work. It trades immediate consistency for resilience and throughput."},
	{"Algorithms", "Why is binary search O(log n), and what invariant must hold to use it?",
		"Each step halves the search space, so it takes log2(n) steps. It requires the data to be sorted (or otherwise monotonic) on the key you're searching."},
	{"Concurrency", "What's the difference between a race condition and a deadlock?",
		"A race condition is a wrong result from unsynchronized access to shared state; a deadlock is two or more threads each waiting on a lock the other holds, so none proceed. One corrupts data, the other freezes progress."},
	{"Software engineering", "Why prefer composition over inheritance?",
		"Inheritance couples subclasses to a base class's implementation and creates rigid hierarchies. Composition builds behavior from small, swappable parts, which is easier to test, change, and reason about."},
	{"Databases", "What problem do database indexes solve, and what do they cost?",
		"They turn full-table scans into fast lookups by maintaining a sorted structure over columns. The cost is extra storage and slower writes, since every insert/update must also maintain the index."},
	{"Security", "What is the difference between authentication and authorization?",
		"Authentication verifies who you are (identity); authorization decides what you're allowed to do (permissions). You authenticate first, then authorization gates each action."},
}

// fallbackQuestion picks a built-in question that rotates by day so it varies.
func fallbackQuestion(now time.Time) question {
	return fallbackQuestions[now.YearDay()%len(fallbackQuestions)]
}
