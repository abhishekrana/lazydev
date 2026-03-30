package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types/container"
)

// RestartContainer restarts the container with the given ID.
func (c *Client) RestartContainer(ctx context.Context, id string) error {
	if err := c.Raw.ContainerRestart(ctx, id, container.StopOptions{}); err != nil {
		return fmt.Errorf("restart container %s: %w", id, err)
	}
	return nil
}

// StopContainer stops the container with the given ID.
func (c *Client) StopContainer(ctx context.Context, id string) error {
	if err := c.Raw.ContainerStop(ctx, id, container.StopOptions{}); err != nil {
		return fmt.Errorf("stop container %s: %w", id, err)
	}
	return nil
}

// RemoveContainer removes the container with the given ID.
func (c *Client) RemoveContainer(ctx context.Context, id string) error {
	if err := c.Raw.ContainerRemove(ctx, id, container.RemoveOptions{}); err != nil {
		return fmt.Errorf("remove container %s: %w", id, err)
	}
	return nil
}
