package resources

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// NamespaceSvc implements port.NamespaceService using the real Kubernetes API.
type NamespaceSvc struct {
	client kubernetes.Interface
}

func NewNamespaceSvc(client kubernetes.Interface) *NamespaceSvc {
	return &NamespaceSvc{client: client}
}

func (s *NamespaceSvc) ListNamespaces(ctx context.Context) ([]NamespaceItem, error) {
	list, err := s.client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	items := make([]NamespaceItem, 0, len(list.Items))
	for _, n := range list.Items {
		items = append(items, NamespaceItem{
			Name:   n.Name,
			Status: string(n.Status.Phase),
			Age:    time.Since(n.CreationTimestamp.Time),
		})
	}
	return items, nil
}
