package gitlab

import (
	"fmt"
	"os"
	"path/filepath"

	gitlab "gitlab.com/gitlab-org/api/client-go"
	"gopkg.in/yaml.v3"
)

// Client wraps the GitLab API client.
type Client struct {
	Raw       *gitlab.Client
	ProjectID string // project path e.g. "mygroup/myproject"
	UserID    int64  // authenticated user's ID
	Username  string // authenticated user's username
}

// NewClient creates a GitLab client with token discovery.
// Discovery order: explicit token → GITLAB_TOKEN env → glab CLI config.
func NewClient(url, token, project string) (*Client, error) {
	if project == "" {
		return nil, fmt.Errorf("gitlab project path is required")
	}

	// Discover URL and token.
	if url == "" {
		url = os.Getenv("GITLAB_URL")
	}
	if token == "" {
		token = os.Getenv("GITLAB_TOKEN")
	}

	// Fall back to glab CLI config.
	if url == "" || token == "" {
		glabURL, glabToken := readGlabConfig()
		if url == "" && glabURL != "" {
			url = glabURL
		}
		if token == "" && glabToken != "" {
			token = glabToken
		}
	}

	if url == "" {
		url = "https://gitlab.com"
	}
	if token == "" {
		return nil, fmt.Errorf("no GitLab token found (set gitlab.token in config, GITLAB_TOKEN env, or configure glab CLI)")
	}

	raw, err := gitlab.NewClient(token, gitlab.WithBaseURL(url+"/api/v4"))
	if err != nil {
		return nil, fmt.Errorf("creating gitlab client: %w", err)
	}

	c := &Client{
		Raw:       raw,
		ProjectID: project,
	}

	// Get authenticated user info.
	user, _, err := raw.Users.CurrentUser()
	if err != nil {
		return nil, fmt.Errorf("authenticating with gitlab: %w", err)
	}
	c.UserID = user.ID
	c.Username = user.Username

	return c, nil
}

// glabConfig represents the glab CLI config structure.
type glabConfig struct {
	Host  string                       `yaml:"host"`
	Hosts map[string]glabHostConfig    `yaml:"hosts"`
}

type glabHostConfig struct {
	Token       string `yaml:"token"`
	APIProtocol string `yaml:"api_protocol"`
	APIHost     string `yaml:"api_host"`
}

// readGlabConfig reads the glab CLI config file and returns (url, token).
func readGlabConfig() (string, string) {
	path := glabConfigPath()
	data, err := os.ReadFile(path) //nolint:gosec // config path is well-known
	if err != nil {
		return "", ""
	}

	var cfg glabConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return "", ""
	}

	host := cfg.Host
	if host == "" {
		host = "gitlab.com"
	}

	hostCfg, ok := cfg.Hosts[host]
	if !ok {
		return "", ""
	}

	protocol := hostCfg.APIProtocol
	if protocol == "" {
		protocol = "https"
	}
	apiHost := hostCfg.APIHost
	if apiHost == "" {
		apiHost = host
	}

	url := fmt.Sprintf("%s://%s", protocol, apiHost)
	return url, hostCfg.Token
}

func glabConfigPath() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "glab-cli", "config.yml")
}
