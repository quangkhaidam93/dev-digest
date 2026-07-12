package sources

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// stripTags removes HTML tags from s, returning collapsed plain text. On parse
// failure it returns the input unchanged.
func stripTags(s string) string {
	if !strings.Contains(s, "<") {
		return s
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(s))
	if err != nil {
		return s
	}
	return collapseSpace(doc.Text())
}

// collapseSpace collapses runs of whitespace into single spaces and trims.
func collapseSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
