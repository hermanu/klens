package resources_test

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/hermanu/klens/k8s/resources"
)

func TestNodeSvc_ListNodes_Ready(t *testing.T) {
	fakeClient := fake.NewSimpleClientset(&corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "node-1",
			CreationTimestamp: metav1.NewTime(time.Now().Add(-24 * time.Hour)),
			Labels: map[string]string{
				"node-role.kubernetes.io/control-plane": "",
				"node-role.kubernetes.io/master":        "",
			},
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			},
			NodeInfo: corev1.NodeSystemInfo{
				KubeletVersion: "v1.28.0",
			},
		},
	})

	svc := resources.NewNodeSvc(fakeClient)
	items, err := svc.ListNodes(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("want 1 node, got %d", len(items))
	}
	n := items[0]
	if n.Name != "node-1" {
		t.Errorf("want node-1, got %s", n.Name)
	}
	if n.Status != "Ready" {
		t.Errorf("want Status=Ready, got %s", n.Status)
	}
	if n.Version != "v1.28.0" {
		t.Errorf("want Version=v1.28.0, got %s", n.Version)
	}
	// Roles should contain both control-plane and master
	if n.Roles == "" {
		t.Error("want non-empty Roles")
	}
}

func TestNodeSvc_ListNodes_NotReady(t *testing.T) {
	fakeClient := fake.NewSimpleClientset(&corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node-2"},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionFalse},
			},
		},
	})

	svc := resources.NewNodeSvc(fakeClient)
	items, err := svc.ListNodes(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if items[0].Status != "NotReady" {
		t.Errorf("want NotReady, got %s", items[0].Status)
	}
}

func TestNodeSvc_ListNodes_NoRoleLabel(t *testing.T) {
	fakeClient := fake.NewSimpleClientset(&corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "worker-1"},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			},
		},
	})

	svc := resources.NewNodeSvc(fakeClient)
	items, err := svc.ListNodes(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if items[0].Roles != "<none>" {
		t.Errorf("want Roles=<none>, got %s", items[0].Roles)
	}
}

func TestNodeSvc_ListNodes_ShowsAllocatableNotCapacity(t *testing.T) {
	// Schedulers and klens should show allocatable (what pods can use), not
	// capacity (raw hardware). On EKS/GKE the gap is 10-30%.
	fakeClient := fake.NewSimpleClientset(&corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "eks-node"},
		Status: corev1.NodeStatus{
			Allocatable: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("3500m"),
				corev1.ResourceMemory: resource.MustParse("14Gi"),
				corev1.ResourcePods:   resource.MustParse("110"),
			},
			Capacity: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("4000m"),
				corev1.ResourceMemory: resource.MustParse("16Gi"),
				corev1.ResourcePods:   resource.MustParse("110"),
			},
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			},
		},
	})

	svc := resources.NewNodeSvc(fakeClient)
	items, err := svc.ListNodes(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	n := items[0]
	if n.CPU != "3500m" {
		t.Errorf("want CPU=3500m (allocatable), got %s", n.CPU)
	}
	if n.Memory != "14Gi" {
		t.Errorf("want Memory=14Gi (allocatable), got %s", n.Memory)
	}
	if n.Pods != "110" {
		t.Errorf("want Pods=110, got %s", n.Pods)
	}
}
