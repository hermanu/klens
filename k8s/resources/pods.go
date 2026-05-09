package resources

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

// PodSvc implements port.PodService using the real Kubernetes API.
type PodSvc struct {
	client kubernetes.Interface
}

func NewPodSvc(client kubernetes.Interface) *PodSvc {
	return &PodSvc{client: client}
}

func (s *PodSvc) ListPods(ctx context.Context, namespace string) ([]PodItem, error) {
	list, err := s.client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	items := make([]PodItem, 0, len(list.Items))
	for _, p := range list.Items {
		items = append(items, podToItem(p))
	}
	return items, nil
}

func (s *PodSvc) DeletePod(ctx context.Context, namespace, name string) error {
	return s.client.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// ListPodsForSelector returns pods in `namespace` matching the supplied
// label-selector map. Used by drill-down flows (deployment / service → pods)
// so a multi-pod log tail can be scoped to a workload.
func (s *PodSvc) ListPodsForSelector(ctx context.Context, namespace string, selector map[string]string) ([]PodItem, error) {
	opts := metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(selector).String(),
	}
	list, err := s.client.CoreV1().Pods(namespace).List(ctx, opts)
	if err != nil {
		return nil, err
	}
	items := make([]PodItem, 0, len(list.Items))
	for _, p := range list.Items {
		items = append(items, podToItem(p))
	}
	return items, nil
}

// DescribePod fetches the rich spec+status of a single pod for the
// describe view: containers (image/command/resources), service account,
// QoS class, conditions, etc.
func (s *PodSvc) DescribePod(ctx context.Context, namespace, name string) (PodDescription, error) {
	p, err := s.client.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return PodDescription{}, err
	}

	// Container status by name → so we can pair Spec containers with Status.
	statusByName := make(map[string]corev1.ContainerStatus, len(p.Status.ContainerStatuses))
	for _, cs := range p.Status.ContainerStatuses {
		statusByName[cs.Name] = cs
	}

	out := PodDescription{
		Name:           p.Name,
		Namespace:      p.Namespace,
		Phase:          podPhase(*p),
		IP:             p.Status.PodIP,
		HostIP:         p.Status.HostIP,
		Node:           p.Spec.NodeName,
		ServiceAccount: p.Spec.ServiceAccountName,
		QoSClass:       string(p.Status.QOSClass),
		RestartPolicy:  string(p.Spec.RestartPolicy),
		Age:            time.Since(p.CreationTimestamp.Time),
		Labels:         p.Labels,
		Annotations:    p.Annotations,
	}
	for _, c := range p.Spec.Containers {
		out.Containers = append(out.Containers, containerInfoFrom(c, statusByName[c.Name]))
	}
	for _, c := range p.Spec.InitContainers {
		out.InitContainers = append(out.InitContainers, containerInfoFrom(c, statusByName[c.Name]))
	}
	for _, cond := range p.Status.Conditions {
		out.Conditions = append(out.Conditions, fmt.Sprintf("%s=%s", cond.Type, cond.Status))
	}
	return out, nil
}

// containerInfoFrom collapses the spec + (optional) status of one container
// into the flat ContainerInfo struct the describe view consumes.
func containerInfoFrom(c corev1.Container, st corev1.ContainerStatus) ContainerInfo {
	ports := make([]string, 0, len(c.Ports))
	for _, p := range c.Ports {
		proto := string(p.Protocol)
		if proto == "" {
			proto = "TCP"
		}
		ports = append(ports, fmt.Sprintf("%d/%s", p.ContainerPort, proto))
	}
	cpuReq, cpuLim := c.Resources.Requests.Cpu(), c.Resources.Limits.Cpu()
	memReq, memLim := c.Resources.Requests.Memory(), c.Resources.Limits.Memory()
	state := containerStateString(st)
	return ContainerInfo{
		Name:    c.Name,
		Image:   c.Image,
		Command: c.Command,
		Args:    c.Args,
		Ports:   joinNonEmpty(ports, ", "),
		CPU:     fmt.Sprintf("%s / %s", cpuReq.String(), cpuLim.String()),
		Memory:  fmt.Sprintf("%s / %s", memReq.String(), memLim.String()),
		Ready:   st.Ready,
		State:   state,
	}
}

// containerStateString summarises a container's runtime state in one line.
func containerStateString(st corev1.ContainerStatus) string {
	switch {
	case st.State.Running != nil:
		return "Running"
	case st.State.Waiting != nil:
		if st.State.Waiting.Reason != "" {
			return "Waiting (" + st.State.Waiting.Reason + ")"
		}
		return "Waiting"
	case st.State.Terminated != nil:
		if st.State.Terminated.Reason != "" {
			return "Terminated (" + st.State.Terminated.Reason + ")"
		}
		return "Terminated"
	}
	return ""
}

func joinNonEmpty(parts []string, sep string) string {
	out := ""
	for i, p := range parts {
		if p == "" {
			continue
		}
		if i > 0 && out != "" {
			out += sep
		}
		out += p
	}
	return out
}

func podToItem(p corev1.Pod) PodItem {
	ready := 0
	total := len(p.Spec.Containers)
	var restarts int32
	for _, cs := range p.Status.ContainerStatuses {
		if cs.Ready {
			ready++
		}
		restarts += cs.RestartCount
	}
	return PodItem{
		Name:      p.Name,
		Namespace: p.Namespace,
		Ready:     fmt.Sprintf("%d/%d", ready, total),
		Status:    podPhase(p),
		Restarts:  restarts,
		Age:       time.Since(p.CreationTimestamp.Time),
		Node:      p.Spec.NodeName,
		IP:        p.Status.PodIP,
	}
}

func podPhase(p corev1.Pod) string {
	for _, cs := range p.Status.ContainerStatuses {
		if cs.State.Waiting != nil && cs.State.Waiting.Reason != "" {
			return cs.State.Waiting.Reason
		}
	}
	return string(p.Status.Phase)
}
