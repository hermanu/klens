package resources_test

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/manu/klens/k8s/resources"
)

func TestServiceSvc_ListServices_ClusterIP(t *testing.T) {
	fakeClient := fake.NewSimpleClientset(&corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "my-svc",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Now().Add(-5 * time.Minute)),
		},
		Spec: corev1.ServiceSpec{
			Type:      corev1.ServiceTypeClusterIP,
			ClusterIP: "10.0.0.1",
			Ports: []corev1.ServicePort{
				{Port: 80, Protocol: corev1.ProtocolTCP, TargetPort: intstr.FromInt(8080)},
			},
		},
	})

	svc := resources.NewServiceSvc(fakeClient)
	items, err := svc.ListServices(context.Background(), "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("want 1 service, got %d", len(items))
	}
	s := items[0]
	if s.Name != "my-svc" {
		t.Errorf("want my-svc, got %s", s.Name)
	}
	if s.Type != "ClusterIP" {
		t.Errorf("want ClusterIP, got %s", s.Type)
	}
	if s.ClusterIP != "10.0.0.1" {
		t.Errorf("want 10.0.0.1, got %s", s.ClusterIP)
	}
	if s.Ports != "80/TCP" {
		t.Errorf("want 80/TCP, got %s", s.Ports)
	}
	if s.ExternalIP != "<none>" {
		t.Errorf("want <none>, got %s", s.ExternalIP)
	}
}

func TestServiceSvc_ListServices_LoadBalancer(t *testing.T) {
	fakeClient := fake.NewSimpleClientset(&corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "lb-svc",
			Namespace: "default",
		},
		Spec: corev1.ServiceSpec{
			Type:      corev1.ServiceTypeLoadBalancer,
			ClusterIP: "10.0.0.2",
			Ports: []corev1.ServicePort{
				{Port: 443, Protocol: corev1.ProtocolTCP, NodePort: 30443},
			},
		},
		Status: corev1.ServiceStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{
					{IP: "1.2.3.4"},
				},
			},
		},
	})

	svc := resources.NewServiceSvc(fakeClient)
	items, err := svc.ListServices(context.Background(), "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := items[0]
	if s.ExternalIP != "1.2.3.4" {
		t.Errorf("want ExternalIP=1.2.3.4, got %s", s.ExternalIP)
	}
	// NodePort should be included in port string
	if s.Ports != "443:30443/TCP" {
		t.Errorf("want 443:30443/TCP, got %s", s.Ports)
	}
}

func TestServiceSvc_ListServices_MultiplePorts(t *testing.T) {
	fakeClient := fake.NewSimpleClientset(&corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multi-svc",
			Namespace: "default",
		},
		Spec: corev1.ServiceSpec{
			Type:      corev1.ServiceTypeClusterIP,
			ClusterIP: "10.0.0.3",
			Ports: []corev1.ServicePort{
				{Port: 80, Protocol: corev1.ProtocolTCP},
				{Port: 443, Protocol: corev1.ProtocolTCP},
			},
		},
	})

	svc := resources.NewServiceSvc(fakeClient)
	items, err := svc.ListServices(context.Background(), "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := items[0]
	if s.Ports != "80/TCP,443/TCP" {
		t.Errorf("want 80/TCP,443/TCP, got %s", s.Ports)
	}
}
