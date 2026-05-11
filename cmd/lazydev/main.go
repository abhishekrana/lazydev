package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/abhishek-rana/lazydev/internal/app"
	"github.com/abhishek-rana/lazydev/internal/config"
	"github.com/abhishek-rana/lazydev/internal/ui"
	"github.com/abhishek-rana/lazydev/internal/ui/tabs"
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

	refreshS := cfg.GitLab.RefreshIntervalS
	tabModels := []ui.TabModel{
		tabs.NewIssuesTab(state.GitLabClient, refreshS),
		tabs.NewMRsTab(state.GitLabClient, refreshS),
	}

	for _, w := range state.Warnings {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", w)
	}

	root := ui.NewRootModel(tabModels)

	p := tea.NewProgram(root)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
