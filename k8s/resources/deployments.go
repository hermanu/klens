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
		// Selector — copy MatchLabels into a plain map so the views layer (which
		// does not import client-go) can pass it back to PodService.ListPodsForSelector.
		var selector map[string]string
		if d.Spec.Selector != nil && len(d.Spec.Selector.MatchLabels) > 0 {
			selector = make(map[string]string, len(d.Spec.Selector.MatchLabels))
			for k, v := range d.Spec.Selector.MatchLabels {
				selector[k] = v
			}
		}
		// Image — pick the first container's image as the SPEC summary.
		image := ""
		if cs := d.Spec.Template.Spec.Containers; len(cs) > 0 {
			image = cs[0].Image
		}
		items = append(items, DeploymentItem{
			Name:      d.Name,
			Namespace: d.Namespace,
			Ready:     fmt.Sprintf("%d/%d", d.Status.ReadyReplicas, d.Status.Replicas),
			UpToDate:  d.Status.UpdatedReplicas,
			Available: d.Status.AvailableReplicas,
			Replicas:  d.Status.Replicas,
			Strategy:  string(d.Spec.Strategy.Type),
			Image:     image,
			Selector:  selector,
			Age:       time.Since(d.CreationTimestamp.Time),
		})
	}
	return items, nil
}
