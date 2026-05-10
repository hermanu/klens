package app

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hermanu/klens/config"
	k8sclient "github.com/hermanu/klens/k8s"
	"github.com/hermanu/klens/k8s/resources"
	"github.com/hermanu/klens/port"
	"github.com/hermanu/klens/ui/components"
	"github.com/hermanu/klens/ui/layout"
	"github.com/hermanu/klens/ui/theme"
	"github.com/hermanu/klens/ui/views"
	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"
)

// flashClearMsg is fired after flashTTL by the tea.Tick scheduled when an
// inline ex-mode command fails. Update clears m.flashErr on receipt so the
// red banner doesn't linger past the user's next keystroke.
type flashClearMsg struct{}

const flashTTL = 1500 * time.Millisecond

type viewKind int

const (
	viewPods viewKind = iota
	viewDeployments
	viewServices
	viewSecrets
	viewConfigMaps
	viewNamespaces
	viewNodes
	viewPVCs
	viewLogs            // dedicated full-screen log tail; entered via `l` on a pod
	viewDescribe        // dedicated full-screen pod describe; entered via Enter on a pod
	viewGenericDescribe // dedicated full-screen KV describe for non-pod resources (PVCs, etc.)
)

// Geometry — the modern shell's pane sizes. No left rail: the table fills
// every column the terminal gives us, minus the right details pane (which
// itself drops below `minDetailsAt`). We don't cap or center the content —
// extra horizontal real estate goes straight to the table.
//
// frameH/frameW account for the rounded focus frame drawn around the content
// area (chips + table + details). The frame eats 1 cell on every edge, so
// the inner content has 2 fewer rows / 2 fewer columns to work with.
const (
	detailsWidth = 44
	topBarHeight = 2 // 1 content row + 1 divider
	cmdBarHeight = 1
	chipsHeight  = 1
	minDetailsAt = 120
	frameH       = 2 // top + bottom border rows
	frameW       = 2 // left + right border columns
)

// Model is the root Bubble Tea model. It owns all views, the input, the
// palette, and the cluster info shown in the top bar.
type Model struct {
	client       *k8sclient.Client
	services     port.Services
	namespace    string // active namespace filter ("" = all)
	cluster      ClusterInfo

	current         viewKind
	pods            views.PodsView
	deployments     views.DeploymentsView
	services_       views.ServicesView
	secrets         views.SecretsView
	configmaps      views.ConfigMapsView
	namespaces      views.NamespacesView
	nodes           views.NodesView
	pvcs            views.PVCsView
	logs            views.LogsView
	describe        views.DescribeView
	genericDescribe views.GenericDescribeView

	// showHelp toggles the `?` help overlay. While true, View() returns the
	// overlay placed full-screen and Update only listens for `?`/esc to dismiss.
	showHelp bool

	palette     components.Palette
	showPalette bool

	// Inline ex-mode (`:`) — separate from the modal palette so the two UIs
	// can coexist. The palette is full-overlay browse-by-list; commandMode is
	// a one-line vim-style prompt with type-ahead suggestions docked above
	// the bottom command bar.
	commandMode  bool
	commandInput textinput.Model
	commandSel   int    // highlighted suggestion index in commandMode
	flashErr     string // transient error banner (e.g. "no command 'foo'") cleared by flashClearMsg

	filterInput   textinput.Model
	filterFocused bool // when true, keystrokes go to filterInput; otherwise to view

	// Context picker state — populated when the kubeconfig has no current-
	// context but multiple contexts are available. The picker takes over the
	// entire frame until the user picks one or quits with esc.
	availableContexts     []string
	showContextPicker     bool
	contextPickerSelected int
	contextPickerErr      string

	// logTailRef is a pointer to a shared function slot. Both the local Model
	// in main.go and the copy held inside tea.NewProgram dereference the same
	// pointer, so SetLogTailStarter (called after tea.NewProgram, when the
	// watcher exists) propagates to the live model running in the program.
	logTailRef *func(ns string, pods []string, sinceSeconds int64)

	// restartWatcherRef is the same pointer trick for the watcher-restart
	// callback used by runtime context switching. main.go provides a closure
	// that stops the current watcher and starts a fresh one bound to the new
	// client + services; the model invokes it through this slot.
	restartWatcherRef *func(client *k8sclient.Client, ns string, metrics port.MetricsService, logs port.LogService)

	// history is the navigation stack. Drill-downs (Enter on a deployment /
	// service / namespace; `l` on a pod) push the current view; Esc on a
	// drilled view pops back. Palette / ex-mode jumps clear the stack so
	// we don't ricochet between unrelated jumps.
	history []viewKind

	width  int
	height int
}

// SetLogTailStarter wires the watcher's StartPodLogTails to the model so that
// `l` on a pod (or a range-shortcut on the logs view, or a workload-scoped
// drill-down) can begin a live log stream over one or more pods.
func (m Model) SetLogTailStarter(f func(ns string, pods []string, sinceSeconds int64)) {
	if m.logTailRef != nil {
		*m.logTailRef = f
	}
}

// SetWatcherRestarter wires a callback that stops the current watcher and
// starts a new one bound to the given client/namespace/services. main.go owns
// the watcher lifecycle, so the callback closes over its local pointer to the
// active *Watcher; the model invokes it from the runtime context-picker
// confirm path.
func (m Model) SetWatcherRestarter(f func(client *k8sclient.Client, ns string, metrics port.MetricsService, logs port.LogService)) {
	if m.restartWatcherRef != nil {
		*m.restartWatcherRef = f
	}
}

// persistState writes the current namespace and active resource view to
// ~/.klens/config.yaml so the next launch reopens to the same scope. Called
// after every meaningful state change (palette jump, drill-down, ns switch).
// Best-effort — write errors are swallowed.
func (m Model) persistState() {
	cfg, err := config.Load("")
	if err != nil {
		return
	}
	cfg.Namespace = m.namespace
	if name := viewKindName(m.current); name != "" {
		cfg.LastView = name
	}
	_ = config.Save(cfg, "")
}

