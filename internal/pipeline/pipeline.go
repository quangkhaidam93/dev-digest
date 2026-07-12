// Package pipeline orchestrates a single digest run: fetch every configured
// source, filter to items new since the last run, summarize (or aggregate),
// render, deliver, and record the seen items so the next run skips them.
package pipeline

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/quangkhaidam93/dev-digest/internal/config"
	"github.com/quangkhaidam93/dev-digest/internal/deliver"
	"github.com/quangkhaidam93/dev-digest/internal/digest"
	"github.com/quangkhaidam93/dev-digest/internal/sources"
	"github.com/quangkhaidam93/dev-digest/internal/store"
)

// Options configures a run.
type Options struct {
	Config            config.Config
	Store             *store.Store
	Now               time.Time
	Log               io.Writer // progress log (may be nil)
	Deliver           bool      // when false, build the digest but skip delivery + state write (preview)
	SummarizeOverride *bool     // when set, overrides cfg.Digest.Summarize (used by "run now")
}

// Result summarizes what a run produced.
type Result struct {
	Digest       digest.Digest
	NewItemCount int
	FetchErrors  []error
	Deliveries   []deliver.Result
}

// Run executes the pipeline and returns the built digest plus any per-stage
// errors. Fetch errors for individual sources are collected, not fatal.
func Run(ctx context.Context, opts Options) (Result, error) {
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	logf := func(format string, a ...any) {
		if opts.Log != nil {
			fmt.Fprintf(opts.Log, format+"\n", a...)
		}
	}

	var res Result
	var newItems []sources.Item
	// recorded holds the IDs to mark seen per source (delivered + seeded). A
	// source present with a nil slice still has its last-run advanced on commit.
	recorded := map[string][]string{}

	// Only include dated items published within maxAge of now (0 = no filter).
	maxAge := opts.Config.Digest.MaxAgeDuration()

	for _, sc := range opts.Config.Sources {
		src, err := sources.New(sc)
		if err != nil {
			res.FetchErrors = append(res.FetchErrors, err)
			logf("source %q: %v", sc.Name, err)
			continue
		}
		items, err := src.Fetch(ctx)
		if err != nil {
			res.FetchErrors = append(res.FetchErrors, err)
			logf("source %q: fetch failed: %v", sc.Name, err)
			continue
		}

		firstRun := opts.Store.LastRun(sc.Name).IsZero()
		if _, ok := recorded[sc.Name]; !ok {
			recorded[sc.Name] = nil // mark source as successfully fetched
		}

		sel := selectNew(opts.Store, sc.Name, items, firstRun, opts.Deliver, now, maxAge)
		newItems = append(newItems, sel.deliver...)
		recorded[sc.Name] = append(recorded[sc.Name], sel.record...)
		logf("source %q: %d items, %d new, %d too old, %d seeded",
			sc.Name, len(items), len(sel.deliver), sel.tooOld, sel.seeded)
	}

	res.NewItemCount = len(newItems)
	if len(newItems) == 0 {
		if opts.Config.Digest.QuestionWhenEmpty {
			logf("no new items; sending a question-of-the-day instead")
			apiKey := opts.Config.APIKey(opts.Config.Digest.ResolvedProvider())
			d := digest.GenerateQuestion(ctx, opts.Config.Digest, apiKey, now)
			res.Digest = d
			if opts.Deliver {
				res.Deliveries = deliver.Deliver(ctx, opts.Config.Delivery, d)
				for _, dr := range res.Deliveries {
					if dr.Err != nil {
						logf("delivery %s: FAILED: %v", dr.Channel, dr.Err)
					} else {
						logf("delivery %s: ok", dr.Channel)
					}
				}
			}
		} else {
			logf("no new items; nothing to deliver")
		}
		if err := commitState(opts, recorded, now); err != nil {
			return res, err
		}
		return res, nil
	}

	summarize := opts.Config.Digest.Summarize
	if opts.SummarizeOverride != nil {
		summarize = *opts.SummarizeOverride
	}

	var d digest.Digest
	if summarize {
		logf("summarizing %d items with %s (%s)…", len(newItems),
			opts.Config.Digest.ResolvedProvider(), opts.Config.Digest.ResolvedModel())
		var err error
		apiKey := opts.Config.APIKey(opts.Config.Digest.ResolvedProvider())
		d, err = digest.Summarize(ctx, opts.Config.Digest, apiKey, now, newItems)
		if err != nil {
			logf("summarize failed (%v); falling back to raw aggregation", err)
			d = digest.Aggregate(opts.Config.Digest.Title, now, newItems)
		}
	} else {
		d = digest.Aggregate(opts.Config.Digest.Title, now, newItems)
	}
	res.Digest = d

	if !opts.Deliver {
		return res, nil
	}

	res.Deliveries = deliver.Deliver(ctx, opts.Config.Delivery, d)
	anyFailed := false
	for _, dr := range res.Deliveries {
		if dr.Err != nil {
			anyFailed = true
			logf("delivery %s: FAILED: %v", dr.Channel, dr.Err)
		} else {
			logf("delivery %s: ok", dr.Channel)
		}
	}

	// Persist state only when no delivery failed, so a transient outage retries
	// the same items next run.
	if anyFailed {
		logf("skipping state update because a delivery failed")
		return res, nil
	}
	if err := commitState(opts, recorded, now); err != nil {
		return res, err
	}
	return res, nil
}

// selection is the result of choosing which of a source's fetched items to
// deliver and which IDs to mark seen.
type selection struct {
	deliver []sources.Item // items new enough to include in the digest
	record  []string       // IDs to mark seen (delivered + seeded)
	seeded  int            // dateless items recorded without delivering (first run)
	tooOld  int            // dated items dropped by the age window
}

// selectNew applies dedup, the age window, and first-run seeding to one source's
// items. Rules:
//   - already-seen items are skipped;
//   - a dated item older than now-maxAge is dropped (and left unrecorded);
//   - on the first real (delivering) run, dateless items are seeded (recorded,
//     not delivered) so the backlog isn't dumped;
//   - everything else is delivered and recorded.
func selectNew(st *store.Store, name string, items []sources.Item, firstRun, deliver bool, now time.Time, maxAge time.Duration) selection {
	var cutoff time.Time
	if maxAge > 0 {
		cutoff = now.Add(-maxAge)
	}
	var s selection
	for _, it := range items {
		if !st.IsNew(name, it.ID) {
			continue
		}
		dated := !it.Published.IsZero()
		if dated && maxAge > 0 && it.Published.Before(cutoff) {
			s.tooOld++
			continue
		}
		if deliver && firstRun && !dated {
			s.record = append(s.record, it.ID)
			s.seeded++
			continue
		}
		s.deliver = append(s.deliver, it)
		s.record = append(s.record, it.ID)
	}
	return s
}

// commitState records seen IDs and advances last-run for every successfully
// fetched source, then persists — but only for a real (delivering) run. Preview
// runs never mutate state.
func commitState(opts Options, recorded map[string][]string, now time.Time) error {
	if !opts.Deliver {
		return nil
	}
	for name, ids := range recorded {
		opts.Store.Record(name, ids, now)
	}
	if err := opts.Store.Save(); err != nil {
		return fmt.Errorf("save state: %w", err)
	}
	return nil
}
