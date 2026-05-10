package resources

import (
	"context"
	"sort"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ConfigMapSvc implements port.ConfigMapService using the real Kubernetes API.
type ConfigMapSvc struct {
	client kubernetes.Interface
}

// NewConfigMapSvc creates a ConfigMapSvc backed by the given Kubernetes client.
func NewConfigMapSvc(client kubernetes.Interface) *ConfigMapSvc {
	return &ConfigMapSvc{client: client}
}

// ListConfigMaps returns all configmaps in namespace. Data is omitted for cost; use GetConfigMap to fetch values.
func (s *ConfigMapSvc) ListConfigMaps(ctx context.Context, namespace string) ([]ConfigMapItem, error) {
	list, err := s.client.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	items := make([]ConfigMapItem, 0, len(list.Items))
	for _, cm := range list.Items {
		// KeyNames — sorted preview for the SPEC pane in list mode (Data stays
		// out of list mode for cost reasons; the names are cheap).
		names := make([]string, 0, len(cm.Data))
		for k := range cm.Data {
			names = append(names, k)
		}
		sort.Strings(names)
		items = append(items, ConfigMapItem{
			Name:      cm.Name,
			Namespace: cm.Namespace,
			Keys:      len(cm.Data),
			KeyNames:  names,
			Age:       time.Since(cm.CreationTimestamp.Time),
			// Data intentionally omitted — too expensive for list view
		})
	}
	return items, nil
}

// GetConfigMap fetches a single configmap including its full Data map.
func (s *ConfigMapSvc) GetConfigMap(ctx context.Context, namespace, name string) (ConfigMapItem, error) {
	cm, err := s.client.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return ConfigMapItem{}, err
	}
	return ConfigMapItem{
		Name:      cm.Name,
		Namespace: cm.Namespace,
		Keys:      len(cm.Data),
		Age:       time.Since(cm.CreationTimestamp.Time),
		Data:      cm.Data,
	}, nil
}

// UpdateConfigMap replaces the data of an existing configmap via Get-then-Update
// so other fields (annotations, labels) survive the write.
func (s *ConfigMapSvc) UpdateConfigMap(ctx context.Context, namespace, name string, data map[string]string) error {
	cm, err := s.client.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	cm.Data = data
	_, err = s.client.CoreV1().ConfigMaps(namespace).Update(ctx, cm, metav1.UpdateOptions{})
	return err
}
