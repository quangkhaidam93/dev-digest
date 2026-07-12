package digest

import (
	"time"

	"github.com/quangkhaidam93/dev-digest/internal/sources"
)

// Aggregate builds a Digest from raw items without calling the API: one section
// per source, items carry their original excerpt as the summary. Used when
// summarization is disabled or no API key is available.
func Aggregate(title string, now time.Time, items []sources.Item) Digest {
	d := Digest{Title: title, Date: now}

	order := []string{}
	bySource := map[string][]Item{}
	for _, it := range items {
		if _, ok := bySource[it.SourceName]; !ok {
			order = append(order, it.SourceName)
		}
		bySource[it.SourceName] = append(bySource[it.SourceName], Item{
			Headline: it.Title,
			Summary:  it.Excerpt,
			URL:      it.URL,
			Source:   it.SourceName,
		})
	}

	for _, name := range order {
		d.Sections = append(d.Sections, Section{Title: name, Items: bySource[name]})
	}
	return d
}
