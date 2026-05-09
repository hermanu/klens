package resources

import (
	"context"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// PVCSvc implements port.PVCService using the real Kubernetes API.
type PVCSvc struct {
	client kubernetes.Interface
}

func NewPVCSvc(client kubernetes.Interface) *PVCSvc {
	return &PVCSvc{client: client}
}

func (s *PVCSvc) ListPVCs(ctx context.Context, namespace string) ([]PVCItem, error) {
	list, err := s.client.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	items := make([]PVCItem, 0, len(list.Items))
	for _, p := range list.Items {
		items = append(items, pvcToItem(p))
	}
	return items, nil
}

func pvcToItem(p corev1.PersistentVolumeClaim) PVCItem {
	capacity := ""
	if q, ok := p.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
		capacity = q.String()
	}

	accessModes := make([]string, 0, len(p.Spec.AccessModes))
	for _, mode := range p.Spec.AccessModes {
		accessModes = append(accessModes, string(mode))
	}

	storageClass := ""
	if p.Spec.StorageClassName != nil {
		storageClass = *p.Spec.StorageClassName
	}

	return PVCItem{
		Name:         p.Name,
		Namespace:    p.Namespace,
		Status:       string(p.Status.Phase),
		Volume:       p.Spec.VolumeName,
		Capacity:     capacity,
		AccessModes:  strings.Join(accessModes, ","),
		StorageClass: storageClass,
		Age:          time.Since(p.CreationTimestamp.Time),
	}
}
