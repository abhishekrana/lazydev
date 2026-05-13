package cache

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/abhishek-rana/lazydev/pkg/messages"
)

func TestStoreRoundtrip(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "cache.db")

	s, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	now := time.Now().Truncate(time.Second)
	issues := []messages.GitLabIssue{
		{
			IID:         1,
			ID:          101,
			ProjectID:   42,
			Title:       "Login times out after refresh",
			Description: "Auth token isn't being refreshed when the session expires.",
			State:       "opened",
			Labels:      []string{"bug", "auth"},
			Assignees:   []string{"alice"},
			Author:      "bob",
			WebURL:      "https://example.com/issues/1",
			CreatedAt:   now.Add(-2 * time.Hour),
			UpdatedAt:   now.Add(-time.Hour),
		},
		{
			IID:       2,
			Title:     "Add export to CSV",
			State:     "closed",
			Assignees: []string{"claude-bot", "alice"},
			UpdatedAt: now,
		},
	}
	if err := s.UpsertIssues(ctx, issues); err != nil {
		t.Fatalf("UpsertIssues: %v", err)
	}

	// Listing opens returns 1; listing all returns 2.
	open, err := s.ListIssues(ctx, Filter{})
	if err != nil {
		t.Fatalf("ListIssues open: %v", err)
	}
	if len(open) != 1 || open[0].IID != 1 {
		t.Fatalf("expected 1 open issue (IID=1), got %d items: %+v", len(open), open)
	}

	all, err := s.ListIssues(ctx, Filter{State: "all"})
	if err != nil {
		t.Fatalf("ListIssues all: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 all-state issues, got %d", len(all))
	}

	// Assignee filter works.
	aiq, err := s.ListIssues(ctx, Filter{State: "all", Assignee: "claude-bot"})
	if err != nil {
		t.Fatalf("ListIssues ai: %v", err)
	}
	if len(aiq) != 1 || aiq[0].IID != 2 {
		t.Fatalf("expected 1 ai-assigned issue (IID=2), got %+v", aiq)
	}

	// Label filter works.
	bugs, err := s.ListIssues(ctx, Filter{State: "all", Labels: []string{"bug"}})
	if err != nil {
		t.Fatalf("ListIssues bug: %v", err)
	}
	if len(bugs) != 1 || bugs[0].IID != 1 {
		t.Fatalf("expected 1 bug-labeled issue, got %+v", bugs)
	}

	// MaxUpdatedAt reflects the most recently updated row (IID=2 @ now).
	maxAt, err := s.MaxIssueUpdatedAt(ctx)
	if err != nil {
		t.Fatalf("MaxIssueUpdatedAt: %v", err)
	}
	if !maxAt.Equal(now) {
		t.Fatalf("MaxIssueUpdatedAt = %v, want %v", maxAt, now)
	}

	// FTS5 search finds the description word.
	hits, err := s.Search(ctx, "refresh", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 1 || hits[0].Kind != "issue" || hits[0].IID != 1 {
		t.Fatalf("expected 1 issue hit (IID=1), got %+v", hits)
	}

	// Notes upsert + FTS5 search of note body.
	if err := s.UpsertNotes(ctx, "issue", 1, []messages.GitLabNote{
		{Author: "alice", Body: "Reproduces on Firefox stable", CreatedAt: now},
	}); err != nil {
		t.Fatalf("UpsertNotes: %v", err)
	}
	hits, err = s.Search(ctx, "Firefox", 10)
	if err != nil {
		t.Fatalf("Search Firefox: %v", err)
	}
	if len(hits) != 1 || hits[0].IID != 1 {
		t.Fatalf("expected note-content search hit on issue 1, got %+v", hits)
	}

	// Get returns issue + notes + (empty) related.
	got, notes, related, err := s.GetIssue(ctx, 1)
	if err != nil {
		t.Fatalf("GetIssue: %v", err)
	}
	if got == nil || got.Title != "Login times out after refresh" {
		t.Fatalf("GetIssue wrong issue: %+v", got)
	}
	if len(notes) != 1 || notes[0].Author != "alice" {
		t.Fatalf("GetIssue notes wrong: %+v", notes)
	}
	if len(related) != 0 {
		t.Fatalf("GetIssue related should be empty, got %+v", related)
	}

	// Re-upsert (mutated title) should not duplicate.
	mutated := issues[0]
	mutated.Title = "Login times out (still)"
	if err := s.UpsertIssues(ctx, []messages.GitLabIssue{mutated}); err != nil {
		t.Fatalf("re-upsert: %v", err)
	}
	again, _, _, err := s.GetIssue(ctx, 1)
	if err != nil {
		t.Fatalf("GetIssue after re-upsert: %v", err)
	}
	if again.Title != "Login times out (still)" {
		t.Fatalf("expected mutated title, got %q", again.Title)
	}
}

func TestStorePrune(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "cache.db")
	s, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	now := time.Now()
	old := now.Add(-90 * 24 * time.Hour)

	// One old closed issue (prunable), one old open issue (kept).
	if err := s.UpsertIssues(ctx, []messages.GitLabIssue{
		{IID: 1, State: "closed", UpdatedAt: old},
		{IID: 2, State: "opened", UpdatedAt: old},
	}); err != nil {
		t.Fatalf("UpsertIssues: %v", err)
	}

	cutoff := now.Add(-30 * 24 * time.Hour)
	removed, err := s.PruneOlderThan(ctx, cutoff)
	if err != nil {
		t.Fatalf("PruneOlderThan: %v", err)
	}
	if removed != 1 {
		t.Fatalf("expected 1 row pruned, got %d", removed)
	}

	all, _ := s.ListIssues(ctx, Filter{State: "all"})
	if len(all) != 1 || all[0].IID != 2 {
		t.Fatalf("expected only IID=2 to survive, got %+v", all)
	}
}

func TestStoreMeta(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "cache.db")
	s, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	v, err := s.GetMeta(ctx, "last_full_sync")
	if err != nil || v != "" {
		t.Fatalf("expected empty meta, got %q err=%v", v, err)
	}
	if err := s.SetMeta(ctx, "last_full_sync", "2026-05-11T12:00:00Z"); err != nil {
		t.Fatalf("SetMeta: %v", err)
	}
	v, err = s.GetMeta(ctx, "last_full_sync")
	if err != nil || v != "2026-05-11T12:00:00Z" {
		t.Fatalf("SetMeta roundtrip failed: %q err=%v", v, err)
	}
}
