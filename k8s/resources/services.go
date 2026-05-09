package resources

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ServiceSvc implements port.SvcService using the real Kubernetes API.
type ServiceSvc struct {
	client kubernetes.Interface
}

func NewServiceSvc(client kubernetes.Interface) *ServiceSvc {
	return &ServiceSvc{client: client}
}

func (s *ServiceSvc) ListServices(ctx context.Context, namespace string) ([]ServiceItem, error) {
	list, err := s.client.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	items := make([]ServiceItem, 0, len(list.Items))
	for _, svc := range list.Items {
		items = append(items, svcToItem(svc))
	}
	return items, nil
}

func svcToItem(svc corev1.Service) ServiceItem {
	externalIP := externalIPs(svc)
	ports := formatPorts(svc.Spec.Ports)
	// Selector — flat copy so callers in the views layer can pass it through
	// PodService.ListPodsForSelector without depending on client-go types.
	var selector map[string]string
	if len(svc.Spec.Selector) > 0 {
		selector = make(map[string]string, len(svc.Spec.Selector))
		for k, v := range svc.Spec.Selector {
			selector[k] = v
		}
	}
	return ServiceItem{
		Name:       svc.Name,
		Namespace:  svc.Namespace,
		Type:       string(svc.Spec.Type),
		ClusterIP:  svc.Spec.ClusterIP,
		ExternalIP: externalIP,
		Ports:      ports,
		Selector:   selector,
		Age:        time.Since(svc.CreationTimestamp.Time),
	}
}

// externalIPs returns the external IP(s) of a service, checking LoadBalancer
// ingress first, then spec.ExternalIPs. Returns "<none>" if none found.
func externalIPs(svc corev1.Service) string {
	// LoadBalancer ingress takes priority
	ingresses := svc.Status.LoadBalancer.Ingress
	if len(ingresses) > 0 {
		addrs := make([]string, 0, len(ingresses))
		for _, ing := range ingresses {
			if ing.IP != "" {
				addrs = append(addrs, ing.IP)
			} else if ing.Hostname != "" {
				addrs = append(addrs, ing.Hostname)
			}
		}
		if len(addrs) > 0 {
			return strings.Join(addrs, ",")
		}
	}
	// Fall back to spec.ExternalIPs
	if len(svc.Spec.ExternalIPs) > 0 {
		return strings.Join(svc.Spec.ExternalIPs, ",")
	}
	return "<none>"
}

// formatPorts formats service ports as "port:nodePort/protocol", joining
// multiple ports with ",". NodePort is only included when > 0.
func formatPorts(ports []corev1.ServicePort) string {
	if len(ports) == 0 {
		return ""
	}
	parts := make([]string, 0, len(ports))
	for _, p := range ports {
		if p.NodePort > 0 {
			parts = append(parts, fmt.Sprintf("%d:%d/%s", p.Port, p.NodePort, p.Protocol))
		} else {
			parts = append(parts, fmt.Sprintf("%d/%s", p.Port, p.Protocol))
		}
	}
	return strings.Join(parts, ",")
}
