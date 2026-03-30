package discovery

import (
	"context"
	"time"

	"github.com/abhishek-rana/lazydev/internal/docker"
	"github.com/abhishek-rana/lazydev/internal/kube"
	"github.com/abhishek-rana/lazydev/pkg/messages"
)

// DiscoveryResult holds the outcome of backend detection.
type DiscoveryResult = messages.DiscoveryResultMsg

// Discover probes for available backends (Docker, Kubernetes).
func Discover(dockerHost, kubeconfig string) DiscoveryResult {
	result := DiscoveryResult{}

	result.DockerAvailable, result.DockerHost = probeDocker(dockerHost, &result)
	result.KubeAvailable = probeKube(kubeconfig, &result)

	return result
}

func probeDocker(host string, result *DiscoveryResult) (bool, string) {
	cli, err := docker.NewClient(host)
	if err != nil {
		result.Warnings = append(result.Warnings, "Docker: "+err.Error())
		return false, host
	}
	defer func() { _ = cli.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := cli.Ping(ctx); err != nil {
		result.Warnings = append(result.Warnings, "Docker: "+err.Error())
		return false, host
	}

	effectiveHost := host
	if effectiveHost == "" {
		effectiveHost = cli.Raw.DaemonHost()
	}

	return true, effectiveHost
}

func probeKube(kubeconfig string, result *DiscoveryResult) bool {
	kc, err := kube.NewClient(kubeconfig)
	if err != nil {
		result.Warnings = append(result.Warnings, "Kubernetes: "+err.Error())
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := kc.Ping(ctx); err != nil {
		result.Warnings = append(result.Warnings, "Kubernetes: "+err.Error())
		return false
	}

	result.KubeContext = kc.CurrentContext()
	return true
}
