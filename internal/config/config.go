package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config is the application configuration.
type Config struct {
	Docker     DockerConfig     `yaml:"docker"`
	Kubernetes KubernetesConfig `yaml:"kubernetes"`
	UI         UIConfig         `yaml:"ui"`
}

// DockerConfig holds Docker-specific settings.
type DockerConfig struct {
	Host             string `yaml:"host"`
	ComposeDetection bool   `yaml:"compose_detection"`
}

// KubernetesConfig holds Kubernetes-specific settings.
type KubernetesConfig struct {
	Kubeconfig string   `yaml:"kubeconfig"`
	Context    string   `yaml:"context"`
	Namespaces []string `yaml:"namespaces"`
}

// UIConfig holds UI display settings.
type UIConfig struct {
	Theme            string `yaml:"theme"`
	SidebarWidth     int    `yaml:"sidebar_width"`
	LogBufferSize    int    `yaml:"log_buffer_size"`
	BatchIntervalMs  int    `yaml:"batch_interval_ms"`
	ShowTimestamps   bool   `yaml:"timestamps"`
	WrapLines        bool   `yaml:"wrap_lines"`
	RefreshIntervalS int    `yaml:"refresh_interval_s"`
}

// Load reads configuration from the default path or returns defaults.
func Load() *Config {
	cfg := DefaultConfig()

	path := configPath()
	data, err := os.ReadFile(path) //nolint:gosec // config path is not user-controlled
	if err != nil {
		return cfg
	}

	_ = yaml.Unmarshal(data, cfg)
	return cfg
}

// configPath returns the XDG-compliant config file path.
func configPath() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "lazydev", "config.yaml")
}
