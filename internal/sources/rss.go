package sources

import (
	"context"
	"fmt"
	"strings"

	"github.com/mmcdole/gofeed"
)

// RSSSource fetches items from an RSS or Atom feed.
type RSSSource struct {
	name string
	url  string
}

func (s *RSSSource) Name() string { return s.name }

func (s *RSSSource) Fetch(ctx context.Context) ([]Item, error) {
	fp := gofeed.NewParser()
	feed, err := fp.ParseURLWithContext(s.url, ctx)
	if err != nil {
		return nil, fmt.Errorf("rss %q: %w", s.name, err)
	}

	items := make([]Item, 0, len(feed.Items))
	for _, it := range feed.Items {
		id := it.GUID
		if id == "" {
			id = it.Link
		}
		if id == "" {
			id = it.Title
		}

		excerpt := it.Description
		if excerpt == "" {
			excerpt = it.Content
		}
		excerpt = truncate(strings.TrimSpace(stripTags(excerpt)), 1200)

		var published = it.PublishedParsed
		if published == nil {
			published = it.UpdatedParsed
		}

		item := Item{
			SourceName: s.name,
			Title:      strings.TrimSpace(it.Title),
			URL:        it.Link,
			Excerpt:    excerpt,
			ID:         id,
		}
		if published != nil {
			item.Published = *published
		}
		items = append(items, item)
	}
	return items, nil
}
