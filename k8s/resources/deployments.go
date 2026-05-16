package resources

import (
	"context"
	"fmt"
	"maps"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// DeploymentSvc implements port.DeploymentService using the real Kubernetes API.
type DeploymentSvc struct {
	client kubernetes.Interface
}

// NewDeploymentSvc wraps client as a DeploymentSvc.
func NewDeploymentSvc(client kubernetes.Interface) *DeploymentSvc {
	return &DeploymentSvc{client: client}
}

// ListDeployments returns all deployments in namespace. An empty namespace lists across all namespaces.
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
			maps.Copy(selector, d.Spec.Selector.MatchLabels)
		}
		// Image — pick the first container's image as the SPEC summary.
		image := ""
		if cs := d.Spec.Template.Spec.Containers; len(cs) > 0 {
			image = cs[0].Image
		}
		// Ready denominator uses Spec.Replicas (desired) not Status.Replicas (observed).
		// During a rollout or scale-down, Status.Replicas may be higher than Spec.Replicas
		// while old pods terminate. Spec.Replicas is the ground truth.
		desired := int32(1)
		if d.Spec.Replicas != nil {
			desired = *d.Spec.Replicas
		}
		conds := make([]string, 0, len(d.Status.Conditions))
		for _, c := range d.Status.Conditions {
			conds = append(conds, fmt.Sprintf("%s=%s", c.Type, c.Status))
		}
		items = append(items, DeploymentItem{
			Name:       d.Name,
			Namespace:  d.Namespace,
			Ready:      fmt.Sprintf("%d/%d", d.Status.ReadyReplicas, desired),
			UpToDate:   d.Status.UpdatedReplicas,
			Available:  d.Status.AvailableReplicas,
			Replicas:   d.Status.Replicas,
			Strategy:   string(d.Spec.Strategy.Type),
			Image:      image,
			Selector:   selector,
			Conditions: conds,
			Age:        time.Since(d.CreationTimestamp.Time),
		})
	}
	return items, nil
}
