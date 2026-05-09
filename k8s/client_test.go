package k8s_test

import (
	"testing"

	k8sclient "github.com/hermanu/klens/k8s"
)

func TestNewClient_InvalidPath(t *testing.T) {
	_, err := k8sclient.NewClient("/nonexistent/kubeconfig")
	if err == nil {
		t.Fatal("expected error for invalid kubeconfig path")
	}
}
