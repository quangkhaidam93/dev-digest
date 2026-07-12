package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/quangkhaidam93/dev-digest/internal/config"
)

// GitHubSource fetches releases or tags for a repository via the GitHub REST
// API. An optional GITHUB_TOKEN env var lifts the rate limit.
type GitHubSource struct {
	name string
	repo string // owner/name
	kind string // releases | tags
}

func (s *GitHubSource) Name() string { return s.name }

func (s *GitHubSource) Fetch(ctx context.Context) ([]Item, error) {
	if s.kind == config.GitHubTags {
		return s.fetchTags(ctx)
	}
	return s.fetchReleases(ctx)
}

type ghRelease struct {
	ID          int64     `json:"id"`
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Body        string    `json:"body"`
	HTMLURL     string    `json:"html_url"`
	PublishedAt time.Time `json:"published_at"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
}

func (s *GitHubSource) fetchReleases(ctx context.Context) ([]Item, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases?per_page=30", s.repo)
	var rels []ghRelease
	if err := s.getJSON(ctx, url, &rels); err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(rels))
	for _, r := range rels {
		if r.Draft {
			continue
		}
		title := r.Name
		if title == "" {
			title = r.TagName
		}
		items = append(items, Item{
			SourceName: s.name,
			Title:      fmt.Sprintf("%s %s", s.repo, title),
			URL:        r.HTMLURL,
			Published:  r.PublishedAt,
			Excerpt:    truncate(collapseSpace(r.Body), 1500),
			ID:         "release:" + strconv.FormatInt(r.ID, 10),
		})
	}
	return items, nil
}

type ghTag struct {
	Name   string `json:"name"`
	Commit struct {
		SHA string `json:"sha"`
	} `json:"commit"`
}

func (s *GitHubSource) fetchTags(ctx context.Context) ([]Item, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/tags?per_page=30", s.repo)
	var tags []ghTag
	if err := s.getJSON(ctx, url, &tags); err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(tags))
	for _, t := range tags {
		items = append(items, Item{
			SourceName: s.name,
			Title:      fmt.Sprintf("%s %s", s.repo, t.Name),
			URL:        fmt.Sprintf("https://github.com/%s/releases/tag/%s", s.repo, t.Name),
			Excerpt:    "",
			ID:         "tag:" + t.Name,
		})
	}
	return items, nil
}

func (s *GitHubSource) getJSON(ctx context.Context, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "dev-digest")
	if tok := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("github %q: %w", s.name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("github %q: %s returned %s", s.name, url, resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("github %q: decode response: %w", s.name, err)
	}
	return nil
}
