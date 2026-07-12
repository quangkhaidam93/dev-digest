// Command dev-digest aggregates new developer content (RSS, GitHub releases,
// dev pages) into a daily newsletter, summarized by Claude and delivered to
// file/email/chat. Run it with no arguments for the interactive TUI, or
// `dev-digest run` (what cron invokes) for a headless run.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/quangkhaidam93/dev-digest/internal/config"
	"github.com/quangkhaidam93/dev-digest/internal/cron"
	"github.com/quangkhaidam93/dev-digest/internal/pipeline"
	"github.com/quangkhaidam93/dev-digest/internal/store"
	"github.com/quangkhaidam93/dev-digest/internal/tui"
)

// defaultModulePath is used for `update` when build info doesn't carry the
// module path (e.g. a bare `go build` without module context).
const defaultModulePath = "github.com/quangkhaidam93/dev-digest"

// version is the released version. Overridable at build time via
// -ldflags "-X main.version=…"; if set to empty it falls back to the embedded
// build info (VCS pseudo-version).
var version = "v1.0.1"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "dev-digest:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	// Handle version flags before flag parsing (so `-v`/`--version` don't error).
	if len(args) > 0 {
		switch args[0] {
		case "version", "--version", "-v":
			return cmdVersion()
		}
	}

	fs := flag.NewFlagSet("dev-digest", flag.ContinueOnError)
	configPath := fs.String("config", "", "path to config.toml (default: ~/.config/dev-digest/config.toml)")
	fs.Usage = usage
	if err := fs.Parse(args); err != nil {
		return err
	}

	path := *configPath
	if path == "" {
		p, err := config.DefaultPath()
		if err != nil {
			return err
		}
		path = p
	}

	cmd := "tui"
	rest := fs.Args()
	if len(rest) > 0 {
		cmd = rest[0]
		rest = rest[1:]
	}

	switch cmd {
	case "tui":
		return cmdTUI(path)
	case "run":
		return cmdRun(path)
	case "cron":
		return cmdCron(path, rest)
	case "uninstall":
		return cmdUninstall(path, rest)
	case "update":
		return cmdUpdate(rest)
	case "version":
		return cmdVersion()
	case "help", "-h", "--help":
		usage()
		return nil
	default:
		usage()
		return fmt.Errorf("unknown command %q", cmd)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `dev-digest — daily developer newsletter aggregator

Usage:
  dev-digest [--config PATH] [command]

Commands:
  tui                    Launch the interactive TUI (default)
  run                    Fetch, summarize, and deliver a digest (used by cron)
  cron install           Install a daily crontab entry that runs `+"`dev-digest run`"+`
  cron status            Show whether the crontab entry is registered
  cron uninstall         Remove the crontab entry
  uninstall              Remove the cron entry and delete the dev-digest binary
  uninstall --complete   Also delete the config and state (all settings/data)
  update [version]       Fetch and build the latest release (default) or a given version
  version                Print the version, commit, and Go version

Summarization provider is set by [digest].provider (anthropic | gemini | openrouter).

Environment:
  ANTHROPIC_API_KEY          API key when provider = anthropic
  GEMINI_API_KEY             API key when provider = gemini (or GOOGLE_API_KEY)
  OPENROUTER_API_KEY         API key when provider = openrouter
  GITHUB_TOKEN               optional, lifts GitHub API rate limits
  DEV_DIGEST_SMTP_PASSWORD   required when email delivery is enabled
`)
}

// loadConfigAndStore loads config (creating a starter file if none exists) and
// opens the state store.
func loadConfigAndStore(path string) (config.Config, *store.Store, error) {
	cfg, exists, err := config.Load(path)
	if err != nil {
		return config.Config{}, nil, err
	}
	if !exists {
		if err := config.Save(path, cfg); err != nil {
			return config.Config{}, nil, err
		}
		fmt.Fprintf(os.Stderr, "created starter config at %s\n", path)
	}
	if err := cfg.Validate(); err != nil {
		return config.Config{}, nil, err
	}
	sp, err := store.DefaultPath()
	if err != nil {
		return config.Config{}, nil, err
	}
	st, err := store.Load(sp)
	if err != nil {
		return config.Config{}, nil, err
	}
	return cfg, st, nil
}

