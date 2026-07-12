package cron

import (
	"strings"
	"testing"
)

func TestEntryLine(t *testing.T) {
	e := Entry{Schedule: "0 8 * * *", Binary: "/usr/local/bin/dev-digest", LogPath: "/tmp/dd.log"}
	got := e.Line()
	want := "0 8 * * * /usr/local/bin/dev-digest run >> /tmp/dd.log 2>&1"
	if got != want {
		t.Errorf("Line()\n got %q\nwant %q", got, want)
	}

	e.Config = "/etc/dd.toml"
	got = e.Line()
	if !strings.Contains(got, "--config /etc/dd.toml run") {
		t.Errorf("Line() with config: %q", got)
	}
}

func TestReplaceBlockIdempotent(t *testing.T) {
	existing := "0 0 * * * /some/other/job\n"
	block := strings.Join([]string{markerBegin, "0 8 * * * /bin/dd run", markerEnd}, "\n")

	// First install: appends block, preserves the existing line.
	out1 := replaceBlock(existing, block)
	if !strings.Contains(out1, "/some/other/job") {
		t.Error("existing job line dropped")
	}
	if strings.Count(out1, markerBegin) != 1 {
		t.Errorf("expected exactly one managed block, got:\n%s", out1)
	}

	// Second install with a new schedule: replaces, does not duplicate.
	block2 := strings.Join([]string{markerBegin, "30 9 * * * /bin/dd run", markerEnd}, "\n")
	out2 := replaceBlock(out1, block2)
	if strings.Count(out2, markerBegin) != 1 {
		t.Errorf("block duplicated on reinstall:\n%s", out2)
	}
	if strings.Contains(out2, "0 8 * * *") || !strings.Contains(out2, "30 9 * * *") {
		t.Errorf("schedule not replaced:\n%s", out2)
	}
	if !strings.Contains(out2, "/some/other/job") {
		t.Error("existing job line dropped on reinstall")
	}

	// Uninstall: removes the block, keeps the other job.
	out3 := replaceBlock(out2, "")
	if strings.Contains(out3, markerBegin) {
		t.Error("managed block not removed on uninstall")
	}
	if !strings.Contains(out3, "/some/other/job") {
		t.Error("existing job line dropped on uninstall")
	}
}
