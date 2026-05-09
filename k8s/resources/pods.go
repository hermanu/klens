package resources

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// PodSvc implements port.PodService using the real Kubernetes API.
type PodSvc struct {
	client kubernetes.Interface
}

func NewPodSvc(client kubernetes.Interface) *PodSvc {
	return &PodSvc{client: client}
}

func (s *PodSvc) ListPods(ctx context.Context, namespace string) ([]PodItem, error) {
	list, err := s.client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	items := make([]PodItem, 0, len(list.Items))
	for _, p := range list.Items {
		items = append(items, podToItem(p))
	}
	return items, nil
}

func (s *PodSvc) DeletePod(ctx context.Context, namespace, name string) error {
	return s.client.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

func podToItem(p corev1.Pod) PodItem {
	ready := 0
	total := len(p.Spec.Containers)
	var restarts int32
	for _, cs := range p.Status.ContainerStatuses {
		if cs.Ready {
			ready++
		}
		restarts += cs.RestartCount
	}
	return PodItem{
		Name:      p.Name,
		Namespace: p.Namespace,
		Ready:     fmt.Sprintf("%d/%d", ready, total),
		Status:    podPhase(p),
		Restarts:  restarts,
		Age:       time.Since(p.CreationTimestamp.Time),
		Node:      p.Spec.NodeName,
		IP:        p.Status.PodIP,
	}
}

func podPhase(p corev1.Pod) string {
	for _, cs := range p.Status.ContainerStatuses {
		if cs.State.Waiting != nil && cs.State.Waiting.Reason != "" {
			return cs.State.Waiting.Reason
		}
	}
	return string(p.Status.Phase)
}
