package resources

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	versioned "k8s.io/metrics/pkg/client/clientset/versioned"
)

// MetricsSvc implements port.MetricsService against k8s.io/metrics.
// Callers must tolerate Forbidden / NotFound errors when metrics-server
// is absent — those bubble up unchanged from the metrics.k8s.io API.
type MetricsSvc struct {
	cs versioned.Interface
}

// NewMetricsSvc wraps cs as a MetricsSvc.
func NewMetricsSvc(cs versioned.Interface) MetricsSvc {
	return MetricsSvc{cs: cs}
}

// PodMetrics returns CPU and memory samples for all pods in namespace.
// An empty namespace samples across all namespaces.
func (s MetricsSvc) PodMetrics(ctx context.Context, namespace string) ([]PodMetricSample, error) {
	list, err := s.cs.MetricsV1beta1().PodMetricses(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	samples := make([]PodMetricSample, 0, len(list.Items))
	for _, pm := range list.Items {
		var cpum, memBytes int64
		for _, c := range pm.Containers {
			if cpu, ok := c.Usage[corev1.ResourceCPU]; ok {
				// MilliValue gives millicores regardless of source unit (n/u/m/whole cores).
				cpum += cpu.MilliValue()
			}
			if mem, ok := c.Usage[corev1.ResourceMemory]; ok {
				memBytes += mem.Value()
			}
		}
		samples = append(samples, PodMetricSample{
			Namespace: pm.Namespace,
			Name:      pm.Name,
			CPUm:      cpum,
			MemMB:     memBytes / (1024 * 1024),
			Time:      pm.Timestamp.Time,
		})
	}
	return samples, nil
}
