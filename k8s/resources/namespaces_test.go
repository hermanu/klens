package resources_test

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/manu/klens/k8s/resources"
)

func TestNamespaceSvc_ListNamespaces(t *testing.T) {
	fakeClient := fake.NewSimpleClientset(
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: "default"},
			Status:     corev1.NamespaceStatus{Phase: corev1.NamespaceActive},
		},
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: "kube-system"},
			Status:     corev1.NamespaceStatus{Phase: corev1.NamespaceActive},
		},
	)

	svc := resources.NewNamespaceSvc(fakeClient)
	items, err := svc.ListNamespaces(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("want 2 namespaces, got %d", len(items))
	}
	// Find default namespace
	var found bool
	for _, ns := range items {
		if ns.Name == "default" {
			found = true
			if ns.Status != "Active" {
				t.Errorf("want Status=Active, got %s", ns.Status)
			}
		}
	}
	if !found {
		t.Error("default namespace not found in results")
	}
}

func TestNamespaceSvc_ListNamespaces_Empty(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()

	svc := resources.NewNamespaceSvc(fakeClient)
	items, err := svc.ListNamespaces(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("want 0 namespaces, got %d", len(items))
	}
}
