package sources

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"

	"github.com/PuerkitoBio/goquery"
)

// WebpageSource fetches a web page and extracts the text of an optional CSS
// selector (or the whole body). It emits a single item whose ID is a hash of
// the extracted text, so dedup only surfaces it when the content changes.
type WebpageSource struct {
	name     string
	url      string
	selector string
}

func (s *WebpageSource) Name() string { return s.name }

func (s *WebpageSource) Fetch(ctx context.Context) ([]Item, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "dev-digest")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("webpage %q: %w", s.name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("webpage %q: %s returned %s", s.name, s.url, resp.Status)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("webpage %q: parse html: %w", s.name, err)
	}

	sel := s.selector
	if sel == "" {
		sel = "body"
	}
	text := collapseSpace(doc.Find(sel).First().Text())
	if text == "" {
		return nil, nil
	}

	sum := sha256.Sum256([]byte(text))
	title := doc.Find("title").First().Text()
	if title == "" {
		title = s.name
	}

	return []Item{{
		SourceName: s.name,
		Title:      collapseSpace(title),
		URL:        s.url,
		Excerpt:    truncate(text, 1500),
		ID:         "hash:" + hex.EncodeToString(sum[:8]),
	}}, nil
}
