package app

import (
	"context"
	"fmt"

	"github.com/abhishek-rana/lazydev/internal/config"
	gitlabpkg "github.com/abhishek-rana/lazydev/internal/gitlab"
)

// SharedState holds backend clients shared across tabs.
type SharedState struct {
	GitLabClient *gitlabpkg.Client
	Config       *config.Config
	Warnings     []string
	cancel       context.CancelFunc
}

// NewSharedState creates shared state, connecting to available backends.
func NewSharedState(cfg *config.Config) (*SharedState, error) {
	_, cancel := context.WithCancel(context.Background())

	state := &SharedState{
		Config: cfg,
		cancel: cancel,
	}

	gc, err := gitlabpkg.NewClient(cfg.GitLab.URL, cfg.GitLab.Token, cfg.GitLab.Project, cfg.GitLab.AdditionalUsers)
	if err == nil {
		state.GitLabClient = gc
	} else {
		state.Warnings = append(state.Warnings, fmt.Sprintf("GitLab: %v", err))
	}

	return state, nil
}

// Close cleans up all resources.
func (s *SharedState) Close() {
	s.cancel()
}