func cmdTUI(path string) error {
	return tui.Run(path)
}

func cmdRun(path string) error {
	cfg, st, err := loadConfigAndStore(path)
	if err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	res, err := pipeline.Run(ctx, pipeline.Options{
		Config:  cfg,
		Store:   st,
		Now:     time.Now(),
		Log:     os.Stderr,
		Deliver: true,
	})
	if err != nil {
		return err
	}

	failed := 0
	for _, d := range res.Deliveries {
		if d.Err != nil {
			failed++
		}
	}
	if failed > 0 {
		return fmt.Errorf("%d of %d deliveries failed", failed, len(res.Deliveries))
	}
	return nil
}

func cmdCron(path string, args []string) error {
	action := ""
	if len(args) > 0 {
		action = args[0]
	}
	switch action {
	case "install":
		cfg, _, err := config.Load(path)
		if err != nil {
			return err
		}
		expr, err := cfg.Schedule.CronExpr()
		if err != nil {
			return fmt.Errorf("schedule: %w", err)
		}
		bin, err := os.Executable()
		if err != nil {
			return err
		}
		bin, _ = filepath.Abs(bin)
		entry := cron.Entry{
			Schedule: expr,
			Binary:   bin,
			LogPath:  cron.DefaultLogPath(),
		}
		// Preserve an explicit --config so cron uses the same file.
		if def, _ := config.DefaultPath(); def != path {
			entry.Config = path
		}
		if err := os.MkdirAll(filepath.Dir(entry.LogPath), 0o755); err != nil {
			return err
		}
		if err := cron.Install(entry); err != nil {
			return err
		}
		fmt.Printf("installed cron entry:\n  %s\n", entry.Line())
		return nil
	case "uninstall":
		if err := cron.Uninstall(); err != nil {
			return err
		}
		fmt.Println("removed dev-digest cron entry")
		return nil
	case "status", "":
		installed, line, err := cron.Status()
		if err != nil {
			return err
		}
		if !installed {
			fmt.Println("cron: not registered — run `dev-digest cron install`")
			return nil
		}
		fmt.Printf("cron: registered\n  %s\n", line)
		// Note if the registered time differs from the current config.
		if cfg, _, cerr := config.Load(path); cerr == nil {
			if expr, eerr := cfg.Schedule.CronExpr(); eerr == nil && !strings.HasPrefix(line, expr+" ") {
				fmt.Printf("  note: config schedule is %q (%s) — run `cron install` to update\n",
					cfg.Schedule.ResolvedDailyTime(), expr)
			}
		}
		return nil
	default:
		return fmt.Errorf("cron: expected 'install', 'uninstall', or 'status'")
	}
}

// cmdUninstall removes the cron entry and the binary. With --complete it also
// deletes the config and state (all settings/data).
func cmdUninstall(path string, args []string) error {
	complete := false
	for _, a := range args {
		switch a {
		case "--complete", "-complete", "complete", "--all", "--purge":
			complete = true
		default:
			return fmt.Errorf("uninstall: unknown argument %q (use --complete to also remove settings)", a)
		}
	}

	var removed []string

	// 1. Cron entry.
	if err := cron.Uninstall(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not remove cron entry: %v\n", err)
	} else {
		removed = append(removed, "cron entry")
	}

	// 2. Config + state (only with --complete).
	if complete {
		paths, err := removeAllSettings(path)
		removed = append(removed, paths...)
		if err != nil {
			return err
		}
	}

	// 3. The binary itself, last (the process keeps running after unlink).
	if bin, err := os.Executable(); err == nil {
		bin, _ = filepath.Abs(bin)
		if rmErr := os.Remove(bin); rmErr == nil {
			removed = append(removed, "binary "+bin)
		} else {
			fmt.Fprintf(os.Stderr, "warning: could not remove binary %s: %v\n", bin, rmErr)
		}
	}

	fmt.Println("uninstalled dev-digest:")
	for _, r := range removed {
		fmt.Println("  removed", r)
	}
	if !complete {
		fmt.Println("\nConfig and state were kept. Run `dev-digest uninstall --complete` to remove them too.")
	}
	return nil
}