// viewKindName returns the palette/config name for a viewKind. Returns "" for
// transient sub-views (logs, describe) that we don't persist as last-active.
func viewKindName(v viewKind) string {
	switch v {
	case viewPods:
		return "pods"
	case viewDeployments:
		return "deployments"
	case viewServices:
		return "services"
	case viewSecrets:
		return "secrets"
	case viewConfigMaps:
		return "configmaps"
	case viewNamespaces:
		return "namespaces"
	case viewNodes:
		return "nodes"
	case viewPVCs:
		return "pvcs"
	}
	return ""
}

// ClusterInfo is the top-bar context block. Populated at New() time from the
// kubeconfig and discovery — values default to placeholders if unavailable so
// the bar still renders cleanly without a cluster.
type ClusterInfo struct {
	Context    string
	Cluster    string
	User       string
	K8sVersion string
	Region     string
	KlensVer   string
}

// New builds the root model. Non-empty overrides take precedence over the
// config file: kubeconfigOverride replaces cfg.Kubeconfig, namespaceOverride
// replaces cfg.Namespace. Pass empty strings to fall back to the config.
// Tolerates a missing cluster: returns a Model with `client == nil` and the
// warning is logged to stderr — the runtime context picker takes over.
func New(kubeconfigOverride, namespaceOverride string) (Model, error) {
	cfg, err := config.Load("")
	if err != nil {
		return Model{}, fmt.Errorf("load config: %w", err)
	}
	// CLI-flag overrides take precedence over the persisted config.
	// Empty strings fall through, leaving cfg.Namespace as either the
	// persisted value or "" (= list across all namespaces, matching
	// the design's "ns:all" default).
	if kubeconfigOverride != "" {
		cfg.Kubeconfig = kubeconfigOverride
	}
	if namespaceOverride != "" {
		cfg.Namespace = namespaceOverride
	}
	ns := cfg.Namespace

	client, clientErr := k8sclient.NewClient(cfg.Kubeconfig)
	if clientErr != nil {
		fmt.Fprintf(os.Stderr, "warn: no k8s cluster: %v\n", clientErr)
	}

	ti := textinput.New()
	// Placeholder is just a verb — we do plain case-insensitive substring
	// matching against every stringy field of the row (name, namespace,
	// status, etc.). The previous "ns:platform status:Running" placeholder
	// implied a query DSL that doesn't exist.
	ti.Placeholder = "type to filter…"
	ti.Prompt = ""
	ti.CharLimit = 96

	cmdTi := textinput.New()
	cmdTi.Placeholder = "command (e.g. dp, ctx, q)"
	cmdTi.Prompt = ""
	cmdTi.CharLimit = 64

	var logTail func(ns string, pods []string, sinceSeconds int64)
	var restart func(client *k8sclient.Client, ns string, metrics port.MetricsService, logs port.LogService)
	m := Model{
		client:            client,
		namespace:         ns,
		palette:           components.NewPalette(nil),
		filterInput:       ti,
		commandInput:      cmdTi,
		logTailRef:        &logTail,
		restartWatcherRef: &restart,
		cluster: ClusterInfo{
			KlensVer:   "0.3.0",
			K8sVersion: "—",
			Region:     "—",
			User:       "—",
		},
	}

	if client == nil {
		// No live cluster — surface a context picker so the user can pick
		// one of the available kubeconfig contexts instead of being stuck on
		// a blank "no cluster" screen. Best-effort: if the kubeconfig itself
		// failed to load, the contexts list comes back empty and the picker
		// renders a "no contexts found" hint.
		if contexts, _, err := k8sclient.Contexts(); err == nil && len(contexts) > 0 {
			m.availableContexts = contexts
			m.showContextPicker = true
		}
		return m, nil
	}

	m = m.attachClient(client, cfg)
	return m, nil
}

// attachClient builds every per-cluster view + service field from a live
// client. Pulled out of New() so the context picker can re-run it after a
// startup-time switch without duplicating wiring.
func (m Model) attachClient(client *k8sclient.Client, cfg config.Config) Model {
	m.client = client
	ns := m.namespace

	m.services = buildServices(client)
	m.pods = views.NewPodsView(m.services.Pods, ns)
	m.deployments = views.NewDeploymentsView(m.services.Deployments, m.services.Pods, ns)
	m.services_ = views.NewServicesView(m.services.Svcs, m.services.Pods, ns)
	m.secrets = views.NewSecretsView(m.services.Secrets, ns)
	m.configmaps = views.NewConfigMapsView(m.services.ConfigMaps, ns)
	m.namespaces = views.NewNamespacesView(m.services.Namespaces)
	m.nodes = views.NewNodesView(m.services.Nodes, m.services.Pods)
	m.pvcs = views.NewPVCsView(m.services.PVCs, ns)
	m.logs = views.NewLogsView()
	m.describe = views.NewDescribeView(m.services.Pods)
	m.genericDescribe = views.NewGenericDescribeView()

	// Best-effort populate top-bar context from the cluster.
	if v, err := client.Kube.Discovery().ServerVersion(); err == nil {
		m.cluster.K8sVersion = v.GitVersion
	}
	// Cluster / user / context are extracted from the loaded kubeconfig
	// when possible; on failure leave the placeholders.
	if info, err := k8sclient.CurrentContextInfo(); err == nil {
		m.cluster.Context = info.Context
		m.cluster.Cluster = info.Cluster
		m.cluster.User = info.User
	}

	// Restore the last-opened view from config so the user re-enters
	// klens on the same screen they left.
	if v := paletteNameToView(cfg.LastView); cfg.LastView != "" {
		m.current = v
	}

	return m
}

func buildServices(client *k8sclient.Client) port.Services {
	out := port.Services{
		Pods:        resources.NewPodSvc(client.Kube),
		Deployments: resources.NewDeploymentSvc(client.Kube),
		Svcs:        resources.NewServiceSvc(client.Kube),
		Secrets:     resources.NewSecretSvc(client.Kube),
		ConfigMaps:  resources.NewConfigMapSvc(client.Kube),
		Namespaces:  resources.NewNamespaceSvc(client.Kube),
		Nodes:       resources.NewNodeSvc(client.Kube),
		PVCs:        resources.NewPVCSvc(client.Kube),
		Logs:        resources.NewLogSvc(client.Kube),
	}
	// metrics-server is optional — we tolerate clusters without it.
	if mcs, err := metricsclient.NewForConfig(client.Config); err == nil {
		out.Metrics = resources.NewMetricsSvc(mcs)
	}
	return out
}

