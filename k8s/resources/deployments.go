package resources

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// DeploymentSvc implements port.DeploymentService using the real Kubernetes API.
type DeploymentSvc struct {
	client kubernetes.Interface
}

func NewDeploymentSvc(client kubernetes.Interface) *DeploymentSvc {
	return &DeploymentSvc{client: client}
}

func (s *DeploymentSvc) ListDeployments(ctx context.Context, namespace string) ([]DeploymentItem, error) {
	list, err := s.client.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	items := make([]DeploymentItem, 0, len(list.Items))
	for _, d := range list.Items {
		items = append(items, DeploymentItem{
			Name:      d.Name,
			Namespace: d.Namespace,
			Ready:     fmt.Sprintf("%d/%d", d.Status.ReadyReplicas, d.Status.Replicas),
			UpToDate:  d.Status.UpdatedReplicas,
			Available: d.Status.AvailableReplicas,
			Age:       time.Since(d.CreationTimestamp.Time),
		})
	}
	return items, nil
}
