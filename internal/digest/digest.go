// Package digest turns a set of new source items into a rendered newsletter.
// When summarization is enabled it uses the Claude API to write a witty intro
// and short per-item summaries; otherwise it aggregates items raw. Both paths
// produce a Digest, which is rendered to Markdown and HTML by the same
// templates so every delivery channel shows identical content.
package digest

import "time"

// Digest is the structured newsletter, ready to render.
type Digest struct {
	Title    string    `json:"title"`
	Date     time.Time `json:"date"`
	Intro    string    `json:"intro"` // the witty "Today's issue: …" paragraph
	Sections []Section `json:"sections"`
}

// Section groups related items under a heading (typically by source or theme).
type Section struct {
	Title string `json:"title"`
	Items []Item `json:"items"`
}

// Item is one rendered entry.
type Item struct {
	Headline string `json:"headline"`
	Summary  string `json:"summary"`
	URL      string `json:"url"`
	Source   string `json:"source"`
}

// ItemCount returns the total number of items across all sections.
func (d Digest) ItemCount() int {
	n := 0
	for _, s := range d.Sections {
		n += len(s.Items)
	}
	return n
}

// Empty reports whether the digest has no items in any section.
func (d Digest) Empty() bool {
	return d.ItemCount() == 0
}
