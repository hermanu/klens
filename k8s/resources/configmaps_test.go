package resources_test

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/hermanu/klens/k8s/resources"
)

func TestConfigMapSvc_ListConfigMaps(t *testing.T) {
	fakeClient := fake.NewSimpleClientset(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "my-cm", Namespace: "default"},
		Data:       map[string]string{"key1": "val1", "key2": "val2"},
	})

	svc := resources.NewConfigMapSvc(fakeClient)
	items, err := svc.ListConfigMaps(context.Background(), "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("want 1 configmap, got %d", len(items))
	}
	cm := items[0]
	if cm.Name != "my-cm" {
		t.Errorf("want my-cm, got %s", cm.Name)
	}
	if cm.Keys != 2 {
		t.Errorf("want Keys=2, got %d", cm.Keys)
	}
	// ListConfigMaps must NOT populate Data
	if cm.Data != nil {
		t.Errorf("ListConfigMaps must not populate Data field")
	}
}

func TestConfigMapSvc_GetConfigMap(t *testing.T) {
	fakeClient := fake.NewSimpleClientset(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "my-cm", Namespace: "default"},
		Data:       map[string]string{"APP_PORT": "8080"},
	})

	svc := resources.NewConfigMapSvc(fakeClient)
	item, err := svc.GetConfigMap(context.Background(), "default", "my-cm")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item.Data["APP_PORT"] != "8080" {
		t.Errorf("want APP_PORT=8080, got %s", item.Data["APP_PORT"])
	}
}

func TestConfigMapSvc_UpdateConfigMap(t *testing.T) {
	fakeClient := fake.NewSimpleClientset(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "my-cm", Namespace: "default"},
		Data:       map[string]string{"KEY": "old"},
	})

	svc := resources.NewConfigMapSvc(fakeClient)
	err := svc.UpdateConfigMap(context.Background(), "default", "my-cm", map[string]string{"KEY": "new"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updated, err := svc.GetConfigMap(context.Background(), "default", "my-cm")
	if err != nil {
		t.Fatalf("unexpected error on get: %v", err)
	}
	if updated.Data["KEY"] != "new" {
		t.Errorf("want KEY=new, got %s", updated.Data["KEY"])
	}
}
