package gitlab

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	gitlab "gitlab.com/gitlab-org/api/client-go"
	"gopkg.in/yaml.v3"
)

// Client wraps the GitLab API client.
type Client struct {
	Raw        *gitlab.Client
	ProjectID  string  // project path e.g. "mygroup/myproject"
	UserID     int64   // authenticated user's ID
	Username   string  // authenticated user's username
	UserIDs    []int64 // all user IDs to track (self + additional users like bots)
	Usernames  []string // all usernames to track
}

// NewClient creates a GitLab client with token discovery.
// Discovery order: explicit token → GITLAB_TOKEN env → glab CLI config.
// additionalUsers are extra usernames (e.g. bot accounts) to include in "my" queries.
func NewClient(url, token, project string, additionalUsers []string) (*Client, error) {
	// Auto-detect project from git remote if not set.
	if project == "" {
		project = detectProjectFromGitRemote(url)
	}
	if project == "" {
		return nil, fmt.Errorf("gitlab project not found (set gitlab.project in config or run from a git repo with a GitLab remote)")
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
	c.UserIDs = []int64{user.ID}
	c.Usernames = []string{user.Username}

	// Resolve additional users (e.g. bot accounts).
	for _, username := range additionalUsers {
		users, _, err := raw.Users.ListUsers(&gitlab.ListUsersOptions{
			Username: gitlab.Ptr(username),
		})
		if err == nil && len(users) > 0 {
			c.UserIDs = append(c.UserIDs, users[0].ID)
			c.Usernames = append(c.Usernames, users[0].Username)
		}
	}

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
// glab uses "!!null" YAML tags for tokens which causes Go's YAML parser to
// error, so we use a combination of YAML parsing and raw text extraction.
func readGlabConfig() (string, string) {
	path := glabConfigPath()
	data, err := os.ReadFile(path) //nolint:gosec // config path is well-known
	if err != nil {
		return "", ""
	}

	// Try YAML first for host field (top-level, always parses fine).
	var cfg glabConfig
	_ = yaml.Unmarshal(data, &cfg) // ignore error — !!null tag breaks full parse

	host := cfg.Host
	if host == "" {
		host = "gitlab.com"
	}

	// Try structured parsing first.
	hostCfg, ok := cfg.Hosts[host]
	if ok && hostCfg.Token != "" {
		protocol := hostCfg.APIProtocol
		if protocol == "" {
			protocol = "https"
		}
		apiHost := hostCfg.APIHost
		if apiHost == "" {
			apiHost = host
		}
		return fmt.Sprintf("%s://%s", protocol, apiHost), hostCfg.Token
	}

	// Fall back to raw text extraction (handles !!null YAML tag).
	token := extractTokenFromRaw(data, host)
	if token == "" {
		return "", ""
	}
	return fmt.Sprintf("https://%s", host), token
}

// extractTokenFromRaw extracts the token from glab config when YAML parsing
// fails due to !!null tag. Looks for "token: !!null <actual-token>" pattern.
func extractTokenFromRaw(data []byte, host string) string {
	lines := strings.Split(string(data), "\n")
	inHost := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Check if we're in the right host section.
		if strings.HasPrefix(trimmed, host+":") {
			inHost = true
			continue
		}
		// Another top-level host starts.
		if inHost && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") && trimmed != "" {
			inHost = false
		}
		if inHost && strings.Contains(trimmed, "token:") {
			// Handle "token: !!null glpat-xxx" or "token: glpat-xxx"
			parts := strings.SplitN(trimmed, "token:", 2)
			if len(parts) == 2 {
				val := strings.TrimSpace(parts[1])
				val = strings.TrimPrefix(val, "!!null")
				val = strings.TrimSpace(val)
				if val != "" {
					return val
				}
			}
		}
	}
	return ""
}

// detectProjectFromGitRemote extracts the GitLab project path from the current
// git repo's remote URL. Supports both SSH and HTTPS formats:
//   - git@gitlab.com:group/project.git → group/project
//   - https://gitlab.com/group/project.git → group/project
func detectProjectFromGitRemote(gitlabURL string) string {
	out, err := exec.Command("git", "remote", "get-url", "origin").Output() //nolint:gosec // well-known command
	if err != nil {
		return ""
	}
	remote := strings.TrimSpace(string(out))
	if remote == "" {
		return ""
	}

	// Determine which host to match against.
	host := "gitlab.com"
	if gitlabURL != "" {
		// Extract host from URL like "https://gitlab.example.com"
		h := strings.TrimPrefix(gitlabURL, "https://")
		h = strings.TrimPrefix(h, "http://")
		h = strings.Split(h, "/")[0]
		if h != "" {
			host = h
		}
	}

	// SSH format: git@gitlab.com:group/project.git
	if strings.HasPrefix(remote, "git@"+host+":") {
		path := strings.TrimPrefix(remote, "git@"+host+":")
		path = strings.TrimSuffix(path, ".git")
		return path
	}

	// HTTPS format: https://gitlab.com/group/project.git
	if strings.Contains(remote, host) {
		idx := strings.Index(remote, host)
		path := remote[idx+len(host):]
		path = strings.TrimPrefix(path, "/")
		path = strings.TrimSuffix(path, ".git")
		if path != "" {
			return path
		}
	}

	return ""
}

func glabConfigPath() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "glab-cli", "config.yml")
}
