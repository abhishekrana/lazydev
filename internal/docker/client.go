package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/client"
)

// Client wraps the Docker API client.
type Client struct {
	Raw *client.Client
}

// NewClient creates a new Docker client. If host is empty, it uses
// the default Docker socket (DOCKER_HOST env var, then the platform default).
func NewClient(host string) (*Client, error) {
	opts := []client.Opt{
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	}
	if host != "" {
		opts = append(opts, client.WithHost(host))
	}

	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("docker client: %w", err)
	}

	return &Client{Raw: cli}, nil
}

// Ping checks connectivity with the Docker daemon.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.Raw.Ping(ctx)
	if err != nil {
		return fmt.Errorf("docker ping: %w", err)
	}
	return nil
}

// Close releases the underlying Docker client resources.
func (c *Client) Close() error {
	return c.Raw.Close()
}
