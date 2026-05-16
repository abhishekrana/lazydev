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

	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "search":
			os.Exit(cmdSearch(cfg, os.Args[2:]))
		case "issue":
			os.Exit(cmdIssue(cfg, os.Args[2:]))
		case "mr":
			os.Exit(cmdMR(cfg, os.Args[2:]))
		case "install-skill":
			os.Exit(cmdInstallSkill(os.Args[2:]))
		case helpFlagShort, "-h", helpFlagLong:
			printRootHelp()
			return
		case "tui":
			// explicit pass-through, fall through to TUI launch
		default:
			fmt.Fprintf(os.Stderr, "lazydev: unknown command %q\n\n", os.Args[1])
			printRootHelp()
			os.Exit(2)
		}
	}

	runTUI(cfg)
}

func printRootHelp() {
	fmt.Fprintln(os.Stderr, "lazydev — terminal cockpit for GitLab issues/MRs with Claude Code")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  lazydev                   launch the TUI (default)")
	fmt.Fprintln(os.Stderr, "  lazydev search <q>        FTS5 search across cached issues + MRs")
	fmt.Fprintln(os.Stderr, "  lazydev issue list|show   read cached issues as JSON")
	fmt.Fprintln(os.Stderr, "  lazydev mr    list|show   read cached MRs as JSON")
	fmt.Fprintln(os.Stderr, "  lazydev install-skill     install the Claude Code skill (~/.claude/skills/lazydev/)")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Run any subcommand with --help for its flags.")
}

func runTUI(cfg *config.Config) {
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

	root := ui.NewRootModel(tabModels)
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
