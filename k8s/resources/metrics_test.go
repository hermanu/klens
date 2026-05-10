package resources_test

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsfake "k8s.io/metrics/pkg/client/clientset/versioned/fake"

	"github.com/hermanu/klens/k8s/resources"
)

// podMetricsGVR matches the resource the fake's FakePodMetricses lists against
// ("pods" under metrics.k8s.io/v1beta1). UnsafeGuessKindToResource — used by
// NewSimpleClientset's Add path — would store the seeded object under
// "podmetrics", which the lister would then never find. Seeding via the
// tracker's Create with this explicit GVR sidesteps that mismatch.
var podMetricsGVR = schema.GroupVersionResource{
	Group:    "metrics.k8s.io",
	Version:  "v1beta1",
	Resource: "pods",
}

func newFakeWithMetrics(t *testing.T, objs ...*v1beta1.PodMetrics) *metricsfake.Clientset {
	t.Helper()
	cs := metricsfake.NewSimpleClientset()
	for _, o := range objs {
		if err := cs.Tracker().Create(podMetricsGVR, o, o.Namespace); err != nil {
			t.Fatalf("seed metrics: %v", err)
		}
	}
	return cs
}

func newPodMetrics(ns, name string, ts time.Time) *v1beta1.PodMetrics {
	return &v1beta1.PodMetrics{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Timestamp:  metav1.NewTime(ts),
		Containers: []v1beta1.ContainerMetrics{
			{
				Name: "app",
				Usage: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("120m"),
					corev1.ResourceMemory: resource.MustParse("200Mi"),
				},
			},
			{
				Name: "sidecar",
				Usage: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("30m"),
					corev1.ResourceMemory: resource.MustParse("50Mi"),
				},
			},
		},
	}
}

func TestMetricsSvc_PodMetrics_SingleNamespace(t *testing.T) {
	ts := time.Now().Truncate(time.Second)
	cs := newFakeWithMetrics(t, newPodMetrics("default", "p1", ts))

	svc := resources.NewMetricsSvc(cs)
	samples, err := svc.PodMetrics(context.Background(), "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(samples) != 1 {
		t.Fatalf("want 1 sample, got %d", len(samples))
	}
	got := samples[0]
	if got.Name != "p1" || got.Namespace != "default" {
		t.Errorf("identity mismatch: %+v", got)
	}
	if got.CPUm != 150 {
		t.Errorf("want CPUm=150, got %d", got.CPUm)
	}
	if got.MemMB != 250 {
		t.Errorf("want MemMB=250, got %d", got.MemMB)
	}
	if !got.Time.Equal(ts) {
		t.Errorf("want Time=%v, got %v", ts, got.Time)
	}
}

func TestMetricsSvc_PodMetrics_AllNamespaces(t *testing.T) {
	ts := time.Now().Truncate(time.Second)
	cs := newFakeWithMetrics(t,
		newPodMetrics("default", "p1", ts),
		newPodMetrics("kube-system", "p2", ts),
	)

	svc := resources.NewMetricsSvc(cs)
	samples, err := svc.PodMetrics(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(samples) != 2 {
		t.Fatalf("want 2 samples across all namespaces, got %d", len(samples))
	}
}