// Client returns the underlying k8s client (may be nil if no cluster).
func (m Model) Client() *k8sclient.Client { return m.client }

// Namespace returns the active namespace filter.
func (m Model) Namespace() string { return m.namespace }

// Metrics returns the optional metrics service (may be nil).
func (m Model) Metrics() port.MetricsService { return m.services.Metrics }

// Logs returns the optional log service (may be nil if no cluster).
func (m Model) Logs() port.LogService { return m.services.Logs }

// PaletteVisible reports whether the modal command palette is open.
func (m Model) PaletteVisible() bool { return m.showPalette }

// CommandModeActive reports whether the inline `:` ex-mode prompt is active.
// Exposed so tests can verify the ctrl+p / `:` split without poking internals.
func (m Model) CommandModeActive() bool { return m.commandMode }

// FlashError returns the current transient error banner text (empty when
// none). Set when an inline-ex command misses; cleared by flashClearMsg.
func (m Model) FlashError() string { return m.flashErr }

// PodsFilter returns the per-view filter on the pods list. Used by tests to
// verify filter persistence across drill-downs.
func (m Model) PodsFilter() string { return m.pods.Filter() }

func (m Model) Init() tea.Cmd {
	// Fire one UpdatedMsg per resource type so every view fetches once and the
	// nav-rail counts populate immediately (not just for the focused view).
	return tea.Batch(
		func() tea.Msg { return k8sclient.PodsUpdatedMsg{} },
		func() tea.Msg { return k8sclient.DeploymentsUpdatedMsg{} },
		func() tea.Msg { return k8sclient.ServicesUpdatedMsg{} },
		func() tea.Msg { return k8sclient.SecretsUpdatedMsg{} },
		func() tea.Msg { return k8sclient.ConfigMapsUpdatedMsg{} },
		func() tea.Msg { return k8sclient.NamespacesUpdatedMsg{} },
		func() tea.Msg { return k8sclient.NodesUpdatedMsg{} },
		func() tea.Msg { return k8sclient.PVCsUpdatedMsg{} },
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Emergency exit — ctrl+c must always quit, regardless of palette state,
	// filter focus, or any view's keymap. Bind this before any other key
	// handler so it can never be swallowed.
	if k, ok := msg.(tea.KeyMsg); ok && k.String() == "ctrl+c" {
		return m, tea.Quit
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case flashClearMsg:
		m.flashErr = ""
		return m, nil

	case tea.KeyMsg:
		// Context picker takes over the whole UI at startup if no current
		// context was loaded — handle navigation/selection here before any
		// view-level routing so keystrokes can't slip through to a view that
		// doesn't exist yet.
		if m.showContextPicker {
			return m.updateContextPicker(msg)
		}
		// View-claimed key capture (e.g. secrets/configmaps in edit mode):
		// route straight to the active view, skipping every global shortcut
		// so the inner editor's `:`, `?`, `/` etc. don't fight the app's
		// command palette / help overlay / filter focus. ctrl+c is already
		// handled above as the emergency exit.
		if cap, ok := m.currentView().(views.Capturing); ok && cap.CapturesKeys() {
			return m.routeToCurrentView(msg)
		}
		// `?` toggles the help overlay from any state where the filter is not
		// focused and no other modal/inline mode is up. Defining this here
		// (above updateGlobal) means the toggle works regardless of the
		// current view's keymap and never collides with view-local keys.
		if !m.filterFocused && !m.commandMode && !m.showPalette && msg.String() == "?" {
			m.showHelp = !m.showHelp
			return m, nil
		}
		// While the help overlay is up, esc closes it; everything else is a
		// no-op so users can't navigate the underlying view by mistake.
		if m.showHelp {
			if msg.String() == "esc" {
				m.showHelp = false
			}
			return m, nil
		}
		if m.showPalette {
			return m.updatePalette(msg)
		}
		if m.commandMode {
			return m.updateCommandMode(msg)
		}
		return m.updateGlobal(msg)
	}

	// A view asked us to start streaming logs from the named pod over the
	// requested lookback window. The root holds the watcher reference so
	// views never need to import client-go.
	if req, ok := msg.(views.LogTailRequestMsg); ok {
		if m.logTailRef != nil && *m.logTailRef != nil {
			(*m.logTailRef)(req.Namespace, req.Pods, req.SinceSeconds)
		}
		return m, nil
	}

	// A list view (pods, deployments, services, nodes) asked us to focus the
	// dedicated full-screen logs view. Pods is a slice so single-pod and
	// multi-pod tails go through the same handler; Title is the chip caption.
	if sw, ok := msg.(views.SwitchToLogsMsg); ok {
		m.history = append(m.history, m.current)
		m.logs = m.logs.WithFocus(sw.Namespace, sw.Pods, sw.Title)
		m.current = viewLogs
		m = m.syncFilterInput()
		return m, nil
	}

	// A non-pod list view (today: PVCs) asked us to focus the GenericDescribe
	// shell with a snapshot of the row's KV preview as the body.
	if sw, ok := msg.(views.SwitchToGenericDescribeMsg); ok {
		m.history = append(m.history, m.current)
		m.genericDescribe = m.genericDescribe.WithFocus(sw.Title, sw.KVs)
		m.current = viewGenericDescribe
		m = m.syncFilterInput()
		return m, nil
	}

	// PodsView asked us to focus the describe view (Enter on a pod row).
	// WithFocus returns the cmd that fetches the pod's full spec async.
	if sw, ok := msg.(views.SwitchToDescribeMsg); ok {
		m.history = append(m.history, m.current)
		var fetch tea.Cmd
		m.describe, fetch = m.describe.WithFocus(sw.Namespace, sw.Pod)
		m.current = viewDescribe
		m = m.syncFilterInput()
		return m, fetch
	}

	// LogsView asked us to return to whatever it was opened from (Esc).
	if _, ok := msg.(views.BackToPodsMsg); ok {
		if len(m.history) > 0 {
			prev := m.history[len(m.history)-1]
			m.history = m.history[:len(m.history)-1]
			m.current = prev
		} else {
			m.current = viewPods
		}
		m = m.syncFilterInput()
		return m, nil
	}

	// k9s-style drill-down: a non-pod list view (deployments / services /
	// nodes / etc.) asked us to switch to pods narrowed by some substring.
	// We apply that as a programmatic *scope* on PodsView rather than
	// hijacking the filter input — the user's typed query stays empty so
	// the bottom command bar reads as clean. The scope chip in the strip
	// shows the user why they're seeing a subset. Esc-back pops the
	// originating view (which preserves its own filter independently).
	if d, ok := msg.(views.DrillToPodsMsg); ok {
		m.history = append(m.history, m.current)
		m.current = viewPods
		label := d.Filter
		if d.Label != "" {
			label = d.Label
		}
		m.pods = m.pods.WithScope(d.Filter, label)
		// Clear the bottom-bar filter — we want this view to feel fresh,
		// matching the user's mental model ("I drilled in; the input is
		// my own filter, not the deployment name").
		m.filterInput.SetValue("")
		// Push an empty FilterMsg so PodsView clears any stale filter
		// from a previous /-typed query (the scope is independent).
		next, c := m.routeToCurrentView(views.FilterMsg{Query: ""})
		nm, _ := next.(Model)
		nm.filterInput = m.filterInput
		go nm.persistState()
		return nm, c
	}

	// k9s-style drill-down: pressing Enter on a Namespaces row fires this msg
	// so the model can switch the active namespace + focus pods. We also
	// persist the namespace to ~/.klens/config.yaml in a goroutine so the
	// next launch re-opens to it (best-effort — we ignore write errors).
	if sel, ok := msg.(views.NamespaceSelectedMsg); ok {
		m.history = append(m.history, m.current)
		m.namespace = sel.Name
		m.current = viewPods
		// Namespace switch is a fresh scope — drop any leftover drill
		// scope from a previous workload narrowing.
		m.pods = m.pods.WithScope("", "")
		go m.persistState()
		nm, broadcastCmd := m.broadcastToViews(views.NamespaceChangedMsg{Namespace: sel.Name})
		nm2, _ := nm.(Model)
		return nm2, tea.Batch(
			broadcastCmd,
			// Refetch every resource type — even cluster-scoped ones
			// (Namespaces, Nodes) — so the nav rail counts all stay current
			// in lock-step with the new scope.
			func() tea.Msg { return k8sclient.PodsUpdatedMsg{} },
			func() tea.Msg { return k8sclient.DeploymentsUpdatedMsg{} },
			func() tea.Msg { return k8sclient.ServicesUpdatedMsg{} },
			func() tea.Msg { return k8sclient.SecretsUpdatedMsg{} },
			func() tea.Msg { return k8sclient.ConfigMapsUpdatedMsg{} },
			func() tea.Msg { return k8sclient.NamespacesUpdatedMsg{} },
			func() tea.Msg { return k8sclient.NodesUpdatedMsg{} },
			func() tea.Msg { return k8sclient.PVCsUpdatedMsg{} },
		)
	}

	// Metrics ticks only matter to PodsView — route directly to skip the
	// broadcast cost on every 5s tick.
	if _, ok := msg.(k8sclient.MetricsTickMsg); ok {
		var c tea.Cmd
		m.pods, c = m.pods.Update(msg)
		return m, c
	}
	// Log lines need to reach BOTH the PodsView (for the details-pane log
	// tail when no full-screen logs view is open) AND the LogsView (for the
	// full-screen view). The simplest path is the broadcast below, so let
	// it fall through to broadcastToViews.

	// Everything else (watcher UpdatedMsgs, namespace changes, async
	// *ListedMsgs returned by view goroutines, save/detail msgs) broadcasts
	// to every view. Each view's Update is a fast no-op for messages it
	// doesn't recognise, and this is what keeps nav-rail counts current
	// regardless of which view is focused.
	return m.broadcastToViews(msg)
}

// broadcastToViews routes msg through every view's Update. Used for watcher /
// metrics ticks where each view filters by its own message type.
func (m Model) broadcastToViews(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var c tea.Cmd
	m.pods, c = m.pods.Update(msg)
	cmds = append(cmds, c)
	m.deployments, c = m.deployments.Update(msg)
	cmds = append(cmds, c)
	m.services_, c = m.services_.Update(msg)
	cmds = append(cmds, c)
	m.secrets, c = m.secrets.Update(msg)
	cmds = append(cmds, c)
	m.configmaps, c = m.configmaps.Update(msg)
	cmds = append(cmds, c)
	m.namespaces, c = m.namespaces.Update(msg)
	cmds = append(cmds, c)
	m.nodes, c = m.nodes.Update(msg)
	cmds = append(cmds, c)
	m.pvcs, c = m.pvcs.Update(msg)
	cmds = append(cmds, c)
	m.logs, c = m.logs.Update(msg)
	cmds = append(cmds, c)
	m.describe, c = m.describe.Update(msg)
	cmds = append(cmds, c)
	m.genericDescribe, c = m.genericDescribe.Update(msg)
	cmds = append(cmds, c)
	return m, tea.Batch(cmds...)
}

func (m Model) updateGlobal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Resource switching is palette/ex-mode only — `:po`, `:dp`, `:svc`,
	// `:ctx`, `:q` (or `ctrl+p` for the modal). The numeric mnemonics 1-8
	// were removed because they collided with view-local digit keys
	// (logs view's 0-5 lookback presets) and the filter input, with no
	// visible affordance to signal the binding to users.

	// `/` enters filter-focus mode (vim-style). The current view's filter is
	// preserved so the user can edit it; clearing is one `esc` away.
	if msg.String() == "/" {
		m.filterFocused = true
		m = m.syncFilterInput()
		m.filterInput.Focus()
		return m, nil
	}

	// `enter` while typing a filter commits it and exits input mode — the
	// filter stays applied, but j/k now navigate the filtered table again.
	if msg.String() == "enter" && m.filterFocused {
		m.filterFocused = false
		m.filterInput.Blur()
		return m, nil
	}

	// `esc` priorities (in order):
	//   1. exit filter-focus + clear the filter on the current view
	//   2. pop the navigation history (drill-back) — preserves per-view filters
	//   3. otherwise let the current view handle it
	if msg.String() == "esc" {
		if m.filterFocused {
			m.filterFocused = false
			m.filterInput.SetValue("")
			m.filterInput.Blur()
			next, cmd := m.routeToCurrentView(views.FilterMsg{Query: ""})
			nm, _ := next.(Model)
			nm.filterInput = m.filterInput
			return nm, cmd
		}
		if len(m.history) > 0 {
			prev := m.history[len(m.history)-1]
			m.history = m.history[:len(m.history)-1]
			m.current = prev
			// Per-view filter persistence: each view stores its own filter
			// in its receiver, so popping back simply re-mirrors that view's
			// filter into the bottom command bar. We deliberately do NOT
			// broadcast FilterMsg{""} — that was the regression that wiped
			// the user's filter on every drill-back from logs/describe.
			m = m.syncFilterInput()
			go m.persistState()
			return m, nil
		}
	}

	// Filter-focus mode: route every keystroke to the textinput. Enter and
	// esc above are the two ways out (commit / clear).
	if m.filterFocused {
		prev := m.filterInput.Value()
		var inputCmd tea.Cmd
		m.filterInput, inputCmd = m.filterInput.Update(msg)
		if m.filterInput.Value() != prev {
			next, cmd := m.routeToCurrentView(views.FilterMsg{Query: m.filterInput.Value()})
			nm, _ := next.(Model)
			nm.filterInput = m.filterInput
			nm.filterFocused = true
			return nm, tea.Batch(inputCmd, cmd)
		}
		return m, inputCmd
	}

	// Default mode global keys:
	//   ctrl+p → modal palette (browse-by-list overlay)
	//   :      → inline ex-mode (vim-style prompt with type-ahead)
	//   q      → quit
	switch msg.String() {
	case "ctrl+p":
		m.showPalette = true
		m.palette = components.NewPalette(nil)
		return m, nil
	case ":":
		m.commandMode = true
		m.commandInput.SetValue("")
		m.commandInput.Focus()
		m.commandSel = 0
		return m, nil
	case "q":
		return m, tea.Quit
	}
	return m.routeToCurrentView(msg)
}

