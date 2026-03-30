package kube

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// DescribePod returns a human-readable description of a pod, similar to kubectl describe.
func (c *Client) DescribePod(ctx context.Context, namespace, name string) (string, error) {
	pod, err := c.clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("getting pod %s/%s: %w", namespace, name, err)
	}

	var b strings.Builder

	fmt.Fprintf(&b, "Name:         %s\n", pod.Name)
	fmt.Fprintf(&b, "Namespace:    %s\n", pod.Namespace)
	fmt.Fprintf(&b, "Status:       %s\n", pod.Status.Phase)
	fmt.Fprintf(&b, "Node:         %s\n", pod.Spec.NodeName)
	fmt.Fprintf(&b, "IP:           %s\n", pod.Status.PodIP)
	fmt.Fprintf(&b, "Start Time:   %s\n", pod.CreationTimestamp.Format(time.RFC3339))

	if len(pod.Labels) > 0 {
		fmt.Fprintf(&b, "Labels:\n")
		for k, v := range pod.Labels {
			fmt.Fprintf(&b, "  %s=%s\n", k, v)
		}
	}

	fmt.Fprintf(&b, "\nContainers:\n")
	for _, container := range pod.Spec.Containers {
		fmt.Fprintf(&b, "  %s:\n", container.Name)
		fmt.Fprintf(&b, "    Image:   %s\n", container.Image)

		// Find matching status.
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.Name == container.Name {
				state := "Unknown"
				if cs.State.Running != nil {
					state = fmt.Sprintf("Running (started %s)", cs.State.Running.StartedAt.Format(time.RFC3339))
				} else if cs.State.Waiting != nil {
					state = fmt.Sprintf("Waiting (%s)", cs.State.Waiting.Reason)
				} else if cs.State.Terminated != nil {
					state = fmt.Sprintf("Terminated (%s, exit code %d)", cs.State.Terminated.Reason, cs.State.Terminated.ExitCode)
				}
				fmt.Fprintf(&b, "    State:   %s\n", state)
				fmt.Fprintf(&b, "    Restarts: %d\n", cs.RestartCount)
				break
			}
		}
	}

	// Fetch events for this pod.
	fieldSelector := fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Pod", name)
	events, err := c.clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fieldSelector,
	})
	if err == nil && len(events.Items) > 0 {
		fmt.Fprintf(&b, "\nEvents:\n")
		fmt.Fprintf(&b, "  %-8s %-10s %-20s %s\n", "Type", "Reason", "Age", "Message")
		fmt.Fprintf(&b, "  %-8s %-10s %-20s %s\n", "----", "------", "---", "-------")
		for _, event := range events.Items {
			age := formatAge(event.LastTimestamp.Time)
			fmt.Fprintf(&b, "  %-8s %-10s %-20s %s\n", event.Type, event.Reason, age, event.Message)
		}
	}

	return b.String(), nil
}

// GetPodYAML returns the pod spec as indented JSON.
func (c *Client) GetPodYAML(ctx context.Context, namespace, name string) (string, error) {
	pod, err := c.clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("getting pod %s/%s: %w", namespace, name, err)
	}

	data, err := json.MarshalIndent(pod, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshaling pod %s/%s to JSON: %w", namespace, name, err)
	}

	return string(data), nil
}

// DeletePod deletes a pod by name in the given namespace.
func (c *Client) DeletePod(ctx context.Context, namespace, name string) error {
	err := c.clientset.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("deleting pod %s/%s: %w", namespace, name, err)
	}
	return nil
}

// RestartDeployment performs a rolling restart by patching the deployment's pod template
// annotation, equivalent to kubectl rollout restart.
func (c *Client) RestartDeployment(ctx context.Context, namespace, name string) error {
	patchData := fmt.Sprintf(
		`{"spec":{"template":{"metadata":{"annotations":{"kubectl.kubernetes.io/restartedAt":"%s"}}}}}`,
		time.Now().Format(time.RFC3339),
	)

	_, err := c.clientset.AppsV1().Deployments(namespace).Patch(
		ctx, name, types.StrategicMergePatchType, []byte(patchData), metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("restarting deployment %s/%s: %w", namespace, name, err)
	}
	return nil
}

// ScaleDeployment scales a deployment to the given replica count.
func (c *Client) ScaleDeployment(ctx context.Context, namespace, name string, replicas int32) error {
	scale, err := c.clientset.AppsV1().Deployments(namespace).GetScale(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting scale for %s/%s: %w", namespace, name, err)
	}

	scale.Spec.Replicas = replicas
	_, err = c.clientset.AppsV1().Deployments(namespace).UpdateScale(ctx, name, scale, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("scaling deployment %s/%s to %d: %w", namespace, name, replicas, err)
	}
	return nil
}

// formatAge returns a human-readable duration string (e.g. "2m", "3h", "1d").
func formatAge(t time.Time) string {
	if t.IsZero() {
		return "<unknown>"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}
