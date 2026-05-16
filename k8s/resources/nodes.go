package resources

import (
	"context"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const rolePrefix = "node-role.kubernetes.io/"

// NodeSvc implements port.NodeService using the real Kubernetes API.
type NodeSvc struct {
	client kubernetes.Interface
}

// NewNodeSvc wraps client as a NodeSvc.
func NewNodeSvc(client kubernetes.Interface) *NodeSvc {
	return &NodeSvc{client: client}
}

// ListNodes returns all nodes in the cluster.
func (s *NodeSvc) ListNodes(ctx context.Context) ([]NodeItem, error) {
	list, err := s.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	items := make([]NodeItem, 0, len(list.Items))
	for _, n := range list.Items {
		items = append(items, nodeToItem(n))
	}
	return items, nil
}

func nodeToItem(n corev1.Node) NodeItem {
	cpu, mem, pods := "", "", ""
	if q, ok := n.Status.Allocatable[corev1.ResourceCPU]; ok {
		cpu = q.String()
	}
	if q, ok := n.Status.Allocatable[corev1.ResourceMemory]; ok {
		mem = q.String()
	}
	if q, ok := n.Status.Allocatable[corev1.ResourcePods]; ok {
		pods = q.String()
	}
	return NodeItem{
		Name:    n.Name,
		Status:  nodeStatus(n),
		Roles:   nodeRoles(n),
		Version: n.Status.NodeInfo.KubeletVersion,
		Kernel:  n.Status.NodeInfo.KernelVersion,
		Runtime: n.Status.NodeInfo.ContainerRuntimeVersion,
		CPU:     cpu,
		Memory:  mem,
		Pods:    pods,
		Age:     time.Since(n.CreationTimestamp.Time),
	}
}

func nodeStatus(n corev1.Node) string {
	for _, cond := range n.Status.Conditions {
		if cond.Type == corev1.NodeReady {
			if cond.Status == corev1.ConditionTrue {
				return "Ready"
			}
			return "NotReady"
		}
	}
	return "Unknown"
}

func nodeRoles(n corev1.Node) string {
	roles := make([]string, 0)
	for label := range n.Labels {
		if strings.HasPrefix(label, rolePrefix) {
			role := strings.TrimPrefix(label, rolePrefix)
			if role != "" {
				roles = append(roles, role)
			}
		}
	}
	if len(roles) == 0 {
		return "<none>"
	}
	sort.Strings(roles)
	return strings.Join(roles, ",")
}