func (m Model) updatePalette(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.showPalette = false
		return m, nil
	case "enter":
		cmd := m.palette.Selected()
		m.showPalette = false
		if cmd != nil {
			return m.runCommand(cmd.Name)
		}
		return m, nil
	}
	var teaCmd tea.Cmd
	m.palette, teaCmd = m.palette.Update(msg)
	return m, teaCmd
}

// runCommand executes one of the shared command-list entries, regardless of
// whether it was selected from the modal palette (ctrl+p) or the inline
// ex-mode prompt (`:`). Centralised so both surfaces dispatch identically.
//
// Returns the updated model + tea.Cmd for the side-effect (reload, quit,
// open context picker). Unknown names map to a no-op so callers don't have
// to validate before calling.
func (m Model) runCommand(name string) (tea.Model, tea.Cmd) {
	switch name {
	case "quit":
		return m, tea.Quit
	case "context":
		return m.openContextPicker(), nil
	case "pods", "deployments", "services", "secrets", "configmaps", "namespaces", "nodes", "pvcs":
		m.current = paletteNameToView(name)
		m.history = nil
		// Non-drill entry to pods clears any stale scope so the user
		// doesn't see narrowing they didn't ask for. Drilling-in
		// (DrillToPodsMsg) is the only path that re-applies it.
		if m.current == viewPods {
			m.pods = m.pods.WithScope("", "")
		}
		m = m.syncFilterInput()
		go m.persistState()
		return m, m.reloadCmd()
	}
	return m, nil
}

