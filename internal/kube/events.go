package kube

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetPodEvents returns formatted event strings for a specific pod.
func (c *Client) GetPodEvents(ctx context.Context, namespace, podName string) ([]string, error) {
	fieldSelector := fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Pod", podName)

	eventList, err := c.clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fieldSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("listing events for pod %s/%s: %w", namespace, podName, err)
	}

	events := make([]string, 0, len(eventList.Items))
	for _, event := range eventList.Items {
		age := formatAge(event.LastTimestamp.Time)
		line := fmt.Sprintf("%-8s %-10s %-15s %s", age, event.Type, event.Reason, event.Message)
		events = append(events, line)
	}

	return events, nil
}
