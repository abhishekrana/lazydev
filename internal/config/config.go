package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config is the application configuration.
type Config struct {
	GitLab GitLabConfig `yaml:"gitlab"`
	Cache  CacheConfig  `yaml:"cache"`
	Export ExportConfig `yaml:"export"`
	UI     UIConfig     `yaml:"ui"`
}

// GitLabConfig holds GitLab-specific settings.
type GitLabConfig struct {
	URL              string   `yaml:"url"`
	Token            string   `yaml:"token"`
	Project          string   `yaml:"project"`
	AdditionalUsers  []string `yaml:"additional_users"`
	AIUser           string   `yaml:"ai_user"`
	RefreshIntervalS int      `yaml:"refresh_interval_s"`
}

// CacheConfig holds SQLite cache + sync settings.
type CacheConfig struct {
	DBPath             string `yaml:"db_path"`
	SyncIntervalS      int    `yaml:"sync_interval_s"`
	PrefetchWindowDays int    `yaml:"prefetch_window_days"`
}

// ExportConfig holds Claude/LLM export settings.
type ExportConfig struct {
	Format            string `yaml:"format"`      // "claude-xml" | "markdown"
	LLMCommand        string `yaml:"llm_command"` // e.g. "claude -p"
	IncludeComments   bool   `yaml:"include_comments"`
	IncludeRelatedMRs string `yaml:"include_related_mrs"` // "stub" | "full" | "none"
}

// UIConfig holds UI display settings.
type UIConfig struct {
	Theme        string `yaml:"theme"`
	SidebarWidth int    `yaml:"sidebar_width"`
	WrapLines    bool   `yaml:"wrap_lines"`
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
