package cache

import (
	"context"
	"errors"
	"math"
	"sync/atomic"
	"time"

	"github.com/abhishek-rana/lazydev/pkg/messages"
)

// Source abstracts the GitLab fetch surface needed by the Syncer.
// *gitlab.Client satisfies it via the ListIssuesUpdatedAfter and
// ListMRsUpdatedAfter helpers in internal/gitlab/sync.go.
//
// Keeping this an interface lets cache stay independent of the gitlab
// package (and of the heavy gitlab client-go dependency).
type Source interface {
	ListMRsUpdatedAfter(t time.Time, state string) ([]messages.GitLabMR, error)
	// StreamWorkItemsUpdatedAfter replaces ListIssuesUpdatedAfter: one
	// GraphQL request per page returns title/state/widgets together.
	StreamWorkItemsUpdatedAfter(t time.Time, onPage func(messages.WorkItemPage) error) error
}

// SyncEvent is what the Syncer publishes after every tick. main.go
// converts it to a bubbletea message and forwards to the program.
type SyncEvent struct {
	// State is the syncer's current high-level state.
	// One of: "prefetching", "syncing", "idle", "offline".
	State string

	// Kind names which cache table was updated, when applicable.
	// Empty for status-only events. One of: "issues", "mrs", "".
	Kind string

	// Progress is a human-readable progress string (e.g. "120/450"),
	// non-empty during prefetch.
	Progress string

	// LastSyncAt is the wall-clock time of the most recent successful
	// tick (any kind).
	LastSyncAt time.Time

	// Err is non-nil on failed ticks. The syncer keeps running and
	// will retry; readers should surface this in a status bar but
	// not abort.
	Err error
}

// Syncer keeps the cache in sync with GitLab.
//
// On Start: if the cache is empty (or last_full_sync is older than
// prefetchWindow) it backfills the last prefetchWindow of activity.
// Then every syncInterval it does an incremental updated_after fetch.
// SyncNow() nudges an immediate incremental tick.
type Syncer struct {
	store          *Store
	source         Source
	syncInterval   time.Duration
	prefetchWindow time.Duration
	events         chan SyncEvent
	nowCh          chan struct{}
	stopped        atomic.Bool
}

// NewSyncer constructs a Syncer. Call Start() to launch its goroutine
// and Events() to consume status events. The channel has a small
// buffer so a slow reader can't stall the sync loop.
func NewSyncer(store *Store, source Source, syncInterval, prefetchWindow time.Duration) *Syncer {
	if syncInterval <= 0 {
		syncInterval = 20 * time.Second
	}
	if prefetchWindow <= 0 {
		prefetchWindow = 30 * 24 * time.Hour
	}
	return &Syncer{
		store:          store,
		source:         source,
		syncInterval:   syncInterval,
		prefetchWindow: prefetchWindow,
		events:         make(chan SyncEvent, 8),
		nowCh:          make(chan struct{}, 1),
	}
}

// Events is the read-only side of the event channel.
func (s *Syncer) Events() <-chan SyncEvent { return s.events }

// SyncNow requests an immediate incremental tick. Non-blocking; a
// pending nudge is coalesced.
func (s *Syncer) SyncNow() {
	select {
	case s.nowCh <- struct{}{}:
	default:
	}
}

// Start launches the sync goroutine. It returns immediately; cancel
// ctx (or call Close on the parent app) to stop. Start is safe to
// call once per Syncer.
func (s *Syncer) Start(ctx context.Context) {
	go s.run(ctx)
}

func (s *Syncer) run(ctx context.Context) {
	defer close(s.events)

	// Empty cache → prefetch; otherwise the incremental loop catches up
	// from MaxIssueUpdatedAt / MaxMRUpdatedAt. `rm cache.db` is the
	// supported way to force a fresh prefetch (or wait for a schema
	// bump in migrate()).
	maxIss, _ := s.store.MaxIssueUpdatedAt(ctx)
	maxMR, _ := s.store.MaxMRUpdatedAt(ctx)
	if maxIss.IsZero() && maxMR.IsZero() {
		if err := s.prefetch(ctx); err != nil {
			s.emit(SyncEvent{State: "offline", Err: err, LastSyncAt: time.Now()})
		}
	}

	// Initial incremental sync to catch anything that changed during
	// the (possibly slow) prefetch.
	s.incremental(ctx)

	ticker := time.NewTicker(s.syncInterval)
	defer ticker.Stop()

	// Exponential backoff for consecutive failures, capped at 2 minutes.
	failures := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.nowCh:
			// Manual nudge — reset backoff and run.
			failures = 0
			ok := s.incremental(ctx)
			if !ok {
				failures++
			}
		case <-ticker.C:
			if failures > 0 {
				delay := time.Duration(math.Min(120, math.Pow(2, float64(failures)))) * time.Second
				select {
				case <-ctx.Done():
					return
				case <-time.After(delay):
				}
			}
			ok := s.incremental(ctx)
			if ok {
				failures = 0
			} else {
				failures++
			}
		}
	}
}

