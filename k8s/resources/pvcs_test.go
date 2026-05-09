package resources_test

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/manu/klens/k8s/resources"
)

func TestPVCSvc_ListPVCs(t *testing.T) {
	storageClass := "standard"
	fakeClient := fake.NewSimpleClientset(&corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-pvc",
			Namespace: "default",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			VolumeName:       "pv-123",
			StorageClassName: &storageClass,
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("10Gi"),
				},
			},
		},
		Status: corev1.PersistentVolumeClaimStatus{
			Phase: corev1.ClaimBound,
		},
	})

	svc := resources.NewPVCSvc(fakeClient)
	items, err := svc.ListPVCs(context.Background(), "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("want 1 PVC, got %d", len(items))
	}
	p := items[0]
	if p.Name != "my-pvc" {
		t.Errorf("want my-pvc, got %s", p.Name)
	}
	if p.Status != "Bound" {
		t.Errorf("want Status=Bound, got %s", p.Status)
	}
	if p.Volume != "pv-123" {
		t.Errorf("want Volume=pv-123, got %s", p.Volume)
	}
	if p.Capacity != "10Gi" {
		t.Errorf("want Capacity=10Gi, got %s", p.Capacity)
	}
	if p.AccessModes != "ReadWriteOnce" {
		t.Errorf("want AccessModes=ReadWriteOnce, got %s", p.AccessModes)
	}
	if p.StorageClass != "standard" {
		t.Errorf("want StorageClass=standard, got %s", p.StorageClass)
	}
}

func TestPVCSvc_ListPVCs_NilStorageClass(t *testing.T) {
	fakeClient := fake.NewSimpleClientset(&corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "no-sc-pvc",
			Namespace: "default",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("5Gi"),
				},
			},
		},
		Status: corev1.PersistentVolumeClaimStatus{
			Phase: corev1.ClaimPending,
		},
	})

	svc := resources.NewPVCSvc(fakeClient)
	items, err := svc.ListPVCs(context.Background(), "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p := items[0]
	if p.StorageClass != "" {
		t.Errorf("want empty StorageClass, got %s", p.StorageClass)
	}
	if p.AccessModes != "ReadOnlyMany" {
		t.Errorf("want ReadOnlyMany, got %s", p.AccessModes)
	}
}
