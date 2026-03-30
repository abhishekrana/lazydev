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

	if len(tabModels) <= 1 {
		fmt.Fprintln(os.Stderr, "Error: No backends available. Ensure Docker is running or kubeconfig exists.")
		os.Exit(1)
	}

	root := ui.NewRootModel(tabModels)

	p := tea.NewProgram(root)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
