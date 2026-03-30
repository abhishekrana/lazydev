package app

import (
	"context"
	"fmt"

	"github.com/abhishek-rana/lazydev/internal/config"
	"github.com/abhishek-rana/lazydev/internal/docker"
	gitlabpkg "github.com/abhishek-rana/lazydev/internal/gitlab"
	"github.com/abhishek-rana/lazydev/internal/kube"
	logpkg "github.com/abhishek-rana/lazydev/internal/log"
)

// SharedState holds backend clients shared across tabs.
type SharedState struct {
	DockerClient *docker.Client
	KubeClient   *kube.Client
	GitLabClient *gitlabpkg.Client
	StreamMgr    *logpkg.StreamManager
	Config       *config.Config
	Warnings     []string
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

	// Try Kubernetes.
	kc, err := kube.NewClient(cfg.Kubernetes.Kubeconfig)
	if err == nil {
		state.KubeClient = kc
	}

	// Try GitLab.
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
	if s.StreamMgr != nil {
		s.StreamMgr.StopAll()
	}
	if s.DockerClient != nil {
		_ = s.DockerClient.Close()
	}
}
