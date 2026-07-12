# dev-digest

**Your own daily developer newsletter, assembled from the sources you actually care about.**

Staying current on framework releases, library updates, and dev blogs usually means
visiting a dozen changelogs and re-reading things you've already seen. dev-digest does that
scan for you: once a day it collects what's **new** across your configured sources, has an
LLM write a concise, Bytes-style briefing (a witty _"Today's issue…"_ intro plus a short
summary per item), and delivers it wherever you read — a file, your inbox, or a chat
channel. Keeping up becomes a 60-second read instead of a chore, and on quiet days it still
nudges you with a short learning question so the habit sticks.

It's a single static Go binary with an interactive terminal UI for setup and a one-shot
`run` command that a daily cron job invokes.

## What it does

- **Aggregates** new content from three source types — **RSS/Atom feeds**, **GitHub
  releases/tags**, and **dev/changelog web pages** (change-detected by content hash).
- **Filters to what's new** since the last run: an age window (default: the last 24h) plus
  first-run seeding, so a newly added source never dumps its whole backlog at you.
- **Summarizes with an LLM** — **Anthropic**, **Google Gemini**, or **OpenRouter** — into an
  intro and per-item summaries grouped into sections. Falls back to raw aggregation if the
  model is unavailable, so a run never fails silently.
- **Delivers** the same digest as **Markdown + HTML** to any combination of **file**,
  **email (SMTP)**, and **chat webhook** (Slack / Discord / generic) — channels are
  independent.
- **Fills quiet days**: when nothing is new it sends an AI **question of the day** (an SE
  fact, code smell, data structure/algorithm, system-design prompt, …) with an offline
  fallback, so a notification always arrives.
- **Runs daily via cron** at a time you pick; the app writes and manages its own crontab
  entry and can tell you whether it's registered.
- **Fully TUI-managed** — sources, provider/model/API keys, delivery channels, the schedule,
  and even the question prompt are all editable in the terminal. Config is plain TOML you can
  also hand-edit.
- **Self-contained**: single binary, no runtime dependencies; self-updating
  (`dev-digest update`) and cleanly removable (`dev-digest uninstall [--complete]`).

## Install

**Prerequisites:** Go 1.22+ (`go version`), and `crontab` if you want the scheduled run
(standard on Linux/macOS).

**Option A — build a binary and put it on your PATH:**

```sh
git clone https://github.com/quangkhaidam93/dev-digest.git
cd dev-digest
go build -o dev-digest .
sudo mv dev-digest /usr/local/bin/      # or: mv dev-digest ~/.local/bin/  (ensure it's on $PATH)
```

**Option B — `go install`** (drops the binary in `$(go env GOBIN)` or `$(go env GOPATH)/bin`,
which should be on your `PATH`):

```sh
# from a clone:
go install .
# or, once published:
go install github.com/quangkhaidam93/dev-digest@latest
```

**First run** creates a starter config at `~/.config/dev-digest/config.toml`:

```sh
dev-digest              # launch the TUI; add sources (a), set provider/keys/schedule (s)
dev-digest cron install # schedule the daily run at [schedule].daily_time
```

Set the API key for your chosen provider in settings (or its env var), and — if you use
email — the SMTP fields. Then `dev-digest cron install` registers the daily job.

## Uninstall

```sh
dev-digest uninstall            # remove the cron entry and delete the dev-digest binary
dev-digest uninstall --complete # the above, plus delete all config and state
```

- **Normal** (`uninstall`) unregisters the cron job and removes the binary, but **keeps**
  your config (`~/.config/dev-digest/`) and state (`~/.local/state/dev-digest/`) so you can
  reinstall later without redoing setup.
- **Complete** (`uninstall --complete`) also deletes those two directories — everything the
  app created. (Any digests you had written to a `file` delivery folder are left alone.)

If you installed with `go install`, the binary lives in your Go bin dir; `uninstall` deletes
whichever binary is actually running. It never touches your crontab's other entries — only
its own managed block.

## Update & version

```sh
dev-digest version        # e.g. "dev-digest v1.2.3" plus module + Go version
dev-digest update         # fetch and build the latest release
dev-digest update v1.3.0  # or pin a specific version
```

