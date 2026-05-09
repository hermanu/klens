package resources_test

import (
	"context"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/hermanu/klens/k8s/resources"
)

func TestDeploymentSvc_ListDeployments(t *testing.T) {
	ready := int32(2)
	replicas := int32(3)
	fakeClient := fake.NewSimpleClientset(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "my-deploy",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Now().Add(-10 * time.Minute)),
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas:     ready,
			Replicas:          replicas,
			UpdatedReplicas:   2,
			AvailableReplicas: 2,
		},
	})

	svc := resources.NewDeploymentSvc(fakeClient)
	items, err := svc.ListDeployments(context.Background(), "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("want 1 deployment, got %d", len(items))
	}
	d := items[0]
	if d.Name != "my-deploy" {
		t.Errorf("want my-deploy, got %s", d.Name)
	}
	if d.Ready != "2/3" {
		t.Errorf("want Ready=2/3, got %s", d.Ready)
	}
	if d.UpToDate != 2 {
		t.Errorf("want UpToDate=2, got %d", d.UpToDate)
	}
	if d.Available != 2 {
		t.Errorf("want Available=2, got %d", d.Available)
	}
	if d.Age <= 0 {
		t.Errorf("want positive Age, got %v", d.Age)
	}
}

func TestDeploymentSvc_ListDeployments_Empty(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()

	svc := resources.NewDeploymentSvc(fakeClient)
	items, err := svc.ListDeployments(context.Background(), "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("want 0 deployments, got %d", len(items))
	}
}
