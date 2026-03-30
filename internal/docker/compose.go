package docker

import (
	"github.com/abhishek-rana/lazydev/pkg/messages"
)

// GroupByCompose groups containers by their compose project (the Group field).
// Standalone containers are grouped under "standalone".
func GroupByCompose(containers []messages.Container) map[string][]messages.Container {
	groups := make(map[string][]messages.Container)
	for _, c := range containers {
		group := c.Group
		if group == "" {
			group = "standalone"
		}
		groups[group] = append(groups[group], c)
	}
	return groups
}
