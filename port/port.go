package port

import (
	"context"

	"github.com/manu/klens/k8s/resources"
)

// PodService is the port for pod operations.
type PodService interface {
	ListPods(ctx context.Context, namespace string) ([]resources.PodItem, error)
	DeletePod(ctx context.Context, namespace, name string) error
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
}
