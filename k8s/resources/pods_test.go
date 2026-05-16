package resources_test

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/hermanu/klens/k8s/resources"
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

func TestPodSvc_ListPodsForSelector(t *testing.T) {
	// Two labeled pods + one unlabeled — the selector must match exactly the
	// two with app=api so workload drill-down can scope multi-pod log tails.
	fakeClient := fake.NewSimpleClientset(
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{
			Name: "api-1", Namespace: "default",
			Labels: map[string]string{"app": "api"},
		}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{
			Name: "api-2", Namespace: "default",
			Labels: map[string]string{"app": "api"},
		}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{
			Name: "worker-1", Namespace: "default",
			Labels: map[string]string{"app": "worker"},
		}},
	)

	svc := resources.NewPodSvc(fakeClient)
	items, err := svc.ListPodsForSelector(context.Background(), "default", map[string]string{"app": "api"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("want 2 pods matching app=api, got %d", len(items))
	}
	got := map[string]bool{items[0].Name: true, items[1].Name: true}
	if !got["api-1"] || !got["api-2"] {
		t.Errorf("want api-1 and api-2, got %v", got)
	}
}

func TestPodSvc_ListPods_InitCrashLoopBackOff(t *testing.T) {
	fakeClient := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "init-crash",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{{Name: "init-c"}},
			Containers:     []corev1.Container{{Name: "main"}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			InitContainerStatuses: []corev1.ContainerStatus{
				{
					Name:  "init-c",
					Ready: false,
					State: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{
							Reason: "CrashLoopBackOff",
						},
					},
				},
			},
		},
	})

	svc := resources.NewPodSvc(fakeClient)
	items, err := svc.ListPods(context.Background(), "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if items[0].Status != "Init:CrashLoopBackOff" {
		t.Errorf("want Init:CrashLoopBackOff, got %s", items[0].Status)
	}
}
