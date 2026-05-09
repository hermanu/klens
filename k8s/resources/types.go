package resources

import "time"

// Resource is the minimal shared interface all k8s resources satisfy.
type Resource interface {
	GetName() string
	GetNamespace() string
	GetAge() time.Duration
}

type PodItem struct {
	Name      string
	Namespace string
	Ready     string // e.g. "2/2"
	Status    string // Running, Pending, Error, etc.
	Restarts  int32
	Age       time.Duration
	Node      string
	IP        string
}

func (p PodItem) GetName() string       { return p.Name }
func (p PodItem) GetNamespace() string  { return p.Namespace }
func (p PodItem) GetAge() time.Duration { return p.Age }

// ContainerInfo is one container's spec/status pair, populating the
// per-container section of the describe view.
type ContainerInfo struct {
	Name    string
	Image   string
	Command []string
	Args    []string
	Ports   string // pre-formatted, e.g. "8080/TCP, 9090/TCP"
	CPU     string // "requests / limits", e.g. "100m / 500m"
	Memory  string // "requests / limits", e.g. "128Mi / 1Gi"
	Ready   bool
	State   string // "Running", "Waiting (CrashLoopBackOff)", etc.
}

// PodDescription is the rich pod info populating the describe view, returned
// by PodService.DescribePod (a Get under the hood).
type PodDescription struct {
	Name           string
	Namespace      string
	Phase          string
	IP             string
	HostIP         string
	Node           string
	ServiceAccount string
	QoSClass       string
	RestartPolicy  string
	Age            time.Duration
	Labels         map[string]string
	Annotations    map[string]string
	Containers     []ContainerInfo
	InitContainers []ContainerInfo
	Conditions     []string // e.g. "Ready=True", "PodScheduled=True"
}

type DeploymentItem struct {
	Name      string
	Namespace string
	Ready     string
	UpToDate  int32
	Available int32
	Replicas  int32 // total desired replicas (Status.Replicas)
	Strategy  string
	Image     string            // first container image — used by the SPEC pane
	Selector  map[string]string // pod-template label selector; used to scope multi-pod log tails
	Age       time.Duration
}

func (d DeploymentItem) GetName() string       { return d.Name }
func (d DeploymentItem) GetNamespace() string  { return d.Namespace }
func (d DeploymentItem) GetAge() time.Duration { return d.Age }

type ServiceItem struct {
	Name       string
	Namespace  string
	Type       string
	ClusterIP  string
	ExternalIP string
	Ports      string
	Selector   map[string]string // pod selector; used to scope multi-pod log tails
	Age        time.Duration
}

func (s ServiceItem) GetName() string       { return s.Name }
func (s ServiceItem) GetNamespace() string  { return s.Namespace }
func (s ServiceItem) GetAge() time.Duration { return s.Age }

type SecretItem struct {
	Name      string
	Namespace string
	Type      string
	Keys      int
	KeyNames  []string          // sorted key names — populated by ListSecrets so the SPEC pane can preview top keys without fetching values
	Age       time.Duration
	Data      map[string][]byte // decoded bytes; only populated on detail fetch via GetSecret
}

func (s SecretItem) GetName() string       { return s.Name }
func (s SecretItem) GetNamespace() string  { return s.Namespace }
func (s SecretItem) GetAge() time.Duration { return s.Age }

type ConfigMapItem struct {
	Name      string
	Namespace string
	Keys      int
	KeyNames  []string // sorted key names — populated by ListConfigMaps so the SPEC pane can preview top keys without fetching values
	Age       time.Duration
	Data      map[string]string
}

func (c ConfigMapItem) GetName() string       { return c.Name }
func (c ConfigMapItem) GetNamespace() string  { return c.Namespace }
func (c ConfigMapItem) GetAge() time.Duration { return c.Age }

type NamespaceItem struct {
	Name   string
	Status string
	Labels map[string]string
	Age    time.Duration
}

func (n NamespaceItem) GetName() string       { return n.Name }
func (n NamespaceItem) GetNamespace() string  { return "" }
func (n NamespaceItem) GetAge() time.Duration { return n.Age }

type NodeItem struct {
	Name    string
	Status  string
	Roles   string
	Version string
	Kernel  string // kernel version from NodeInfo (may be empty)
	Runtime string // container runtime version (may be empty)
	CPU     string // capacity[cpu]
	Memory  string // capacity[memory] — human-readable, e.g. "16Gi"
	Pods    string // capacity[pods]
	Age     time.Duration
}

func (n NodeItem) GetName() string       { return n.Name }
func (n NodeItem) GetNamespace() string  { return "" }
func (n NodeItem) GetAge() time.Duration { return n.Age }

type PVCItem struct {
	Name         string
	Namespace    string
	Status       string
	Volume       string
	Capacity     string
	AccessModes  string
	StorageClass string
	Age          time.Duration
}

func (p PVCItem) GetName() string       { return p.Name }
func (p PVCItem) GetNamespace() string  { return p.Namespace }
func (p PVCItem) GetAge() time.Duration { return p.Age }

// PodMetricSample is one metrics-server reading for a pod, taken at Time.
// Klens keeps a small ring buffer of these per pod for sparkline trends.
type PodMetricSample struct {
	Namespace string
	Name      string
	CPUm      int64 // millicores summed across containers
	MemMB     int64 // megabytes summed across containers
	Time      time.Time
}

// LogLine is one streamed log entry from a pod container.
type LogLine struct {
	Pod       string
	Container string
	Time      time.Time
	Level     string // INFO/WARN/ERROR/DEBUG, parsed best-effort from line prefix
	Message   string
}
