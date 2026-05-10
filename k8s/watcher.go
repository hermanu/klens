package k8s

import (
	"context"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hermanu/klens/k8s/resources"
	"github.com/hermanu/klens/port"
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
	EventsUpdatedMsg      struct{}
	CronJobsUpdatedMsg    struct{}
)

// MetricsTickMsg fires every 5 seconds with one batched batch of pod samples.
// Views consume it to advance their per-pod sparkline ring buffers.
type MetricsTickMsg struct {
	Samples []resources.PodMetricSample
}

// LogLineMsg is one streamed log entry routed to the focused pod view.
type LogLineMsg struct {
	Line resources.LogLine
}

// PulseTickMsg fires every 400ms — drives the pulsing watch dot and tailing
// indicator. Views/models read the boolean phase to alternate cell brightness.
type PulseTickMsg struct {
	Phase bool
}

// Watcher starts Kubernetes informers and forwards resource-change events
// to a Bubble Tea program as tea.Msg values. It also runs the metrics ticker,
// the pulse ticker, and the optional pod-log stream for the focused pod.
type Watcher struct {
	factory informers.SharedInformerFactory
	stopCh  chan struct{}
	stopOnce sync.Once // guards Stop() so a defer + an explicit context-switch teardown don't double-close stopCh.
	program *tea.Program

	metrics port.MetricsService
	logs    port.LogService

	// log-stream lifecycle, guarded by mu.
	mu        sync.Mutex
	logCancel context.CancelFunc
	logCh     chan resources.LogLine
}

// NewWatcher creates a Watcher scoped to the given namespace.
// Pass namespace="" to watch all namespaces. metrics/logs may be nil — the
// watcher silently no-ops the corresponding tickers/streams when they are.
func NewWatcher(client *Client, namespace string, program *tea.Program, metrics port.MetricsService, logs port.LogService) *Watcher {
	factory := informers.NewSharedInformerFactoryWithOptions(
		client.Kube,
		30*time.Second,
		informers.WithNamespace(namespace),
	)
	return &Watcher{
		factory: factory,
		stopCh:  make(chan struct{}),
		program: program,
		metrics: metrics,
		logs:    logs,
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
	w.register(w.factory.Core().V1().Events().Informer(), EventsUpdatedMsg{})
	w.register(w.factory.Batch().V1().CronJobs().Informer(), CronJobsUpdatedMsg{})
	w.factory.Start(w.stopCh)

	go w.metricsLoop()
	go w.pulseLoop()
}

// Stop shuts down all informers and any active log stream. Safe to call
// multiple times — runtime context-switching tears down the current watcher
// explicitly, while main.go still holds a deferred Stop for clean shutdown.
func (w *Watcher) Stop() {
	w.stopOnce.Do(func() {
		w.StopPodLogTail()
		close(w.stopCh)
	})
}

// StartPodLogTails begins streaming logs for one or more pods in `ns` over a
// `sinceSeconds` lookback window and forwards each line as a LogLineMsg to the
// program. Calling it again replaces the previous stream (so a view can switch
// focus, expand to a workload's pod set, or change the time window without
// leaking goroutines).
func (w *Watcher) StartPodLogTails(ns string, pods []string, sinceSeconds int64) {
	if w.logs == nil {
		return
	}
	w.StopPodLogTail()
	if len(pods) == 0 {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	out := make(chan resources.LogLine, 64)

	w.mu.Lock()
	w.logCancel = cancel
	w.logCh = out
	w.mu.Unlock()

	go func() {
		_ = w.logs.StreamPodLogsMulti(ctx, ns, pods, sinceSeconds, out)
	}()
	go func() {
		for line := range out {
			w.program.Send(LogLineMsg{Line: line})
		}
	}()
}

// StopPodLogTail cancels the active log stream (if any) and closes its channel.
func (w *Watcher) StopPodLogTail() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.logCancel != nil {
		w.logCancel()
		w.logCancel = nil
	}
	if w.logCh != nil {
		close(w.logCh)
		w.logCh = nil
	}
}

// register attaches a debounced event handler. Each AddFunc / UpdateFunc /
// DeleteFunc resets a 500 ms timer; only when the timer fires do we Send the
// msg. On busy clusters this collapses a flood of informer events into a
// single refresh, so views aren't drowning in synchronous re-fetches.
func (w *Watcher) register(informer cache.SharedIndexInformer, msg tea.Msg) {
	const debounce = 500 * time.Millisecond
	var mu sync.Mutex
	var timer *time.Timer
	fire := func() {
		mu.Lock()
		defer mu.Unlock()
		if timer != nil {
			timer.Stop()
		}
		timer = time.AfterFunc(debounce, func() { w.program.Send(msg) })
	}
	_, _ = informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(_ interface{}) { fire() },
		UpdateFunc: func(_, _ interface{}) { fire() },
		DeleteFunc: func(_ interface{}) { fire() },
	})
}

// metricsLoop polls metrics-server every 5s. One batched MetricsTickMsg per
// tick keeps channel pressure low even on clusters with thousands of pods.
// If metrics-server is absent (Forbidden / NotFound) we silently keep ticking
// so a later install starts working without restart.
func (w *Watcher) metricsLoop() {
	if w.metrics == nil {
		return
	}
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	// Fire one immediately so views populate without a 5s lag on first paint.
	w.emitMetrics()
	for {
		select {
		case <-w.stopCh:
			return
		case <-t.C:
			w.emitMetrics()
		}
	}
}

func (w *Watcher) emitMetrics() {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	samples, err := w.metrics.PodMetrics(ctx, "")
	if err != nil {
		return
	}
	w.program.Send(MetricsTickMsg{Samples: samples})
}

// pulseLoop ticks 400ms — drives the watch / tailing dot animation.
func (w *Watcher) pulseLoop() {
	t := time.NewTicker(400 * time.Millisecond)
	defer t.Stop()
	phase := false
	for {
		select {
		case <-w.stopCh:
			return
		case <-t.C:
			phase = !phase
			w.program.Send(PulseTickMsg{Phase: phase})
		}
	}
}