// modulePath returns the Go module path for updates, from build info when
// available, else the compiled-in default.
func modulePath() string {
	if bi, ok := debug.ReadBuildInfo(); ok {
		if p := bi.Main.Path; p != "" && p != "command-line-arguments" {
			return p
		}
	}
	return defaultModulePath
}

// versionString returns the human-readable version: the ldflags value if set,
// otherwise the module version plus VCS revision from the embedded build info.
func versionString() string {
	if version != "" {
		return version
	}
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	// A released install (@vX.Y.Z) or `@latest` pseudo-version already encodes the
	// commit and dirty state — use it verbatim.
	if v := bi.Main.Version; v != "" && v != "(devel)" {
		return v
	}
	// Local dev build: synthesize "dev+<commit>[.dirty]" from VCS build settings.
	v := "dev"
	var rev string
	var dirty bool
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if rev != "" {
		if len(rev) > 12 {
			rev = rev[:12]
		}
		v += "+" + rev
		if dirty {
			v += ".dirty"
		}
	}
	return v
}

// cmdVersion prints version, commit, and Go/runtime details.
func cmdVersion() error {
	fmt.Printf("dev-digest %s\n", versionString())
	fmt.Printf("  module: %s\n", modulePath())
	fmt.Printf("  go:     %s (%s/%s)\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	return nil
}

// updateTarget builds the `go install` target for a version ref (default latest).
func updateTarget(ref string) string {
	if ref == "" {
		ref = "latest"
	}
	return modulePath() + "@" + ref
}

// cmdUpdate fetches and builds the latest (or a given) version via `go install`.
func cmdUpdate(args []string) error {
	ref := "latest"
	if len(args) > 0 && args[0] != "" {
		ref = args[0]
	}
	if _, err := exec.LookPath("go"); err != nil {
		return fmt.Errorf("update needs the Go toolchain on PATH: %w", err)
	}
	target := updateTarget(ref)
	fmt.Printf("updating: go install %s\n", target)

	cmd := exec.Command("go", "install", target)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("update failed (needs network; the module must be published): %w", err)
	}
	fmt.Println("updated — run `dev-digest version` to confirm.")
	return nil
}

// removeAllSettings deletes the config and state directories, returning the
// paths it removed.
func removeAllSettings(configPath string) ([]string, error) {
	var removed []string

	// Config: remove the whole dev-digest config dir when the config lives in
	// one; otherwise just the file (a custom --config path may be shared).
	cfgDir := filepath.Dir(configPath)
	if filepath.Base(cfgDir) == "dev-digest" {
		if err := os.RemoveAll(cfgDir); err != nil {
			return removed, fmt.Errorf("remove config dir: %w", err)
		}
		removed = append(removed, "config "+cfgDir)
	} else {
		if err := os.Remove(configPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return removed, fmt.Errorf("remove config: %w", err)
		}
		removed = append(removed, "config "+configPath)
	}

	// State directory (~/.local/state/dev-digest).
	if sp, err := store.DefaultPath(); err == nil {
		sdir := filepath.Dir(sp)
		if err := os.RemoveAll(sdir); err != nil {
			return removed, fmt.Errorf("remove state dir: %w", err)
		}
		removed = append(removed, "state "+sdir)
	}

	return removed, nil
}
