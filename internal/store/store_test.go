package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestDedupAcrossReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	now := time.Date(2026, 7, 11, 8, 0, 0, 0, time.UTC)

	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// First run: everything is new.
	for _, id := range []string{"a", "b", "c"} {
		if !s.IsNew("src", id) {
			t.Errorf("expected %q new on first run", id)
		}
	}
	s.Record("src", []string{"a", "b", "c"}, now)
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Reload: previously-seen IDs must be recognized.
	s2, err := Load(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	for _, id := range []string{"a", "b", "c"} {
		if s2.IsNew("src", id) {
			t.Errorf("expected %q seen after reload", id)
		}
	}
	if s2.IsNew("src", "d") == false {
		t.Error("expected new id d to be new")
	}
	if !s2.LastRun("src").Equal(now) {
		t.Errorf("last run: got %v want %v", s2.LastRun("src"), now)
	}
	// Different source must not share seen state.
	if !s2.IsNew("other", "a") {
		t.Error("expected id a new for a different source")
	}
}

func TestSeenTrimmed(t *testing.T) {
	s, _ := Load(filepath.Join(t.TempDir(), "state.json"))
	ids := make([]string, maxSeenPerSource+50)
	for i := range ids {
		ids[i] = string(rune('a'+i%26)) + itoa(i)
	}
	s.Record("src", ids, time.Now())
	got := len(s.state.Sources["src"].Seen)
	if got != maxSeenPerSource {
		t.Errorf("seen length: got %d want %d", got, maxSeenPerSource)
	}
	// The most recent ID must still be present (not trimmed).
	if s.IsNew("src", ids[len(ids)-1]) {
		t.Error("most recent id should be retained")
	}
	// The oldest ID should have been trimmed.
	if !s.IsNew("src", ids[0]) {
		t.Error("oldest id should have been trimmed")
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}
