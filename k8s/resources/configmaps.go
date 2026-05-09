package resources

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ConfigMapSvc implements port.ConfigMapService using the real Kubernetes API.
type ConfigMapSvc struct {
	client kubernetes.Interface
}

func NewConfigMapSvc(client kubernetes.Interface) *ConfigMapSvc {
	return &ConfigMapSvc{client: client}
}

func (s *ConfigMapSvc) ListConfigMaps(ctx context.Context, namespace string) ([]ConfigMapItem, error) {
	list, err := s.client.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	items := make([]ConfigMapItem, 0, len(list.Items))
	for _, cm := range list.Items {
		items = append(items, ConfigMapItem{
			Name:      cm.Name,
			Namespace: cm.Namespace,
			Keys:      len(cm.Data),
			Age:       time.Since(cm.CreationTimestamp.Time),
			// Data intentionally omitted — too expensive for list view
		})
	}
	return items, nil
}

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

func (s *ConfigMapSvc) UpdateConfigMap(ctx context.Context, namespace, name string, data map[string]string) error {
	cm, err := s.client.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	cm.Data = data
	_, err = s.client.CoreV1().ConfigMaps(namespace).Update(ctx, cm, metav1.UpdateOptions{})
	return err
}
