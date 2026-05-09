package k8s

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

// Update messages — sent to the tea.Program when resources change in the cluster.
// Each view listens for its own message type and re-fetches via its service.
type (
	PodsUpdatedMsg        struct{}
	DeploymentsUpdatedMsg struct{}
	ServicesUpdatedMsg    struct{}
	SecretsUpdatedMsg     struct{}
	ConfigMapsUpdatedMsg  struct{}
	NamespacesUpdatedMsg  struct{}
	NodesUpdatedMsg       struct{}
	PVCsUpdatedMsg        struct{}
)

// Watcher starts Kubernetes informers and forwards resource-change events
// to a Bubble Tea program as tea.Msg values.
type Watcher struct {
	factory informers.SharedInformerFactory
	stopCh  chan struct{}
	program *tea.Program
}

// NewWatcher creates a Watcher scoped to the given namespace.
// Pass namespace="" to watch all namespaces.
func NewWatcher(client *Client, namespace string, program *tea.Program) *Watcher {
	factory := informers.NewSharedInformerFactoryWithOptions(
		client.Kube,
		30*time.Second,
		informers.WithNamespace(namespace),
	)
	return &Watcher{
		factory: factory,
		stopCh:  make(chan struct{}),
		program: program,
	}
}

// Start registers informers for all supported resource types and begins watching.
// It is non-blocking — informers run in background goroutines.
func (w *Watcher) Start() {
	w.register(w.factory.Core().V1().Pods().Informer(), PodsUpdatedMsg{})
	w.register(w.factory.Apps().V1().Deployments().Informer(), DeploymentsUpdatedMsg{})
	w.register(w.factory.Core().V1().Services().Informer(), ServicesUpdatedMsg{})
	w.register(w.factory.Core().V1().Secrets().Informer(), SecretsUpdatedMsg{})
	w.register(w.factory.Core().V1().ConfigMaps().Informer(), ConfigMapsUpdatedMsg{})
	w.register(w.factory.Core().V1().Namespaces().Informer(), NamespacesUpdatedMsg{})
	w.register(w.factory.Core().V1().Nodes().Informer(), NodesUpdatedMsg{})
	w.register(w.factory.Core().V1().PersistentVolumeClaims().Informer(), PVCsUpdatedMsg{})
	w.factory.Start(w.stopCh)
}

// Stop shuts down all informers.
func (w *Watcher) Stop() {
	close(w.stopCh)
}

func (w *Watcher) register(informer cache.SharedIndexInformer, msg tea.Msg) {
	send := func(_ interface{}) { w.program.Send(msg) }
	_, _ = informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    send,
		UpdateFunc: func(_, obj interface{}) { w.program.Send(msg) },
		DeleteFunc: send,
	})
}
