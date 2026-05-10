package port

import (
	"context"

	"github.com/hermanu/klens/k8s/resources"
)

// PodService is the port for pod operations.
type PodService interface {
	ListPods(ctx context.Context, namespace string) ([]resources.PodItem, error)
	DeletePod(ctx context.Context, namespace, name string) error
	// DescribePod fetches the rich spec+status of a single pod, used by the
	// full-screen describe view (image, env, resources, conditions, etc.).
	DescribePod(ctx context.Context, namespace, name string) (resources.PodDescription, error)
	// ListPodsForSelector returns pods in `namespace` matching `selector` as a
	// label-selector map. Used by deployment/service/etc. drill-down to scope
	// log aggregation.
	ListPodsForSelector(ctx context.Context, namespace string, selector map[string]string) ([]resources.PodItem, error)
	// ListPodsOnNode returns every pod scheduled onto the named node. Used by
	// the nodes view's `l` to fan out a multi-pod log tail across the node's
	// workload — selectors don't apply (nodes are cluster-scoped, not labeled
	// onto workloads), so this is a separate spec.nodeName field-selector path.
	ListPodsOnNode(ctx context.Context, nodeName string) ([]resources.PodItem, error)
}

// DeploymentService is the port for deployment operations.
type DeploymentService interface {
	ListDeployments(ctx context.Context, namespace string) ([]resources.DeploymentItem, error)
}

// SvcService is the port for service operations.
// (Named SvcService to avoid collision with the Go built-in concept of "service".)
type SvcService interface {
	ListServices(ctx context.Context, namespace string) ([]resources.ServiceItem, error)
}

// SecretService is the port for secret operations.
// This is the primary killer feature — full CRUD with decoded values.
type SecretService interface {
	ListSecrets(ctx context.Context, namespace string) ([]resources.SecretItem, error)
	GetSecret(ctx context.Context, namespace, name string) (resources.SecretItem, error)
	UpdateSecret(ctx context.Context, namespace, name string, data map[string][]byte) error
	CreateSecret(ctx context.Context, namespace, name string, data map[string][]byte) error
	DeleteSecret(ctx context.Context, namespace, name string) error
}

// ConfigMapService is the port for configmap operations.
type ConfigMapService interface {
	ListConfigMaps(ctx context.Context, namespace string) ([]resources.ConfigMapItem, error)
	GetConfigMap(ctx context.Context, namespace, name string) (resources.ConfigMapItem, error)
	UpdateConfigMap(ctx context.Context, namespace, name string, data map[string]string) error
}

// NamespaceService is the port for namespace operations.
type NamespaceService interface {
	ListNamespaces(ctx context.Context) ([]resources.NamespaceItem, error)
}

// NodeService is the port for node operations.
type NodeService interface {
	ListNodes(ctx context.Context) ([]resources.NodeItem, error)
}

// PVCService is the port for persistent volume claim operations.
type PVCService interface {
	ListPVCs(ctx context.Context, namespace string) ([]resources.PVCItem, error)
}

// MetricsService is the port for pod resource metrics (CPU/memory).
type MetricsService interface {
	PodMetrics(ctx context.Context, namespace string) ([]resources.PodMetricSample, error)
}

// LogService is the port for streaming pod logs. The implementation writes
// LogLine values to `out` and returns when the context is cancelled or the
// stream ends. The channel is the caller's — do not close it from the impl.
//
// `sinceSeconds` is the lookback window in seconds (e.g. 1800 for 30 min).
// Pass 0 for no since-filter; the impl will fall back to a tail-line cap so
// it doesn't replay the entire pod history on busy pods.
type LogService interface {
	StreamPodLogs(ctx context.Context, namespace, pod, container string, sinceSeconds int64, out chan<- resources.LogLine) error
	// StreamPodLogsMulti opens one log stream per pod and forwards lines into the
	// shared channel. Each LogLine.Pod is set to the source pod. Cancellation via
	// ctx closes all streams. Returns when ctx is done or the first stream errors.
	StreamPodLogsMulti(ctx context.Context, namespace string, pods []string, sinceSeconds int64, out chan<- resources.LogLine) error
}

// Services bundles all port interfaces together for injection into the app.
// The app root model holds one of these; views receive only the interface they need.
type Services struct {
	Pods        PodService
	Deployments DeploymentService
	Svcs        SvcService
	Secrets     SecretService
	ConfigMaps  ConfigMapService
	Namespaces  NamespaceService
	Nodes       NodeService
	PVCs        PVCService
	Metrics     MetricsService
	Logs        LogService
}