// openContextPicker prepares the runtime context-switch UI: load contexts,
// pre-select the one we're currently on so the user sees their starting
// point, and surface the picker. The picker takes over the frame; mid-
// session esc dismisses without quitting.
func (m Model) openContextPicker() Model {
	contexts, _, _ := k8sclient.Contexts()
	m.availableContexts = contexts
	m.contextPickerSelected = 0
	for i, c := range contexts {
		if c == m.cluster.Context {
			m.contextPickerSelected = i
			break
		}
	}
	m.contextPickerErr = ""
	m.showContextPicker = true
	return m
}

// updateCommandMode handles keystrokes while the inline `:` ex-mode is
// active. Mirrors vim/helix: Tab autocompletes to the longest common prefix,
// ↑/↓ cycles suggestions, Enter runs the highlighted (or exact-match) entry,
// Esc cancels. Unknown commands emit a transient flash error.
func (m Model) updateCommandMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	cmds := components.DefaultCommands()
	suggestions := components.FilterCommands(cmds, m.commandInput.Value())

	switch msg.String() {
	case "esc":
		m.commandMode = false
		m.commandInput.SetValue("")
		m.commandInput.Blur()
		m.commandSel = 0
		return m, nil

	case "enter":
		raw := strings.TrimSpace(m.commandInput.Value())
		// Exact match always wins, even when the substring filter happened
		// to surface multiple candidates — this is what makes `:q` quit
		// without disambiguation prompts.
		var picked *components.Command
		if c := components.ExactCommand(cmds, raw); c != nil {
			picked = c
		} else if len(suggestions) > 0 && m.commandSel < len(suggestions) {
			picked = &suggestions[m.commandSel]
		}
		m.commandMode = false
		m.commandInput.Blur()
		if picked == nil {
			// Per design call: flash a red banner instead of silently
			// dismissing, so the user knows the input wasn't a command.
			m.flashErr = fmt.Sprintf("no command \"%s\"", raw)
			m.commandInput.SetValue("")
			return m, tea.Tick(flashTTL, func(time.Time) tea.Msg { return flashClearMsg{} })
		}
		m.commandInput.SetValue("")
		return m.runCommand(picked.Name)

	case "tab", "right":
		if len(suggestions) > 0 {
			lcp := components.LongestCommonPrefix(suggestions)
			if lcp != "" && len(lcp) > len(m.commandInput.Value()) {
				m.commandInput.SetValue(lcp)
			}
		}
		return m, nil

	case "down", "ctrl+n":
		if m.commandSel < len(suggestions)-1 {
			m.commandSel++
		}
		return m, nil

	case "up", "ctrl+p":
		if m.commandSel > 0 {
			m.commandSel--
		}
		return m, nil
	}

	prev := m.commandInput.Value()
	var inputCmd tea.Cmd
	m.commandInput, inputCmd = m.commandInput.Update(msg)
	if m.commandInput.Value() != prev {
		m.commandSel = 0 // re-query: the previous selection no longer maps to a stable index
	}
	return m, inputCmd
}