`update` runs `go install <module>@latest` (or `@<version>`), so it needs the Go toolchain
on your `PATH` and the module to be published; it fetches, builds, and installs the new
binary into your Go bin dir, then reports the installed version.

> **Just released a tag and `update` still shows the old version?** The Go module proxy
> caches the `@latest` version list, so a brand-new tag can take a little while to appear.
> `update` detects this and tells you to either pin the version — `dev-digest update v1.2.3`
> — or bypass the proxy cache: `GOPROXY=direct dev-digest update`.

> **Version stamping:** a `go build`/`go install` from a clone reports Go's VCS-derived
> pseudo-version (commit + `dirty`). To bake in a release string, build with
> `go build -ldflags "-X main.version=v1.2.3" -o dev-digest .`. Installs via
> `go install …@vX.Y.Z` report that tag automatically.

## Usage

```sh
dev-digest                 # interactive TUI (manage sources, settings, preview, cron)
dev-digest run             # headless: fetch → summarize → deliver (this is what cron runs)
dev-digest cron install    # add a daily crontab entry (at [schedule].daily_time) running `dev-digest run`
dev-digest cron status     # show whether the crontab entry is registered (and its schedule)
dev-digest cron uninstall  # remove just the crontab entry
dev-digest uninstall       # remove the cron entry + the binary (keeps config/state)
dev-digest uninstall --complete  # also delete all config and state
dev-digest update          # fetch + build the latest release (go install …@latest)
dev-digest update v1.2.3   # or a specific version
dev-digest version         # print version, module, and Go version (also: -v)
dev-digest --config PATH … # use a specific config file
```

In the TUI: `a` add source · `e` edit · `d` delete · `r` **send now** (fetches and
actually delivers) · `p` preview (renders the digest without sending or changing state) ·
`s` settings · `c` install cron · `u` uninstall cron · `q` quit.

