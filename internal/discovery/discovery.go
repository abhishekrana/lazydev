package discovery

import (
	"context"
	"os"
	"time"

	"github.com/abhishek-rana/lazydk/internal/docker"
	"github.com/abhishek-rana/lazydk/pkg/messages"
)

// DiscoveryResult holds the outcome of backend detection.
type DiscoveryResult = messages.DiscoveryResultMsg

// Discover probes for available backends (Docker, Kubernetes).
// dockerHost may be empty to use defaults; kubeconfig is the path to a
// kubeconfig file (empty uses ~/.kube/config).
func Discover(dockerHost, kubeconfig string) DiscoveryResult {
	result := DiscoveryResult{}

	// --- Docker ---
	result.DockerAvailable, result.DockerHost = probeDocker(dockerHost, &result)

	// --- Kubernetes ---
	result.KubeAvailable = probeKube(kubeconfig, &result)

	return result
}

// probeDocker tries to connect to Docker and ping the daemon.
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

	// Resolve the effective host for display.
	effectiveHost := host
	if effectiveHost == "" {
		effectiveHost = cli.Raw.DaemonHost()
	}

	return true, effectiveHost
}

// probeKube checks whether a kubeconfig file exists.
func probeKube(kubeconfig string, result *DiscoveryResult) bool {
	if kubeconfig == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			result.Warnings = append(result.Warnings, "Kubernetes: cannot determine home directory: "+err.Error())
			return false
		}
		kubeconfig = home + "/.kube/config"
	}

	info, err := os.Stat(kubeconfig)
	if err != nil {
		result.Warnings = append(result.Warnings, "Kubernetes: kubeconfig not found at "+kubeconfig)
		return false
	}
	if info.IsDir() {
		result.Warnings = append(result.Warnings, "Kubernetes: kubeconfig path is a directory: "+kubeconfig)
		return false
	}

	result.KubeContext = kubeconfig
	return true
}
