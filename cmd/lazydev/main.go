package main

import (
	"fmt"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/abhishek-rana/lazydev/internal/app"
	"github.com/abhishek-rana/lazydev/internal/cache"
	"github.com/abhishek-rana/lazydev/internal/config"
	"github.com/abhishek-rana/lazydev/internal/ui"
	"github.com/abhishek-rana/lazydev/internal/ui/tabs"
	"github.com/abhishek-rana/lazydev/pkg/messages"
)

func main() {
	cfg := config.Load()

	state, err := app.NewSharedState(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer state.Close()

	if state.GitLabClient == nil {
		fmt.Fprintln(os.Stderr, "Error: GitLab is not configured. Set GITLAB_TOKEN or configure ~/.config/lazydev/config.yaml.")
		for _, w := range state.Warnings {
			fmt.Fprintf(os.Stderr, "  %s\n", w)
		}
		os.Exit(1)
	}

	for _, w := range state.Warnings {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", w)
	}

	opts := &tabs.Options{
		AIUser:       cfg.GitLab.AIUser,
		ExportFormat: cfg.Export.Format,
		LLMCommand:   cfg.Export.LLMCommand,
		ClaudeEnv:    state.ClaudeEnv,
		ClaudeStore:  state.ClaudeStore,
		TmuxSession:  cfg.Claude.TmuxSession,
	}
	tabModels := []ui.TabModel{
		tabs.NewIssuesTab(state.GitLabClient, state.Cache, state.Syncer, opts),
		tabs.NewMRsTab(state.GitLabClient, state.Cache, state.Syncer, opts),
		tabs.NewClaudeTab(opts),
	}

	root := ui.NewRootModel(tabModels, state.Views)
	p := tea.NewProgram(root)

	// Forward sync events into the Bubble Tea program before starting
	// the syncer — the first event (state="prefetching") fires within
	// milliseconds of Start().
	go forwardSyncEvents(p, state.Syncer)
	state.StartSync()

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func forwardSyncEvents(p *tea.Program, syncer *cache.Syncer) {
	if syncer == nil {
		return
	}
	for ev := range syncer.Events() {
		p.Send(messages.SyncStatusMsg{
			State:      ev.State,
			Progress:   ev.Progress,
			LastSyncAt: ev.LastSyncAt,
			Err:        ev.Err,
		})
		if ev.Kind != "" {
			p.Send(messages.CacheUpdatedMsg{Kind: ev.Kind})
		}
	}
	// Channel closed — silence one final "offline" so the UI doesn't
	// look stuck mid-sync after a forced shutdown.
	p.Send(messages.SyncStatusMsg{State: "offline", LastSyncAt: time.Now()})
}