The **settings** screen (`s`) is grouped into *Summarization* (summarize on/off, provider,
model, API key, effort, max age), *Delivery* (file/email/webhook), and *Schedule* (the
daily run time, shown converting live to a cron expression). Enabling **Email** or
**Webhook** reveals its fields inline — SMTP host/port/username/password/from/recipients, or
webhook kind/URL — so the whole setup is doable in the TUI. Use `↑↓` to move, `space`/`←→`
to toggle or cycle a row, and just type to edit text fields; `esc` saves and returns (it
won't leave if an enabled channel is missing required fields). The SMTP password and API
keys are stored in the `0600` config; their env vars still work as a fallback.

## Configuration

Config lives at `~/.config/dev-digest/config.toml` (override with `--config`); state
(seen items, last-run times) lives at `~/.local/state/dev-digest/state.json`. The TUI
reads and rewrites the config; you can also edit it by hand.

```toml
[digest]
title     = "Dev Digest"
summarize = true            # false = raw aggregation, no API call
provider  = "anthropic"     # anthropic | gemini | openrouter
model     = "claude-opus-4-8"
effort    = "medium"        # low | medium | high (anthropic only)
max_age   = "24h"           # only include items published within this window; "" or "0" disables
question_when_empty = true   # on no-news days, send an AI learning question instead of nothing
# base_url = "…"            # optional: override the endpoint for gemini/openrouter

[[sources]]
type = "rss"
name = "Bytes"
url  = "https://bytes.dev/rss"

[[sources]]
type = "github"
name = "React"
repo = "facebook/react"
kind = "releases"           # releases | tags

[[sources]]
type     = "webpage"
name     = "Go release notes"
url      = "https://go.dev/doc/devel/release"
selector = "#content"       # optional CSS selector; defaults to <body>

[delivery.file]
enabled = true
dir     = "./out"
formats = ["md", "html"]    # writes out/<YYYY-MM-DD>.md / .html

[delivery.email]
enabled   = false
smtp_host = "smtp.example.com"
smtp_port = 587
username  = "me@example.com"
from      = "me@example.com"
to        = ["you@example.com"]

[delivery.webhook]
enabled = false
kind    = "slack"           # slack | discord | generic
url     = "https://hooks.slack.com/services/…"

[schedule]
daily_time = "08:00"        # local time to run; `cron install` converts this to a cron expr
```

After changing `daily_time` (in settings or the config), re-run `dev-digest cron install`
(or press `c` in the TUI) to update the crontab entry to the new time.

## LLM providers

Summarization works with three providers, selected by `[digest].provider`. Gemini and
OpenRouter both speak the OpenAI-compatible `/chat/completions` protocol, so dev-digest
talks to them through one client; Anthropic uses its native SDK. Switch providers in the
TUI settings screen (it also resets `model` to that provider's default) or by editing the
config. Each provider reads its API key from the environment:

| Provider | `model` example | API key env var | Default endpoint |
|---|---|---|---|
| `anthropic` | `claude-opus-4-8` | `ANTHROPIC_API_KEY` (or `ant` login) | native SDK |
| `gemini` | `gemini-2.5-flash` | `GEMINI_API_KEY` (or `GOOGLE_API_KEY`) | `generativelanguage.googleapis.com/v1beta/openai` |
| `openrouter` | `google/gemini-2.5-flash` | `OPENROUTER_API_KEY` | `openrouter.ai/api/v1` |

`effort` applies to Anthropic only. If summarization fails for any reason (missing key,
network, refusal), the run falls back to raw aggregation instead of failing.

**Setting the API key:** the fastest way is the TUI settings screen (`s`) — pick the
provider, then edit the **API key** field. Keys are stored per-provider in a `[keys]`
section of the config, and the file is written with `0600` permissions:

```toml
[keys]
gemini = "…"        # takes precedence over $GEMINI_API_KEY
```

If a provider has no key in `[keys]`, its environment variable is used as a fallback, so
either approach works.

## Secrets

Provider API keys and the SMTP password can be set in the TUI and are stored in the config
file (written `0600`), **or** supplied via environment variables — whichever you prefer. If
a secret isn't in the config, the matching env var below is used as a fallback:

| Variable | Purpose |
|---|---|
| `ANTHROPIC_API_KEY` / `GEMINI_API_KEY` (or `GOOGLE_API_KEY`) / `OPENROUTER_API_KEY` | API key for the selected provider. |
| `GITHUB_TOKEN` | Optional — lifts the GitHub API rate limit. |
| `DEV_DIGEST_SMTP_PASSWORD` | SMTP password for email delivery (if not set in config). |

## How a run works

1. Fetch every configured source.
2. Keep only items that are **new**: never seen before **and** — for items that carry a
   publish date (RSS entries, GitHub releases) — published within `max_age` (default 24h,
   so a daily run only picks up the last day). Items without a date (GitHub tags, webpage
   changes) aren't age-filtered; on a source's **first** run they're *seeded* (recorded as
   seen without being delivered) so a brand-new source doesn't dump its whole backlog — you
   start getting deltas from the next run. (A TUI "Run now" preview ignores seeding so you
   can see current content.)
3. If `summarize=true`, call the configured LLM provider with a JSON-schema-constrained
   request to produce the intro and per-item summaries; otherwise aggregate raw. On any
   summarization error, it falls back to raw aggregation so a run never fails silently.
4. Render Markdown + HTML and deliver to every enabled channel (channels are independent).
5. Record seen items so the next run skips them — but only if delivery succeeded, so a
   transient outage retries the same items next time.

## No-news days: question of the day

With `question_when_empty = true` (default), a run that finds nothing new doesn't go
silent — it asks the LLM for a short **learning question** rotating across a software-
engineering fact, a code smell, a data structure/algorithm, system design, concurrency,
databases, testing, or security, and delivers that through your channels. If the LLM
isn't available (no key, network error), it falls back to a small built-in question pool,
so a daily notification always arrives. Turn it off in settings or with
`question_when_empty = false`.

**Customizing the prompt:** in settings, select **Question prompt** and press `enter` to
open a full-screen editor for the system prompt that steers the question (`ctrl+r` resets
to the built-in default, `esc` saves). It's also `[digest].question_prompt` in the config;
leave it empty to use the default.