// syncFilterInput mirrors the current view's per-view filter into the bottom
// command-bar textinput so each view's filter survives drill-downs and
// round-trips through logs/describe. Sub-views without a filter (Filterable
// not implemented) clear the input.
func (m Model) syncFilterInput() Model {
	if f, ok := m.currentView().(views.Filterable); ok {
		m.filterInput.SetValue(f.Filter())
	} else {
		m.filterInput.SetValue("")
	}
	return m
}

func (m Model) routeToCurrentView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.current {
	case viewPods:
		m.pods, cmd = m.pods.Update(msg)
	case viewDeployments:
		m.deployments, cmd = m.deployments.Update(msg)
	case viewServices:
		m.services_, cmd = m.services_.Update(msg)
	case viewSecrets:
		m.secrets, cmd = m.secrets.Update(msg)
	case viewConfigMaps:
		m.configmaps, cmd = m.configmaps.Update(msg)
	case viewNamespaces:
		m.namespaces, cmd = m.namespaces.Update(msg)
	case viewNodes:
		m.nodes, cmd = m.nodes.Update(msg)
	case viewPVCs:
		m.pvcs, cmd = m.pvcs.Update(msg)
	case viewLogs:
		m.logs, cmd = m.logs.Update(msg)
	case viewDescribe:
		m.describe, cmd = m.describe.Update(msg)
	case viewGenericDescribe:
		m.genericDescribe, cmd = m.genericDescribe.Update(msg)
	}
	return m, cmd
}

func (m Model) reloadCmd() tea.Cmd {
	switch m.current {
	case viewPods:
		return func() tea.Msg { return k8sclient.PodsUpdatedMsg{} }
	case viewDeployments:
		return func() tea.Msg { return k8sclient.DeploymentsUpdatedMsg{} }
	case viewServices:
		return func() tea.Msg { return k8sclient.ServicesUpdatedMsg{} }
	case viewSecrets:
		return func() tea.Msg { return k8sclient.SecretsUpdatedMsg{} }
	case viewConfigMaps:
		return func() tea.Msg { return k8sclient.ConfigMapsUpdatedMsg{} }
	case viewNamespaces:
		return func() tea.Msg { return k8sclient.NamespacesUpdatedMsg{} }
	case viewNodes:
		return func() tea.Msg { return k8sclient.NodesUpdatedMsg{} }
	case viewPVCs:
		return func() tea.Msg { return k8sclient.PVCsUpdatedMsg{} }
	}
	return nil
}

// View composes the modern shell:
//
//	┌─────────────────────────────────────────────────────────────────┐
//	│ ctx maisa-sdlc · v1.30 · ▆ europa  ── K L E N S ──     ● live   │ top bar
//	├─────────────────────────────────────────────────────────────────┤
//	│ ▌1 pods 4/23   2 deployments 14   3 services 12   ...           │ nav strip
//	│ filter chips ........................                           │ chips
//	│                                                                 │
//	│ table                                          │ details        │ content
//	│                                                                 │
//	├─────────────────────────────────────────────────────────────────┤
//	│ › / type to filter         ↵ describe   l logs   / filter   ?   │ command bar
//	└─────────────────────────────────────────────────────────────────┘
//
// The vertical nav rail was replaced by a horizontal strip directly under the
// top bar — gives the table full horizontal real estate, drops the cluster-
// meta block (cpu/mem are unwired anyway), and consolidates the count onto
// the active nav item so it doesn't duplicate the resource label or scope.
//
// The palette overlay replaces the entire frame when open; lipgloss has no
// real cell-coordinate overlay support, so this matches klens's pre-redesign
// behaviour.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}
	// Startup cluster picker takes over the whole frame — no client to drive
	// any view yet, so we render it on a blank canvas instead of the shell.
	if m.showContextPicker {
		return components.ContextPicker(m.width, m.height,
			m.availableContexts, m.contextPickerSelected, m.contextPickerErr)
	}
	v := m.currentView()

	visible, total := v.Count()
	top := layout.TopBar(m.width, layout.TopBarConfig{
		Context:      fallback(m.cluster.Context, "—"),
		Cluster:      fallback(m.cluster.Cluster, "—"),
		User:         fallback(m.cluster.User, "—"),
		K8sVersion:   fallback(m.cluster.K8sVersion, "—"),
		Region:       fallback(m.cluster.Region, "—"),
		KlensVer:     fallback(m.cluster.KlensVer, "dev"),
		Namespace:    fallback(m.namespace, "all"),
		Resource:     v.Title(),
		Live:         m.client != nil,
		VisibleCount: visible,
		TotalCount:   total,
		Totals:       m.totals(),
	})

	// Sub-views (logs, describe, genericDescribe) take the full content area —
	// hide the right details pane so their content isn't squashed.
	showDetails := m.width >= minDetailsAt &&
		m.current != viewLogs &&
		m.current != viewDescribe &&
		m.current != viewGenericDescribe

	// Inline ex-mode docks a one-line suggestions strip just above the
	// command bar, so we have to budget that row off the content area or the
	// table would push the prompt off-screen.
	extraBottom := 0
	if m.commandMode {
		extraBottom = 1
	}
	// Inner content height = total height minus everything else, including
	// the focus frame's two border rows. contentH counts table rows; the
	// chip strip lives above the table inside the frame, so its row is
	// already accounted for here.
	contentH := m.height - topBarHeight - cmdBarHeight - chipsHeight - extraBottom - frameH
	if contentH < 1 {
		contentH = 1
	}

	innerW := m.width - frameW
	if innerW < 1 {
		innerW = 1
	}
	detW := 0
	if showDetails {
		detW = detailsWidth
	}
	midW := innerW - detW

	chips := layout.FilterChips(midW, v.Chips(), visible, total)
	tbl := v.Table(midW, contentH)
	center := lipgloss.JoinVertical(lipgloss.Left, chips, tbl)

	cols := []string{center}
	if showDetails {
		cols = append(cols, v.Details(detW, contentH+chipsHeight))
	}
	row := lipgloss.JoinHorizontal(lipgloss.Top, cols...)

	// Wrap the content in a rounded focus frame. K9s-style accent border
	// makes the active pane unambiguous and gives the table edges a clean
	// boundary; we keep the resource title in the top bar rather than
	// inset on the border (lipgloss has no native inset-label support and
	// ANSI-aware splicing is fragile across terminal emulators).
	row = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorBorderFaint).
		Render(row)

	// Bottom region. In default mode this is just the command bar. In inline
	// ex-mode (`:` typed), we replace it with a 2-line block: a suggestions
	// strip with the highlighted match plus the `: <input>` prompt. The flash
	// banner overrides everything when set, so the user sees feedback before
	// the next keystroke clears it.
	bottom := m.renderBottom(v)

	frame := lipgloss.JoinVertical(lipgloss.Left, top, row, bottom)

	if m.showPalette {
		modal := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.ColorAccent).
			Padding(0, 1).
			Render(m.palette.View(60))
		return overlayCentered(frame, modal, m.width, m.height)
	}
	// Help overlay also paints over the frame so the user keeps context while
	// reading the keymap. Source of truth for keys is the active view's
	// KeyMap() when implemented, falling back to KeyHints().
	if m.showHelp {
		body := components.HelpBody(v.Title(), helpSpecs(v))
		return overlayCentered(frame, body, m.width, m.height)
	}
	return frame
}

