package resources_test

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/manu/klens/k8s/resources"
)

func TestPodSvc_ListPods(t *testing.T) {
	fakeClient := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-pod",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Now().Add(-5 * time.Minute)),
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	})

	svc := resources.NewPodSvc(fakeClient)
	items, err := svc.ListPods(context.Background(), "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("want 1 pod, got %d", len(items))
	}
	if items[0].Name != "test-pod" {
		t.Errorf("want test-pod, got %s", items[0].Name)
	}
	if items[0].Status != "Running" {
		t.Errorf("want Running, got %s", items[0].Status)
	}
}

func TestPodSvc_DeletePod(t *testing.T) {
	fakeClient := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "victim", Namespace: "default"},
	})

	svc := resources.NewPodSvc(fakeClient)
	if err := svc.DeletePod(context.Background(), "default", "victim"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify deletion
	items, _ := svc.ListPods(context.Background(), "default")
	if len(items) != 0 {
		t.Errorf("want 0 pods after delete, got %d", len(items))
	}
}
