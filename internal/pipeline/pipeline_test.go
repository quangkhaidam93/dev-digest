package pipeline

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/quangkhaidam93/dev-digest/internal/sources"
	"github.com/quangkhaidam93/dev-digest/internal/store"
)

func newStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Load(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	return st
}

var now = time.Date(2026, 7, 12, 8, 0, 0, 0, time.UTC)

func dated(id string, ago time.Duration) sources.Item {
	return sources.Item{ID: id, Title: id, Published: now.Add(-ago)}
}
func dateless(id string) sources.Item { return sources.Item{ID: id, Title: id} }

func ids(items []sources.Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.ID
	}
	return out
}

// Dated items outside the 24h window are dropped; recent ones pass.
func TestSelectAgeWindow(t *testing.T) {
	st := newStore(t)
	items := []sources.Item{
		dated("fresh", 2*time.Hour),  // within 24h → deliver
		dated("edge", 23*time.Hour),  // within 24h → deliver
		dated("stale", 48*time.Hour), // older than 24h → drop
		dated("ancient", 300*time.Hour),
	}
	sel := selectNew(st, "rss", items, false /*firstRun*/, true /*deliver*/, now, 24*time.Hour)

	got := ids(sel.deliver)
	want := []string{"fresh", "edge"}
	if len(got) != len(want) || got[0] != "fresh" || got[1] != "edge" {
		t.Errorf("delivered = %v, want %v", got, want)
	}
	if sel.tooOld != 2 {
		t.Errorf("tooOld = %d, want 2", sel.tooOld)
	}
}

// maxAge=0 disables the age filter entirely.
func TestSelectNoAgeFilter(t *testing.T) {
	st := newStore(t)
	items := []sources.Item{dated("a", 2*time.Hour), dated("b", 1000*time.Hour)}
	sel := selectNew(st, "rss", items, false, true, now, 0)
	if len(sel.deliver) != 2 {
		t.Errorf("with maxAge=0 expected all delivered, got %d", len(sel.deliver))
	}
}

// First real run seeds dateless items (recorded, not delivered) so the backlog
// isn't dumped; a later run delivers only genuinely new ones.
func TestSelectFirstRunSeedsDateless(t *testing.T) {
	st := newStore(t)
	items := []sources.Item{dateless("t1"), dateless("t2"), dateless("t3")}

	sel := selectNew(st, "gh", items, true /*firstRun*/, true /*deliver*/, now, 24*time.Hour)
	if len(sel.deliver) != 0 {
		t.Errorf("first run should deliver nothing dateless, got %d", len(sel.deliver))
	}
	if sel.seeded != 3 || len(sel.record) != 3 {
		t.Errorf("expected 3 seeded/recorded, got seeded=%d record=%d", sel.seeded, len(sel.record))
	}

	// Commit the seed, then a second run with one new tag delivers only that one.
	st.Record("gh", sel.record, now)
	items2 := append(items, dateless("t4"))
	sel2 := selectNew(st, "gh", items2, false /*firstRun*/, true, now, 24*time.Hour)
	if got := ids(sel2.deliver); len(got) != 1 || got[0] != "t4" {
		t.Errorf("second run delivered = %v, want [t4]", got)
	}
}

// Preview (deliver=false) does NOT seed — it shows dateless items so you can
// verify a newly added source produces content.
func TestSelectPreviewShowsDateless(t *testing.T) {
	st := newStore(t)
	items := []sources.Item{dateless("t1"), dateless("t2")}
	sel := selectNew(st, "gh", items, true /*firstRun*/, false /*preview*/, now, 24*time.Hour)
	if len(sel.deliver) != 2 {
		t.Errorf("preview should show dateless items, got %d", len(sel.deliver))
	}
}

// Already-seen items are never re-delivered.
func TestSelectDedup(t *testing.T) {
	st := newStore(t)
	st.Record("rss", []string{"seen"}, now)
	items := []sources.Item{dated("seen", time.Hour), dated("new", time.Hour)}
	sel := selectNew(st, "rss", items, false, true, now, 24*time.Hour)
	if got := ids(sel.deliver); len(got) != 1 || got[0] != "new" {
		t.Errorf("delivered = %v, want [new]", got)
	}
}
