package resources

import (
	"context"
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// SecretSvc implements port.SecretService using the real Kubernetes API.
// NOTE: client-go stores secret Data as raw []byte (already decoded from base64
// by the API server). We never touch base64 manually.
type SecretSvc struct {
	client kubernetes.Interface
}

func NewSecretSvc(client kubernetes.Interface) *SecretSvc {
	return &SecretSvc{client: client}
}

func (s *SecretSvc) ListSecrets(ctx context.Context, namespace string) ([]SecretItem, error) {
	list, err := s.client.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	items := make([]SecretItem, 0, len(list.Items))
	for _, sec := range list.Items {
		// KeyNames — sorted preview so the SPEC pane can show the top-N keys.
		// Storing names (not values) is cheap; the actual []byte values stay
		// in Data, which is intentionally omitted from list mode.
		names := make([]string, 0, len(sec.Data))
		for k := range sec.Data {
			names = append(names, k)
		}
		sort.Strings(names)
		items = append(items, SecretItem{
			Name:      sec.Name,
			Namespace: sec.Namespace,
			Type:      string(sec.Type),
			Keys:      len(sec.Data),
			KeyNames:  names,
			Age:       time.Since(sec.CreationTimestamp.Time),
			// Data intentionally omitted — too expensive to decode for every row
		})
	}
	return items, nil
}

func (s *SecretSvc) GetSecret(ctx context.Context, namespace, name string) (SecretItem, error) {
	sec, err := s.client.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return SecretItem{}, err
	}
	return SecretItem{
		Name:      sec.Name,
		Namespace: sec.Namespace,
		Type:      string(sec.Type),
		Keys:      len(sec.Data),
		Age:       time.Since(sec.CreationTimestamp.Time),
		Data:      sec.Data,
	}, nil
}

func (s *SecretSvc) UpdateSecret(ctx context.Context, namespace, name string, data map[string][]byte) error {
	sec, err := s.client.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	sec.Data = data
	_, err = s.client.CoreV1().Secrets(namespace).Update(ctx, sec, metav1.UpdateOptions{})
	return err
}

func (s *SecretSvc) CreateSecret(ctx context.Context, namespace, name string, data map[string][]byte) error {
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Type:       corev1.SecretTypeOpaque,
		Data:       data,
	}
	_, err := s.client.CoreV1().Secrets(namespace).Create(ctx, sec, metav1.CreateOptions{})
	return err
}

func (s *SecretSvc) DeleteSecret(ctx context.Context, namespace, name string) error {
	return s.client.CoreV1().Secrets(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}
