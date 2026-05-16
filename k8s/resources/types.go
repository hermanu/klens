package resources

import "time"

// Resource is the minimal shared interface all k8s resources satisfy.
type Resource interface {
	GetName() string
	GetNamespace() string
	GetAge() time.Duration
}

// PodItem is a minimal pod summary used by the pods list view.
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

// GetName implements Resource.
func (p PodItem) GetName() string { return p.Name }

// GetNamespace implements Resource.
func (p PodItem) GetNamespace() string { return p.Namespace }

// GetAge implements Resource.
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

// DeploymentItem is a minimal deployment summary used by the deployments list view.
type DeploymentItem struct {
	Name      string
	Namespace string
	Ready     string
	UpToDate  int32
	Available int32
	Replicas  int32 // observed replica count from Status.Replicas; use Ready string for desired/actual display
	Strategy  string
	Image     string            // first container image — used by the SPEC pane
	Selector  map[string]string // pod-template label selector; used to scope multi-pod log tails
	Age       time.Duration
}

// GetName implements Resource.
func (d DeploymentItem) GetName() string { return d.Name }

// GetNamespace implements Resource.
func (d DeploymentItem) GetNamespace() string { return d.Namespace }

// GetAge implements Resource.
func (d DeploymentItem) GetAge() time.Duration { return d.Age }

// ServiceItem is a minimal service summary used by the services list view.
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

// GetName implements Resource.
func (s ServiceItem) GetName() string { return s.Name }

// GetNamespace implements Resource.
func (s ServiceItem) GetNamespace() string { return s.Namespace }

// GetAge implements Resource.
func (s ServiceItem) GetAge() time.Duration { return s.Age }

// SecretItem is a minimal secret summary used by the secrets list view.
// Data is only populated on detail fetch via GetSecret — not in list mode.
type SecretItem struct {
	Name      string
	Namespace string
	Type      string
	Keys      int
	KeyNames  []string // sorted key names — populated by ListSecrets so the SPEC pane can preview top keys without fetching values
	Age       time.Duration
	Data      map[string][]byte // decoded bytes; only populated on detail fetch via GetSecret
}

// GetName implements Resource.
func (s SecretItem) GetName() string { return s.Name }

// GetNamespace implements Resource.
func (s SecretItem) GetNamespace() string { return s.Namespace }

// GetAge implements Resource.
func (s SecretItem) GetAge() time.Duration { return s.Age }

// ConfigMapItem is a minimal configmap summary used by the configmaps list view.
// Data is only populated on detail fetch via GetConfigMap — not in list mode.
type ConfigMapItem struct {
	Name      string
	Namespace string
	Keys      int
	KeyNames  []string // sorted key names — populated by ListConfigMaps so the SPEC pane can preview top keys without fetching values
	Age       time.Duration
	Data      map[string]string
}

// GetName implements Resource.
func (c ConfigMapItem) GetName() string { return c.Name }

// GetNamespace implements Resource.
func (c ConfigMapItem) GetNamespace() string { return c.Namespace }

// GetAge implements Resource.
func (c ConfigMapItem) GetAge() time.Duration { return c.Age }

// NamespaceItem is a minimal namespace summary used by the namespaces list view.
type NamespaceItem struct {
	Name   string
	Status string
	Labels map[string]string
	Age    time.Duration
}

// GetName implements Resource.
func (n NamespaceItem) GetName() string { return n.Name }

// GetNamespace implements Resource — namespaces are cluster-scoped, so always "".
func (n NamespaceItem) GetNamespace() string { return "" }

// GetAge implements Resource.
func (n NamespaceItem) GetAge() time.Duration { return n.Age }

// NodeItem is a minimal node summary used by the nodes list view.
type NodeItem struct {
	Name       string
	Status     string
	Roles      string
	Version    string
	Kernel     string // kernel version from NodeInfo (may be empty)
	Runtime    string // container runtime version (may be empty)
	CPU        string // allocatable[cpu] — excludes OS/kubelet reserved resources
	Memory     string // allocatable[memory] — human-readable, e.g. "14Gi"
	Pods       string // allocatable[pods]
	Taints     string // taint summary, e.g. "key:NoSchedule,key=val:NoExecute" or "<none>"
	Conditions string // active pressure conditions, e.g. "MemoryPressure,DiskPressure" or "<none>"
	Age        time.Duration
}

// GetName implements Resource.
func (n NodeItem) GetName() string { return n.Name }

// GetNamespace implements Resource — nodes are cluster-scoped, so always "".
func (n NodeItem) GetNamespace() string { return "" }

// GetAge implements Resource.
func (n NodeItem) GetAge() time.Duration { return n.Age }

// PVCItem is a minimal PersistentVolumeClaim summary used by the PVCs list view.
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

// GetName implements Resource.
func (p PVCItem) GetName() string { return p.Name }

// GetNamespace implements Resource.
func (p PVCItem) GetNamespace() string { return p.Namespace }

// GetAge implements Resource.
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
//
// When IsMarker is true, the entry isn't a real log line — it's a
// user-inserted bookmark (logs view `m` key) rendered as a horizontal
// separator so the user can spot where they paused to investigate a
// failing event in a fast-scrolling stream.
type LogLine struct {
	Pod       string
	Container string
	Time      time.Time
	Level     string // INFO/WARN/ERROR/DEBUG, parsed best-effort from line prefix
	Message   string
	IsMarker  bool
}
