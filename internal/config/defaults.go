package config

import (
	"os"
	"path/filepath"
	"time"
)

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		GitLab: GitLabConfig{
			RefreshIntervalS: 20,
		},
		Cache: CacheConfig{
			DBPath:             defaultCacheDBPath(),
			SyncIntervalS:      20,
			PrefetchWindowDays: 30,
		},
		Export: ExportConfig{
			Format:            "claude-xml",
			LLMCommand:        "claude -p",
			IncludeComments:   true,
			IncludeRelatedMRs: "stub",
		},
		UI: UIConfig{
			Theme:        "light",
			SidebarWidth: 30,
			WrapLines:    false,
		},
	}
}

// DefaultRefreshInterval returns the default tick interval.
func DefaultRefreshInterval() time.Duration {
	return 20 * time.Second
}

// defaultCacheDBPath returns the XDG-compliant cache DB path.
func defaultCacheDBPath() string {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(dir, "lazydev", "cache.db")
}