// overlayCentered paints `modal` over `frame` centered in (width, height).
// Falls back to lipgloss.Place if the modal is bigger than the frame.
func overlayCentered(frame, modal string, width, height int) string {
	mw := lipgloss.Width(modal)
	mh := lipgloss.Height(modal)
	if mw >= width || mh >= height {
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, modal)
	}
	col := (width - mw) / 2
	row := (height - mh) / 2
	if col < 0 {
		col = 0
	}
	if row < 0 {
		row = 0
	}
	return components.Overlay(frame, modal, col, row)
}

// helpSpecs returns the KeySpec list a view exposes to the `?` overlay.
// Views that implement views.KeyMap surface the full keymap (including Soon
// entries advertising upcoming wave items); the rest fall back to KeyHints
// converted into KeySpecs so every view is at least minimally documented in
// the overlay.
func helpSpecs(v views.View) []components.KeySpec {
	if km, ok := v.(views.KeyMap); ok {
		return km.KeyMap()
	}
	hints := v.KeyHints()
	out := make([]components.KeySpec, 0, len(hints))
	for _, h := range hints {
		out = append(out, components.KeySpec{Key: h.Key, Label: h.Label})
	}
	return out
}

// updateContextPicker handles keystrokes while the cluster picker is up.
// The picker has two modes:
//   - Startup (m.client == nil): esc quits, since there's nothing else to
//     return to. This is what `klens` shows when kubeconfig has no current-
//     context.
//   - Runtime (m.client != nil): esc dismisses the picker and returns to
//     the previous view. Triggered from the palette / `:ctx` ex-mode.
//
// Picking the same context the user is already on is a no-op — we don't
// rebuild services or restart the watcher when nothing actually changed.
func (m Model) updateContextPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	runtime := m.client != nil
	switch msg.String() {
	case "esc":
		if runtime {
			m.showContextPicker = false
			m.contextPickerErr = ""
			return m, nil
		}
		return m, tea.Quit
	case "j", "down", "ctrl+n":
		if m.contextPickerSelected < len(m.availableContexts)-1 {
			m.contextPickerSelected++
		}
	case "k", "up", "ctrl+p":
		if m.contextPickerSelected > 0 {
			m.contextPickerSelected--
		}
	case "enter":
		if len(m.availableContexts) == 0 {
			return m, tea.Quit
		}
		picked := m.availableContexts[m.contextPickerSelected]
		// Same context → just dismiss. Saves a watcher tear-down + a fresh
		// list across every resource.
		if runtime && picked == m.cluster.Context {
			m.showContextPicker = false
			m.contextPickerErr = ""
			return m, nil
		}
		cfg, _ := config.Load("")
		client, err := k8sclient.NewClientForContext(cfg.Kubeconfig, picked)
		if err != nil {
			m.contextPickerErr = err.Error()
			return m, nil
		}
		m = m.attachClient(client, cfg)
		m.showContextPicker = false
		m.contextPickerErr = ""
		// Mid-session: tear down the old watcher and start a fresh one so
		// informers / metrics ticks bind to the new cluster. main.go owns
		// the watcher pointer; we invoke it through restartWatcherRef.
		if runtime && m.restartWatcherRef != nil && *m.restartWatcherRef != nil {
			(*m.restartWatcherRef)(client, m.namespace, m.services.Metrics, m.services.Logs)
		}
		// Refetch every resource type so all per-view counts and tables
		// repopulate against the new cluster, mirroring Init().
		return m, tea.Batch(
			func() tea.Msg { return k8sclient.PodsUpdatedMsg{} },
			func() tea.Msg { return k8sclient.DeploymentsUpdatedMsg{} },
			func() tea.Msg { return k8sclient.ServicesUpdatedMsg{} },
			func() tea.Msg { return k8sclient.SecretsUpdatedMsg{} },
			func() tea.Msg { return k8sclient.ConfigMapsUpdatedMsg{} },
			func() tea.Msg { return k8sclient.NamespacesUpdatedMsg{} },
			func() tea.Msg { return k8sclient.NodesUpdatedMsg{} },
			func() tea.Msg { return k8sclient.PVCsUpdatedMsg{} },
		)
	}
	return m, nil
}

