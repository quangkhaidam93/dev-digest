package digest

import (
	"strings"
	"testing"
	"time"
)

func sampleDigest() Digest {
	return Digest{
		Title: "Dev Digest",
		Date:  time.Date(2026, 7, 11, 8, 0, 0, 0, time.UTC),
		Intro: "Today's issue: a spicy Go proposal and Bun eating Node's lunch again.",
		Sections: []Section{{
			Title: "releases",
			Items: []Item{{
				Headline: "React 19.2 RC",
				Summary:  "Server components get faster.",
				URL:      "https://example.com/react",
				Source:   "React",
			}},
		}},
	}
}

func TestRenderMarkdown(t *testing.T) {
	md, err := sampleDigest().RenderMarkdown()
	if err != nil {
		t.Fatalf("RenderMarkdown: %v", err)
	}
	for _, want := range []string{
		"# Dev Digest",
		"Today's issue:",
		"## releases",
		"[React 19.2 RC](https://example.com/react)",
		"Server components get faster.",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q\n---\n%s", want, md)
		}
	}
}

func TestRenderHTML(t *testing.T) {
	html, err := sampleDigest().RenderHTML()
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	for _, want := range []string{
		"<!doctype html>",
		"Dev Digest",
		"Today&#39;s issue:", // html-escaped apostrophe
		`href="https://example.com/react"`,
		"React 19.2 RC",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("html missing %q", want)
		}
	}
}

func TestEmptyAndCount(t *testing.T) {
	if !(Digest{}).Empty() {
		t.Error("zero digest should be empty")
	}
	d := sampleDigest()
	if d.Empty() {
		t.Error("sample digest should not be empty")
	}
	if d.ItemCount() != 1 {
		t.Errorf("ItemCount: got %d want 1", d.ItemCount())
	}
}

func TestAggregate(t *testing.T) {
	// Aggregate groups by source and preserves order.
	now := time.Now()
	d := Aggregate("T", now, nil)
	if !d.Empty() {
		t.Error("aggregate of no items should be empty")
	}
}
