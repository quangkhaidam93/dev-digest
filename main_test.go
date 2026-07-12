package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/quangkhaidam93/dev-digest/internal/config"
	"github.com/quangkhaidam93/dev-digest/internal/store"
)

func TestVersionString(t *testing.T) {
	// The baked-in release version is reported by default.
	if got := versionString(); got != "v1.0.2" {
		t.Errorf("default version = %q, want v1.0.2", got)
	}
	// A build-time override wins.
	old := version
	version = "v9.9.9"
	defer func() { version = old }()
	if got := versionString(); got != "v9.9.9" {
		t.Errorf("ldflags version should win, got %q", got)
	}
	// With version cleared, it falls back to embedded build info (non-empty).
	version = ""
	if got := versionString(); got == "" {
		t.Error("fallback versionString() must not be empty")
	}
}

func TestModulePath(t *testing.T) {
	if p := modulePath(); !strings.Contains(p, "dev-digest") {
		t.Errorf("modulePath() = %q, want it to contain dev-digest", p)
	}
}

func TestUpdateTarget(t *testing.T) {
	if got := updateTarget(""); !strings.HasSuffix(got, "@latest") {
		t.Errorf("updateTarget(\"\") = %q, want …@latest", got)
	}
	if got := updateTarget("v1.2.3"); !strings.HasSuffix(got, "@v1.2.3") {
		t.Errorf("updateTarget(v1.2.3) = %q, want …@v1.2.3", got)
	}
	if got := updateTarget("latest"); !strings.HasPrefix(got, modulePath()+"@") {
		t.Errorf("updateTarget should prefix the module path, got %q", got)
	}
}

// removeAllSettings should delete both the config dir and the state dir.
func TestRemoveAllSettings(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(base, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(base, "state"))

	cfgPath, err := config.DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	sp, err := store.DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(sp), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sp, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	removed, err := removeAllSettings(cfgPath)
	if err != nil {
		t.Fatalf("removeAllSettings: %v", err)
	}
	if len(removed) != 2 {
		t.Errorf("expected 2 removed paths, got %v", removed)
	}
	if _, err := os.Stat(filepath.Dir(cfgPath)); !os.IsNotExist(err) {
		t.Error("config dir was not removed")
	}
	if _, err := os.Stat(filepath.Dir(sp)); !os.IsNotExist(err) {
		t.Error("state dir was not removed")
	}
}

// A custom (non-dev-digest) config path should have only the file removed, not
// its (possibly shared) parent directory.
func TestRemoveAllSettingsCustomConfigPath(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_STATE_HOME", filepath.Join(base, "state"))

	shared := filepath.Join(base, "shared")
	if err := os.MkdirAll(shared, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(shared, "dd.toml")
	if err := os.WriteFile(cfgPath, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(shared, "other.txt")
	if err := os.WriteFile(keep, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := removeAllSettings(cfgPath); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(cfgPath); !os.IsNotExist(err) {
		t.Error("custom config file should have been removed")
	}
	if _, err := os.Stat(keep); err != nil {
		t.Error("sibling file in a shared dir must be preserved")
	}
}
