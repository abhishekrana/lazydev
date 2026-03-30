package config

import "time"

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		Docker: DockerConfig{
			Host:             "", // auto-detect
			ComposeDetection: true,
		},
		Kubernetes: KubernetesConfig{
			Kubeconfig: "", // auto-detect
			Context:    "", // current context
		},
		UI: UIConfig{
			Theme:            "dark",
			SidebarWidth:     30,
			LogBufferSize:    10000,
			BatchIntervalMs:  50,
			ShowTimestamps:   true,
			WrapLines:        false,
			RefreshIntervalS: 5,
		},
	}
}

// DefaultRefreshInterval returns the default tick interval.
func DefaultRefreshInterval() time.Duration {
	return 5 * time.Second
}

// DefaultBatchInterval returns the default log batch interval.
func DefaultBatchInterval() time.Duration {
	return 50 * time.Millisecond
}

// DefaultLogBufferSize returns the default ring buffer size.
func DefaultLogBufferSize() int {
	return 10000
}