// prefetch paginates everything in the prefetchWindow into the cache.
// Sends progress events as it goes.
func (s *Syncer) prefetch(ctx context.Context) error {
	since := time.Now().Add(-s.prefetchWindow)

	s.emit(SyncEvent{State: "prefetching", LastSyncAt: time.Now()})

	if err := s.syncIssuesBulk(ctx, since); err != nil {
		return err
	}

	mrs, err := s.source.ListMRsUpdatedAfter(since, "all")
	if err != nil {
		return err
	}
	if err := s.store.UpsertMRs(ctx, mrs); err != nil {
		return err
	}
	s.emit(SyncEvent{State: "prefetching", Kind: "mrs",
		Progress:   formatProgress(len(mrs), len(mrs)),
		LastSyncAt: time.Now()})
	return nil
}

// syncIssuesBulk streams work-items pages from the source, upserting
// each page (issues + their linked / child relations) in turn. Shared
// between prefetch and incremental — only the `since` anchor differs.
func (s *Syncer) syncIssuesBulk(ctx context.Context, since time.Time) error {
	total := 0
	return s.source.StreamWorkItemsUpdatedAfter(since, func(page messages.WorkItemPage) error {
		if err := s.store.UpsertIssues(ctx, page.Issues); err != nil {
			return err
		}
		for iid, items := range page.Linked {
			if err := s.store.UpsertLinkedItems(ctx, iid, items); err != nil {
				return err
			}
		}
		for iid, items := range page.Children {
			if err := s.store.UpsertChildItems(ctx, iid, items); err != nil {
				return err
			}
		}
		total += len(page.Issues)
		s.emit(SyncEvent{State: "prefetching", Kind: "issues",
			Progress:   formatProgress(total, total),
			LastSyncAt: time.Now()})
		return nil
	})
}

// incremental fetches items updated after the cached high-water mark
// (minus a small overlap to avoid races) and upserts them. Returns
// true on success.
func (s *Syncer) incremental(ctx context.Context) bool {
	s.emit(SyncEvent{State: "syncing", LastSyncAt: time.Now()})

	if err := s.syncKind(ctx, "issues"); err != nil {
		s.emit(SyncEvent{State: "offline", Err: err, LastSyncAt: time.Now()})
		return false
	}
	if err := s.syncKind(ctx, "mrs"); err != nil {
		s.emit(SyncEvent{State: "offline", Err: err, LastSyncAt: time.Now()})
		return false
	}

	s.emit(SyncEvent{State: "idle", LastSyncAt: time.Now()})
	return true
}

func (s *Syncer) syncKind(ctx context.Context, kind string) error {
	var (
		anchor time.Time
		err    error
		count  int
	)

	switch kind {
	case "issues":
		anchor, err = s.store.MaxIssueUpdatedAt(ctx)
		if err != nil {
			return err
		}
		if !anchor.IsZero() {
			anchor = anchor.Add(-time.Minute) // overlap window
		}
		// Count via callback so we can avoid buffering the full
		// result set; syncIssuesBulk emits its own progress events.
		if err := s.source.StreamWorkItemsUpdatedAfter(anchor, func(page messages.WorkItemPage) error {
			if err := s.store.UpsertIssues(ctx, page.Issues); err != nil {
				return err
			}
			for iid, items := range page.Linked {
				if err := s.store.UpsertLinkedItems(ctx, iid, items); err != nil {
					return err
				}
			}
			for iid, items := range page.Children {
				if err := s.store.UpsertChildItems(ctx, iid, items); err != nil {
					return err
				}
			}
			count += len(page.Issues)
			return nil
		}); err != nil {
			return err
		}

	case "mrs":
		anchor, err = s.store.MaxMRUpdatedAt(ctx)
		if err != nil {
			return err
		}
		if !anchor.IsZero() {
			anchor = anchor.Add(-time.Minute)
		}
		items, fetchErr := s.source.ListMRsUpdatedAfter(anchor, "all")
		if fetchErr != nil {
			return fetchErr
		}
		if err := s.store.UpsertMRs(ctx, items); err != nil {
			return err
		}
		count = len(items)

	default:
		return errors.New("unknown kind: " + kind)
	}

	if count > 0 {
		s.emit(SyncEvent{State: "syncing", Kind: kind, LastSyncAt: time.Now()})
	}
	return nil
}

func (s *Syncer) emit(e SyncEvent) {
	if s.stopped.Load() {
		return
	}
	select {
	case s.events <- e:
	default:
		// Drop on overflow — status events are not load-bearing.
	}
}

func formatProgress(n, total int) string {
	if total == 0 {
		return ""
	}
	if n == total {
		return countOnly(total)
	}
	return countOnly(n) + "/" + countOnly(total)
}

func countOnly(n int) string {
	// Tiny helper so we can swap in i18n number formatting later.
	return itoa(n)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	buf := make([]byte, 0, 12)
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	if neg {
		buf = append([]byte{'-'}, buf...)
	}
	return string(buf)
}
