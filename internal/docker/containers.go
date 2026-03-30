package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/docker/docker/api/types/container"

	"github.com/abhishek-rana/lazydk/pkg/messages"
)

// ListContainers returns all containers mapped to the shared Container type.
func (c *Client) ListContainers(ctx context.Context) ([]messages.Container, error) {
	raw, err := c.Raw.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}

	containers := make([]messages.Container, 0, len(raw))
	for _, r := range raw {
		name := ""
		if len(r.Names) > 0 {
			// Docker prefixes names with "/".
			name = r.Names[0]
			if len(name) > 0 && name[0] == '/' {
				name = name[1:]
			}
		}

		group := "standalone"
		if project, ok := r.Labels["com.docker.compose.project"]; ok && project != "" {
			group = project
		}

		containers = append(containers, messages.Container{
			ID:       r.ID,
			Name:     name,
			Status:   r.Status,
			State:    mapContainerState(r.State),
			Source:   "docker",
			Group:    group,
			Image:    r.Image,
			Created:  time.Unix(r.Created, 0),
			Restarts: 0,
		})
	}

	return containers, nil
}

// GetLogs returns a following log stream for the given container.
// The caller is responsible for closing the returned ReadCloser.
func (c *Client) GetLogs(ctx context.Context, containerID string, tail int) (io.ReadCloser, error) {
	opts := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Tail:       fmt.Sprintf("%d", tail),
	}

	reader, err := c.Raw.ContainerLogs(ctx, containerID, opts)
	if err != nil {
		return nil, fmt.Errorf("container logs %s: %w", containerID, err)
	}
	return reader, nil
}

// InspectContainer returns a pretty-printed JSON representation of the
// container's inspect data.
func (c *Client) InspectContainer(ctx context.Context, id string) (string, error) {
	inspect, err := c.Raw.ContainerInspect(ctx, id)
	if err != nil {
		return "", fmt.Errorf("inspect container %s: %w", id, err)
	}

	data, err := json.MarshalIndent(inspect, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal inspect %s: %w", id, err)
	}
	return string(data), nil
}

// mapContainerState converts a Docker state string to a ContainerState.
func mapContainerState(state string) messages.ContainerState {
	switch state {
	case "running":
		return messages.StateRunning
	case "exited", "dead":
		return messages.StateStopped
	case "restarting":
		return messages.StateRestarting
	case "created", "paused":
		return messages.StatePending
	default:
		return messages.StateUnknown
	}
}
