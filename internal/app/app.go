package app

import (
	"context"

	"github.com/abhishek-rana/lazydk/internal/config"
	"github.com/abhishek-rana/lazydk/internal/docker"
	logpkg "github.com/abhishek-rana/lazydk/internal/log"
)

// SharedState holds backend clients shared across tabs.
type SharedState struct {
	DockerClient *docker.Client
	StreamMgr    *logpkg.StreamManager
	Config       *config.Config
	cancel       context.CancelFunc
}

// NewSharedState creates shared state, connecting to available backends.
func NewSharedState(cfg *config.Config) (*SharedState, error) {
	ctx, cancel := context.WithCancel(context.Background())

	state := &SharedState{
		StreamMgr: logpkg.NewStreamManager(ctx),
		Config:    cfg,
		cancel:    cancel,
	}

	// Try Docker.
	dc, err := docker.NewClient(cfg.Docker.Host)
	if err == nil {
		state.DockerClient = dc
	}

	return state, nil
}

// Close cleans up all resources.
func (s *SharedState) Close() {
	s.cancel()
	if s.StreamMgr != nil {
		s.StreamMgr.StopAll()
	}
	if s.DockerClient != nil {
		_ = s.DockerClient.Close()
	}
}
