// Package store persists per-source run state: the last time a source was
// fetched and a rolling set of item IDs already seen, so each run emits only
// items that are new. State is a single JSON file — no database dependency,
// inspectable, and sufficient for this volume.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// maxSeenPerSource bounds the rolling seen-ID set so state.json can't grow
// without limit. Newest IDs are kept.
const maxSeenPerSource = 500

// State is the whole persisted document, keyed by source name.
type State struct {
	Sources map[string]SourceState `json:"sources"`
}

// SourceState is per-source run state.
type SourceState struct {
	LastRun time.Time `json:"last_run"`
	// Seen is an ordered list of item IDs already reported (oldest first).
	Seen []string `json:"seen"`
}

// Store wraps a State bound to a file path.
type Store struct {
	path  string
	state State
}

// DefaultPath returns ~/.local/state/dev-digest/state.json, honoring
// XDG_STATE_HOME.
func DefaultPath() (string, error) {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(dir, "dev-digest", "state.json"), nil
}

// Load reads state from path, returning an empty store if the file is absent.
func Load(path string) (*Store, error) {
	s := &Store{path: path, state: State{Sources: map[string]SourceState{}}}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read state: %w", err)
	}
	if err := json.Unmarshal(data, &s.state); err != nil {
		return nil, fmt.Errorf("parse state %s: %w", path, err)
	}
	if s.state.Sources == nil {
		s.state.Sources = map[string]SourceState{}
	}
	return s, nil
}

// LastRun returns the last-run time for a source (zero if never run).
func (s *Store) LastRun(source string) time.Time {
	return s.state.Sources[source].LastRun
}

// IsNew reports whether an item ID has not been seen for the given source.
func (s *Store) IsNew(source, id string) bool {
	st := s.state.Sources[source]
	for _, seen := range st.Seen {
		if seen == id {
			return false
		}
	}
	return true
}

// Record marks the given item IDs as seen for a source and sets its last-run
// time to now. Call after a successful fetch+deliver cycle. The seen set is
// trimmed to maxSeenPerSource, keeping the most recent IDs.
func (s *Store) Record(source string, ids []string, now time.Time) {
	st := s.state.Sources[source]
	st.LastRun = now
	st.Seen = append(st.Seen, ids...)
	if len(st.Seen) > maxSeenPerSource {
		st.Seen = st.Seen[len(st.Seen)-maxSeenPerSource:]
	}
	s.state.Sources[source] = st
}

// Save writes state back to disk atomically.
func (s *Store) Save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	data, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write state: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("replace state: %w", err)
	}
	return nil
}
