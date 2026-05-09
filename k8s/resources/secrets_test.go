package resources_test

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/manu/klens/k8s/resources"
)

func TestSecretSvc_ListSecrets(t *testing.T) {
	fakeClient := fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "my-secret", Namespace: "default"},
		Type:       corev1.SecretTypeOpaque,
		Data:       map[string][]byte{"KEY": []byte("value")},
	})

	svc := resources.NewSecretSvc(fakeClient)
	items, err := svc.ListSecrets(context.Background(), "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("want 1 secret, got %d", len(items))
	}
	if items[0].Name != "my-secret" {
		t.Errorf("want my-secret, got %s", items[0].Name)
	}
	if items[0].Keys != 1 {
		t.Errorf("want 1 key, got %d", items[0].Keys)
	}
	// ListSecrets must NOT populate Data (too expensive for list view)
	if items[0].Data != nil {
		t.Errorf("ListSecrets must not populate Data field")
	}
}

func TestSecretSvc_GetSecret(t *testing.T) {
	fakeClient := fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "my-secret", Namespace: "default"},
		Type:       corev1.SecretTypeOpaque,
		Data:       map[string][]byte{"API_KEY": []byte("super-secret")},
	})

	svc := resources.NewSecretSvc(fakeClient)
	item, err := svc.GetSecret(context.Background(), "default", "my-secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// GetSecret MUST populate Data with decoded values
	if string(item.Data["API_KEY"]) != "super-secret" {
		t.Errorf("want super-secret, got %s", item.Data["API_KEY"])
	}
}

func TestSecretSvc_UpdateSecret(t *testing.T) {
	fakeClient := fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "my-secret", Namespace: "default"},
		Type:       corev1.SecretTypeOpaque,
		Data:       map[string][]byte{"KEY": []byte("old")},
	})

	svc := resources.NewSecretSvc(fakeClient)
	err := svc.UpdateSecret(context.Background(), "default", "my-secret", map[string][]byte{
		"KEY": []byte("new"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updated, err := svc.GetSecret(context.Background(), "default", "my-secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(updated.Data["KEY"]) != "new" {
		t.Errorf("want new, got %s", updated.Data["KEY"])
	}
}

func TestSecretSvc_CreateAndDelete(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()

	svc := resources.NewSecretSvc(fakeClient)

	if err := svc.CreateSecret(context.Background(), "default", "new-secret", map[string][]byte{
		"TOKEN": []byte("abc123"),
	}); err != nil {
		t.Fatalf("create error: %v", err)
	}

	items, _ := svc.ListSecrets(context.Background(), "default")
	if len(items) != 1 {
		t.Fatalf("want 1 secret after create, got %d", len(items))
	}

	if err := svc.DeleteSecret(context.Background(), "default", "new-secret"); err != nil {
		t.Fatalf("delete error: %v", err)
	}

	items, _ = svc.ListSecrets(context.Background(), "default")
	if len(items) != 0 {
		t.Errorf("want 0 secrets after delete, got %d", len(items))
	}
}
