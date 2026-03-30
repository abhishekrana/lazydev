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

	// Build available tabs.
	var tabModels []ui.TabModel

	if state.DockerClient != nil {
		tabModels = append(tabModels, tabs.NewDockerTab(state.DockerClient, state.StreamMgr))
	}

	if state.KubeClient != nil {
		tabModels = append(tabModels, tabs.NewKubeTab(state.KubeClient, state.StreamMgr))
	}

	// All Logs tab.
	tabModels = append(tabModels, tabs.NewLogsTab())

	// Dashboard tab.
	tabModels = append(tabModels, tabs.NewDashboardTab(state.DockerClient, state.KubeClient))

	// GitLab tabs.
	if state.GitLabClient != nil {
		refreshS := cfg.GitLab.RefreshIntervalS
		tabModels = append(tabModels,
			tabs.NewIssuesTab(state.GitLabClient, refreshS),
			tabs.NewMRsTab(state.GitLabClient, refreshS),
			tabs.NewPipelinesTab(state.GitLabClient, refreshS),
		)
	}

	// Print warnings for backends that failed to connect.
	for _, w := range state.Warnings {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", w)
	}

	if len(tabModels) <= 1 {
		fmt.Fprintln(os.Stderr, "Error: No backends available. Ensure Docker is running, kubeconfig exists, or GitLab is configured.")
		os.Exit(1)
	}

	root := ui.NewRootModel(tabModels)

	p := tea.NewProgram(root)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
