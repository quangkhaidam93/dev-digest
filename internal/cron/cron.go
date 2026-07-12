// Package cron installs and removes the dev-digest crontab entry idempotently,
// guarded by a marker comment so we only ever touch our own line.
package cron

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const markerBegin = "# dev-digest (managed) — do not edit this block"
const markerEnd = "# end dev-digest"

// Entry describes the scheduled job.
type Entry struct {
	Schedule string // cron expression, e.g. "0 8 * * *"
	Binary   string // absolute path to the dev-digest binary
	LogPath  string // where to append run output
	Config   string // optional --config path (empty to use default)
}

// Line renders the crontab command line for the entry.
func (e Entry) Line() string {
	cmd := e.Binary
	if e.Config != "" {
		cmd += " --config " + e.Config
	}
	cmd += " run"
	if e.LogPath != "" {
		cmd += " >> " + e.LogPath + " 2>&1"
	}
	return e.Schedule + " " + cmd
}

// Install adds or replaces the managed block in the user's crontab.
func Install(e Entry) error {
	current, err := readCrontab()
	if err != nil {
		return err
	}
	block := strings.Join([]string{markerBegin, e.Line(), markerEnd}, "\n")
	updated := replaceBlock(current, block)
	return writeCrontab(updated)
}

// Status reports whether the managed dev-digest entry is present in the user's
// crontab and, if so, its schedule/command line.
func Status() (installed bool, line string, err error) {
	content, err := readCrontab()
	if err != nil {
		return false, "", err
	}
	installed, line = parseStatus(content)
	return installed, line, nil
}

// parseStatus extracts whether the managed block exists (and its command line)
// from crontab content.
func parseStatus(content string) (bool, string) {
	inBlock := false
	for _, ln := range strings.Split(content, "\n") {
		t := strings.TrimSpace(ln)
		switch {
		case t == markerBegin:
			inBlock = true
		case t == markerEnd:
			inBlock = false
		case inBlock && t != "":
			return true, t
		}
	}
	return false, ""
}

// Uninstall removes the managed block from the user's crontab.
func Uninstall() error {
	current, err := readCrontab()
	if err != nil {
		return err
	}
	updated := replaceBlock(current, "")
	return writeCrontab(updated)
}

// readCrontab returns the current crontab contents, or "" if none is set.
func readCrontab() (string, error) {
	out, err := exec.Command("crontab", "-l").Output()
	if err != nil {
		// `crontab -l` exits non-zero when there is no crontab yet.
		if ee, ok := err.(*exec.ExitError); ok {
			if strings.Contains(strings.ToLower(string(ee.Stderr)), "no crontab") {
				return "", nil
			}
			// Some implementations print to stdout / exit 1 with empty output.
			if len(bytes.TrimSpace(out)) == 0 {
				return "", nil
			}
		}
		if len(bytes.TrimSpace(out)) == 0 {
			return "", nil
		}
	}
	return string(out), nil
}

// replaceBlock removes any existing managed block from content and appends the
// new block (if non-empty).
func replaceBlock(content, block string) string {
	lines := strings.Split(content, "\n")
	var kept []string
	inBlock := false
	for _, ln := range lines {
		switch {
		case strings.TrimSpace(ln) == markerBegin:
			inBlock = true
			continue
		case strings.TrimSpace(ln) == markerEnd:
			inBlock = false
			continue
		case inBlock:
			continue
		default:
			kept = append(kept, ln)
		}
	}
	// Trim trailing blank lines.
	for len(kept) > 0 && strings.TrimSpace(kept[len(kept)-1]) == "" {
		kept = kept[:len(kept)-1]
	}
	result := strings.Join(kept, "\n")
	if block != "" {
		if result != "" {
			result += "\n"
		}
		result += block
	}
	if result != "" {
		result += "\n"
	}
	return result
}

// writeCrontab installs content via `crontab -` (empty content clears it).
func writeCrontab(content string) error {
	if strings.TrimSpace(content) == "" {
		// `crontab -r` removes the crontab; ignore "no crontab" errors.
		cmd := exec.Command("crontab", "-r")
		_ = cmd.Run()
		return nil
	}
	cmd := exec.Command("crontab", "-")
	cmd.Stdin = strings.NewReader(content)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("crontab install failed: %v: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// DefaultLogPath returns a sensible per-user log path for the cron job.
func DefaultLogPath() string {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			dir = home + "/.local/state"
		}
	}
	if dir == "" {
		return "/tmp/dev-digest.log"
	}
	return dir + "/dev-digest/run.log"
}
