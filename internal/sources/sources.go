// Package sources fetches content from developer sources (RSS/Atom feeds,
// GitHub releases/tags, and web pages) and normalizes each into an Item.
package sources

import (
	"context"
	"fmt"
	"time"

	"github.com/quangkhaidam93/dev-digest/internal/config"
)

// Item is a single normalized piece of content from a source.
type Item struct {
	SourceName string
	Title      string
	URL        string
	Published  time.Time // zero if unknown
	Excerpt    string    // raw content/summary text fed to Claude
	ID         string    // GUID / release id / content hash — dedup key
}

// Source fetches items. Implementations are constructed from a config.Source.
type Source interface {
	// Fetch returns the current items for this source (not yet deduped).
	Fetch(ctx context.Context) ([]Item, error)
	// Name returns the source's configured display name.
	Name() string
}

// New builds a Source from its configuration.
func New(c config.Source) (Source, error) {
	switch c.Type {
	case config.SourceRSS:
		return &RSSSource{name: c.Name, url: c.URL}, nil
	case config.SourceGitHub:
		kind := c.Kind
		if kind == "" {
			kind = config.GitHubReleases
		}
		return &GitHubSource{name: c.Name, repo: c.Repo, kind: kind}, nil
	case config.SourceWebpage:
		return &WebpageSource{name: c.Name, url: c.URL, selector: c.Selector}, nil
	default:
		return nil, fmt.Errorf("unknown source type %q", c.Type)
	}
}

// truncate shortens s to at most n runes, appending an ellipsis if cut.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
