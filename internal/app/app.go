package app

import (
	"context"
	"fmt"
	"time"

	"github.com/abhishek-rana/lazydev/internal/cache"
	"github.com/abhishek-rana/lazydev/internal/config"
	gitlabpkg "github.com/abhishek-rana/lazydev/internal/gitlab"
	"github.com/abhishek-rana/lazydev/internal/views"
)

// SharedState holds backend clients shared across tabs.
type SharedState struct {
	GitLabClient *gitlabpkg.Client
	Cache        *cache.Store
	Syncer       *cache.Syncer
	Views        *views.Store
	Config       *config.Config
	Warnings     []string
	ctx          context.Context
	cancel       context.CancelFunc
}

// NewSharedState creates shared state, connecting to available backends
// and opening the cache. The Syncer is constructed but not started —
// the caller is expected to wire its event channel into the UI program
// first, then call SharedState.StartSync.
func NewSharedState(cfg *config.Config) (*SharedState, error) {
	ctx, cancel := context.WithCancel(context.Background())

	state := &SharedState{
		Config: cfg,
		ctx:    ctx,
		cancel: cancel,
	}

	gc, err := gitlabpkg.NewClient(cfg.GitLab.URL, cfg.GitLab.Token, cfg.GitLab.Project, cfg.GitLab.AdditionalUsers)
	if err == nil {
		state.GitLabClient = gc
	} else {
		state.Warnings = append(state.Warnings, fmt.Sprintf("GitLab: %v", err))
	}

	store, err := cache.Open(ctx, cfg.Cache.DBPath)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("cache: %w", err)
	}
	state.Cache = store

	vs, err := views.Load(views.DefaultPath())
	if err != nil {
		state.Warnings = append(state.Warnings, fmt.Sprintf("views: %v", err))
	} else {
		state.Views = vs
	}

	if state.GitLabClient != nil {
		syncInterval := time.Duration(cfg.Cache.SyncIntervalS) * time.Second
		window := time.Duration(cfg.Cache.PrefetchWindowDays) * 24 * time.Hour
		state.Syncer = cache.NewSyncer(store, state.GitLabClient, syncInterval, window)
	}

	return state, nil
}

// StartSync launches the syncer goroutine.
func (s *SharedState) StartSync() {
	if s.Syncer != nil {
		s.Syncer.Start(s.ctx)
	}
}

// Close cleans up all resources.
func (s *SharedState) Close() {
	s.cancel()
	if s.Cache != nil {
		_ = s.Cache.Close()
	}
}
