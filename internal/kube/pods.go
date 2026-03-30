package kube

import (
	"context"
	"fmt"
	"io"

	"github.com/abhishek-rana/lazydk/pkg/messages"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ListPods lists pods in the given namespace (empty string for all namespaces)
// and maps them to messages.Container.
func (c *Client) ListPods(ctx context.Context, namespace string) ([]messages.Container, error) {
	podList, err := c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing pods in namespace %q: %w", namespace, err)
	}

	containers := make([]messages.Container, 0, len(podList.Items))
	for i := range podList.Items {
		pod := &podList.Items[i]

		var image string
		if len(pod.Spec.Containers) > 0 {
			image = pod.Spec.Containers[0].Image
		}

		var restarts int
		for _, cs := range pod.Status.ContainerStatuses {
			restarts += int(cs.RestartCount)
		}

		containers = append(containers, messages.Container{
			ID:       pod.Name,
			Name:     pod.Name,
			Status:   string(pod.Status.Phase),
			State:    mapPodState(pod),
			Source:   "kubernetes",
			Group:    pod.Namespace,
			Image:    image,
			Created:  pod.CreationTimestamp.Time,
			Restarts: restarts,
		})
	}

	return containers, nil
}

// GetPodLogs returns a streaming reader for pod logs.
func (c *Client) GetPodLogs(ctx context.Context, namespace, podName, containerName string, tailLines int64) (io.ReadCloser, error) {
	opts := &corev1.PodLogOptions{
		Follow:    true,
		TailLines: &tailLines,
		Container: containerName,
	}

	stream, err := c.clientset.CoreV1().Pods(namespace).GetLogs(podName, opts).Stream(ctx)
	if err != nil {
		return nil, fmt.Errorf("streaming logs for pod %s/%s: %w", namespace, podName, err)
	}

	return stream, nil
}

// mapPodState maps a Kubernetes pod phase and container statuses to a ContainerState.
func mapPodState(pod *corev1.Pod) messages.ContainerState {
	// Check container statuses for error conditions first.
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Waiting != nil {
			reason := cs.State.Waiting.Reason
			if reason == "CrashLoopBackOff" || reason == "Error" {
				return messages.StateError
			}
		}
		if cs.State.Terminated != nil && cs.State.Terminated.Reason == "Error" {
			return messages.StateError
		}
	}

	switch pod.Status.Phase {
	case corev1.PodRunning:
		return messages.StateRunning
	case corev1.PodSucceeded, corev1.PodFailed:
		return messages.StateStopped
	case corev1.PodPending:
		return messages.StatePending
	default:
		return messages.StateUnknown
	}
}