// commandBarInput returns the raw textinput.View() — layout.CommandBar prepends
// the "›" + "/" prompt so we don't add it here.
func (m Model) commandBarInput() string {
	return m.filterInput.View()
}

// renderBottom assembles the bottom of the frame in three flavors, in
// priority order:
//   - Inline ex-mode (`:`): suggestions strip + `: <input>` prompt (2 rows).
//   - Flash banner: a 1-row red banner replacing the command bar to surface
//     transient errors (e.g. unknown command), auto-cleared by flashClearMsg.
//   - Default: the regular layout.CommandBar with key hints.
//
// The 2-row ex-mode geometry is budgeted in View() via extraBottom; flash
// keeps the original 1-row footprint so it doesn't reflow the table.
func (m Model) renderBottom(v views.View) string {
	if m.commandMode {
		cmds := components.DefaultCommands()
		suggestions := components.FilterCommands(cmds, m.commandInput.Value())
		strip := renderSuggestionsStrip(m.width, suggestions, m.commandSel)
		prompt := renderCommandPrompt(m.width, m.commandInput.View())
		return lipgloss.JoinVertical(lipgloss.Left, strip, prompt)
	}
	if m.flashErr != "" {
		return renderFlashBanner(m.width, m.flashErr)
	}
	hints := append([]layout.KeyHint{}, v.KeyHints()...)
	hints = append(hints, layout.KeyHint{Key: "?", Label: "help"})
	return layout.CommandBar(m.width, m.commandBarInput(), hints)
}

// renderSuggestionsStrip renders the type-ahead candidates docked above the
// `:` prompt. The selected item is bolded in accent; siblings are dimmed.
// Long lists silently truncate at the right edge — the modal palette
// (ctrl+p) is the discoverability surface for the full set.
func renderSuggestionsStrip(width int, suggestions []components.Command, selected int) string {
	if width < 1 {
		width = 1
	}
	if len(suggestions) == 0 {
		empty := theme.Faint.Render("no matches — Tab autocompletes, Esc cancels")
		return theme.Panel.Width(width).Padding(0, 1).Render(empty)
	}
	parts := make([]string, 0, len(suggestions))
	for i, s := range suggestions {
		label := s.Name
		if s.Alias != "" {
			label = s.Name + " " + theme.Faint.Render(s.Alias)
		}
		if i == selected {
			label = lipgloss.NewStyle().
				Foreground(theme.ColorAccent).
				Bold(true).
				Render("▌ " + s.Name + " " + theme.Faint.Render(s.Alias))
		} else {
			label = "  " + theme.Mid.Render(label)
		}
		parts = append(parts, label)
	}
	line := strings.Join(parts, "  ")
	// Width-clip at the right so an overflow strip doesn't push the prompt
	// onto a second visual row.
	if w := lipgloss.Width(line); w > width-2 {
		line = lipgloss.NewStyle().MaxWidth(width - 4).Render(line) + theme.Faint.Render(" …")
	}
	return theme.Panel.Width(width).Padding(0, 1).Render(line)
}

// renderCommandPrompt is the inline ex-mode input row — accent ":" prompt,
// bubbles textinput in the middle, and a couple of hint chips on the right.
func renderCommandPrompt(width int, inputView string) string {
	if width < 1 {
		width = 1
	}
	prompt := lipgloss.NewStyle().Foreground(theme.ColorAccent).Bold(true).Render(":") + " "
	left := prompt + inputView

	hints := []layout.KeyHint{
		{Key: "↵", Label: "run"},
		{Key: "⇥", Label: "complete"},
		{Key: "⎋", Label: "cancel"},
	}
	chips := make([]string, 0, len(hints))
	for _, h := range hints {
		chips = append(chips, theme.KeyChip.Render(h.Key)+theme.Dim.Render(" "+h.Label))
	}
	right := strings.Join(chips, "  ")

	inner := width - 2
	if inner < 1 {
		inner = 1
	}
	gap := inner - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	line := left + strings.Repeat(" ", gap) + right
	return theme.Panel.Width(width).Padding(0, 1).Render(line)
}

// renderFlashBanner shows a transient red banner for unknown-command errors
// from the inline ex-mode. flashClearMsg removes it after flashTTL so it
// doesn't linger past the user's next interaction.
func renderFlashBanner(width int, err string) string {
	if width < 1 {
		width = 1
	}
	style := lipgloss.NewStyle().
		Foreground(theme.ColorError).
		Bold(true)
	body := style.Render("✕ "+err) + theme.Faint.Render("  (any key to dismiss)")
	return theme.Panel.Width(width).Padding(0, 1).Render(body)
}

// currentView returns the view for the active viewKind as a views.View
// interface, so the shell can call Table/Details/Chips/KeyHints generically.
func (m Model) currentView() views.View {
	switch m.current {
	case viewPods:
		return m.pods
	case viewDeployments:
		return m.deployments
	case viewServices:
		return m.services_
	case viewSecrets:
		return m.secrets
	case viewConfigMaps:
		return m.configmaps
	case viewNamespaces:
		return m.namespaces
	case viewNodes:
		return m.nodes
	case viewPVCs:
		return m.pvcs
	case viewLogs:
		return m.logs
	case viewDescribe:
		return m.describe
	case viewGenericDescribe:
		return m.genericDescribe
	}
	return m.pods
}

// totals returns the legacy aggregate counter set. The top bar's TopBarConfig
// still has a Totals field for backwards compat; the bar itself no longer
// renders it (the resource label + V/T count is shown inline instead).
func (m Model) totals() layout.Totals {
	_, p := m.pods.Count()
	_, d := m.deployments.Count()
	_, s := m.services_.Count()
	return layout.Totals{Pods: p, Deployments: d, Services: s}
}

func paletteNameToView(name string) viewKind {
	switch name {
	case "pods":
		return viewPods
	case "deployments":
		return viewDeployments
	case "services":
		return viewServices
	case "secrets":
		return viewSecrets
	case "configmaps":
		return viewConfigMaps
	case "namespaces":
		return viewNamespaces
	case "nodes":
		return viewNodes
	case "pvcs":
		return viewPVCs
	}
	return viewPods
}

func fallback(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

