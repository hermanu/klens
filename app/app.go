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

// Canonical resource names shared between viewKindName, paletteNameToView,
// navRailConfig, and railOrder. Defined as constants to satisfy goconst.
const (
	viewNamePods        = "pods"
	viewNameDeployments = "deployments"
	viewNameServices    = "services"
	viewNameSecrets     = "secrets"
	viewNameConfigMaps  = "configmaps"
	viewNameNamespaces  = "namespaces"
	viewNameNodes       = "nodes"
	viewNamePVCs        = "pvcs"
)

// Geometry — bordered-panel shell. Every pane carries its own 1-row border
// top + bottom, so the chrome cost is higher than the previous outer-frame
// shell. minDetailsAt unchanged (right column drops below 120 cols).
const (
	navRailWidth     = 22
	detailsWidth     = 44
	topBarRowsWide   = 4   // 1 top border + 2 body + 1 bottom border
	topBarRowsNarrow = 3   // 1 top border + 1 body + 1 bottom border
	topBarWideAt     = 64  // width >= this enables the block logo + 3-col body
	cmdBarRows       = 4   // 1 top border + 2 body + 1 bottom border
	minDetailsAt     = 120 // unchanged — drop right column below this width
	minNavRailAt     = 60  // below this, hide the rail too
)

// Model is the root Bubble Tea model. It owns all views, the input, the
// palette, and the cluster info shown in the top bar.
type Model struct {
	client    *k8sclient.Client
	services  port.Services
	namespace string // active namespace filter ("" = all)
	cluster   ClusterInfo
	buildInfo BuildInfo

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
		return viewNamePods
	case viewDeployments:
		return viewNameDeployments
	case viewServices:
		return viewNameServices
	case viewSecrets:
		return viewNameSecrets
	case viewConfigMaps:
		return viewNameConfigMaps
	case viewNamespaces:
		return viewNameNamespaces
	case viewNodes:
		return viewNameNodes
	case viewPVCs:
		return viewNamePVCs
	default:
		return "" // transient sub-views (logs, describe) are not persisted
	}
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
		// A broken ~/.klens/config.yaml (manual edit, stale fields from an
		// older release, etc.) used to crash the binary at startup. Surface
		// the parse error and proceed with whatever Load could recover plus
		// defaults — losing the persisted namespace + last view is preferable
		// to refusing to launch.
		fmt.Fprintf(os.Stderr, "warn: ignoring broken config: %v\n", err)
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

// CurrentResource returns the active view's canonical name (pods, deployments,
// ...). Exposed so tests can verify mnemonic / bracket navigation without
// poking package-internal state.
func (m Model) CurrentResource() string {
	return viewKindName(m.current)
}

// Init implements tea.Model. It fires one UpdatedMsg per resource type so all
// view counts populate on first render, not just the focused view. No-ops when
// client is nil (context picker / offline boot).
func (m Model) Init() tea.Cmd {
	// No cluster wired (CI / context picker / boot before kubeconfig
	// resolves) → no fetches. Without this guard the *UpdatedMsg
	// broadcast trickles into views whose service interfaces are nil,
	// and a method call on a nil port.PodService panics with
	// "invalid memory address or nil pointer dereference".
	if m.client == nil {
		return nil
	}
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

// Update implements tea.Model. It routes messages through the global key handler,
// the active sub-view, or the broadcast path (watcher events) and returns an updated
// Model plus any Cmds to run.
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
		if capt, ok := m.currentView().(views.Capturing); ok && capt.CapturesKeys() {
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
	cmds := make([]tea.Cmd, 0, 11)
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
	// Mnemonics — only on top-level list views (skip when in logs/describe/
	// genericDescribe so digit keys there still work for view-local features
	// like the logs view's 1-5 lookback presets).
	if m.isTopLevelList() {
		switch msg.String() {
		case "1":
			return m.runCommand(viewNamePods)
		case "2":
			return m.runCommand(viewNameDeployments)
		case "3":
			return m.runCommand(viewNameServices)
		case "4":
			return m.runCommand(viewNameNodes)
		case "5":
			return m.runCommand(viewNameConfigMaps)
		case "6":
			return m.runCommand(viewNameSecrets)
		case "7":
			return m.runCommand(viewNameNamespaces)
		case "8":
			return m.runCommand(viewNamePVCs)
		case "[":
			return m.runCommand(cyclePrev(viewKindName(m.current)))
		case "]":
			return m.runCommand(cycleNext(viewKindName(m.current)))
		}
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
	case "all":
		// Clear the namespace scope so the current view lists across every
		// namespace. Reuse the NamespaceSelectedMsg path so the handler
		// fires the same broadcast + per-resource reload it does for a
		// regular ns switch (Enter on a row in the namespaces view).
		return m, func() tea.Msg { return views.NamespaceSelectedMsg{Name: ""} }
	case viewNamePods, viewNameDeployments, viewNameServices, viewNameSecrets,
		viewNameConfigMaps, viewNameNamespaces, viewNameNodes, viewNamePVCs:
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
			m.flashErr = fmt.Sprintf("no command %q", raw)
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
	default:
		return nil // sub-views (logs, describe) have no periodic reload
	}
}

// View composes the bordered-panel shell:
//
//	┌─ ◎ KLENS v0.3.0 · build … ──────────────── ● watching ─┐  top bar
//	│  block logo   ctx … cluster …    nodes 9/9              │
//	│               user … k8s …       cpu ▃▅▇█▇ 62%          │
//	└──────────────────────────────────────────────────────────┘
//	┌─ RESOURCES ─┐ ┌─ PODS [4/25] ─────────┐ ┌─ FOCUS ↵ desc ─┐
//	│ ▌ 1 pods 23 │ │   NS  NAME  READY ... │ │ api-gateway-…  │
//	│   2 deps 18 │ │ ▌ pl  api-… 2/2  ...  │ │ ...            │
//	│   ...       │ │ ...                   │ │ METRICS / CONT │
//	└─ [ ] cycle ─┘ └─ 4 / 25 · j/k …───────┘ └ l logs · s sh ─┘
//	┌─ COMMAND ────────────────────────────────────────────────┐
//	│ › / type to filter…                                       │
//	│ <↵> describe  <l> logs  <s> shell  <e> edit  ...          │
//	└───────────────────────────────────────────────────────────┘
//
// Modal palette and help overlays still paint on top via Overlay.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}
	if m.showContextPicker {
		return components.ContextPicker(m.width, m.height,
			m.availableContexts, m.contextPickerSelected, m.contextPickerErr)
	}
	v := m.currentView()
	visible, total := v.Count()

	// Heights
	topBarH := topBarRowsWide
	if m.width < topBarWideAt {
		topBarH = topBarRowsNarrow
	}
	extraBottom := 0
	if m.commandMode {
		extraBottom = 1
	}
	midH := max(m.height-topBarH-cmdBarRows-extraBottom, 5)

	// Widths
	showRail := m.width >= minNavRailAt && m.isTopLevelList()
	showDetails := m.width >= minDetailsAt &&
		m.current != viewLogs &&
		m.current != viewDescribe &&
		m.current != viewGenericDescribe
	railW := 0
	detW := 0
	if showRail {
		railW = navRailWidth
	}
	if showDetails {
		detW = detailsWidth
	}
	tableW := max(m.width-railW-detW, 20)

	cm := m.clusterMeta()

	// 1. Top bar panel
	topCfg := layout.TopBarConfig{
		Context:    fallback(m.cluster.Context, "—"),
		Cluster:    fallback(m.cluster.Cluster, "—"),
		User:       fallback(m.cluster.User, "—"),
		K8sVersion: fallback(m.cluster.K8sVersion, "—"),
		Region:     fallback(m.cluster.Region, "—"),
		KlensVer:   fallback(m.cluster.KlensVer, m.buildInfo.Version),
		BuildID:    m.buildID(),
		Uptime:     cm.Uptime,
		NodesReady: cm.NodesReady,
		NodesTotal: cm.NodesTotal,
		CPUSamples: cm.CPUSamples,
		CPUPercent: cm.CPUPercent,
		Namespace:  fallback(m.namespace, "all"),
		Resource:   v.Title(),
		Live:       m.client != nil,
	}
	pulseOn := (time.Now().UnixMilli()/700)%2 == 0
	topPanel := components.Panel(components.PanelConfig{
		Width:  m.width,
		Height: topBarH,
		Title:  layout.TopBarTitle(topCfg),
		Foot:   layout.TopBarFoot(pulseOn, topCfg.Live),
		Body:   layout.TopBar(m.width-2, topCfg),
	})

	// 2. Mid row: rail | table | details
	var midPanels []string
	if showRail {
		railBody := layout.NavRail(railW-2, midH-2, m.navRailConfig(cm))
		railPanel := components.Panel(components.PanelConfig{
			Width:  railW,
			Height: midH,
			Title:  lipgloss.NewStyle().Foreground(theme.ColorAccent).Bold(true).Render("RESOURCES"),
			Foot:   lipgloss.NewStyle().Foreground(theme.ColorMuted).Render("[ ] cycle"),
			Body:   railBody,
		})
		midPanels = append(midPanels, railPanel)
	}

	tableBody := v.Table(tableW-2, midH-2)
	tableTitle := tablePanelTitle(v.Title(), visible, total, m.pods.Scope())
	tableFoot := tableFootForView(v, visible, total, tableW-4)
	tablePanel := components.Panel(components.PanelConfig{
		Width:  tableW,
		Height: midH,
		Title:  tableTitle,
		Foot:   tableFoot,
		Active: !m.commandMode && !m.filterFocused && !m.showPalette,
		Body:   tableBody,
	})
	midPanels = append(midPanels, tablePanel)

	if showDetails {
		detBody := v.Details(detW-2, midH-2)
		detPanel := components.Panel(components.PanelConfig{
			Width:  detW,
			Height: midH,
			Title:  lipgloss.NewStyle().Foreground(theme.ColorAccent).Bold(true).Render("FOCUS"),
			Foot:   detailsFootForView(v),
			Body:   detBody,
		})
		midPanels = append(midPanels, detPanel)
	}
	midRow := lipgloss.JoinHorizontal(lipgloss.Top, midPanels...)

	// 3. Command bar panel. ex-mode and flash banner override the default
	// 2-line body so the inline-`:` UX and the unknown-command flash from
	// the previous shell still surface.
	cmdBody := m.renderCmdBody(v, m.width-2)
	cmdPanel := components.Panel(components.PanelConfig{
		Width:  m.width,
		Height: cmdBarRows,
		Title:  lipgloss.NewStyle().Foreground(theme.ColorAccent).Bold(true).Render("COMMAND"),
		Active: m.commandMode || m.filterFocused,
		Body:   cmdBody,
	})

	frame := lipgloss.JoinVertical(lipgloss.Left, topPanel, midRow, cmdPanel)

	// Modal palette / help overlays (existing logic preserved).
	if m.showPalette {
		modal := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.ColorAccent).
			Padding(0, 1).
			Render(m.palette.View(60))
		return overlayCentered(frame, modal, m.width, m.height)
	}
	if m.showHelp {
		body := components.HelpBody(v.Title(), helpSpecs(v))
		return overlayCentered(frame, body, m.width, m.height)
	}
	return frame
}

// navRailConfig builds the NavRailConfig from current model state.
func (m Model) navRailConfig(cm clusterMeta) layout.NavRailConfig {
	items := []layout.NavItem{
		{Mnemonic: "1", Label: viewNamePods, Active: m.current == viewPods},
		{Mnemonic: "2", Label: viewNameDeployments, Active: m.current == viewDeployments},
		{Mnemonic: "3", Label: viewNameServices, Active: m.current == viewServices},
		{Mnemonic: "4", Label: viewNameNodes, Active: m.current == viewNodes},
		{Mnemonic: "5", Label: viewNameConfigMaps, Active: m.current == viewConfigMaps},
		{Mnemonic: "6", Label: viewNameSecrets, Active: m.current == viewSecrets},
		{Mnemonic: "7", Label: viewNameNamespaces, Active: m.current == viewNamespaces},
		{Mnemonic: "8", Label: viewNamePVCs, Active: m.current == viewPVCs},
	}
	_, p := m.pods.Count()
	_, d := m.deployments.Count()
	_, s := m.services_.Count()
	_, n := m.nodes.Count()
	_, c := m.configmaps.Count()
	_, sec := m.secrets.Count()
	_, ns := m.namespaces.Count()
	_, pv := m.pvcs.Count()
	counts := []int{p, d, s, n, c, sec, ns, pv}
	for i := range items {
		items[i].Count = counts[i]
	}
	return layout.NavRailConfig{
		Items: items,
		Cluster: layout.ClusterMeta{
			NodesReady: cm.NodesReady,
			NodesTotal: cm.NodesTotal,
			Pods:       cm.Pods,
			CPUSamples: cm.CPUSamples,
			MEMSamples: cm.MEMSamples,
			CPUPercent: cm.CPUPercent,
			MEMPercent: cm.MEMPercent,
		},
	}
}

// tablePanelTitle renders the table panel's notched title, including the
// drill scope chip when set on the pods view.
func tablePanelTitle(resource string, visible, total int, scope string) string {
	title := lipgloss.NewStyle().Foreground(theme.ColorAccent).Bold(true).Render(strings.ToUpper(resource))
	count := lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(fmt.Sprintf(" [%d]", total))
	if visible != total {
		count = lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(
			fmt.Sprintf(" [%d/%d]", visible, total))
	}
	if scope != "" {
		count = lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(
			fmt.Sprintf(" [%d/%d · scope: %s]", visible, total, scope))
	}
	return title + count
}

// tableFootForView assembles the table panel's bottom-right foot — just
// the visible/total count. Key hints intentionally live ONLY in the
// command bar's hint row; duplicating them here doubled the noise
// (every keymap shortcut rendered twice on every frame).
func tableFootForView(_ views.View, visible, total, _ int) string {
	dim := lipgloss.NewStyle().Foreground(theme.ColorMuted)
	if visible == total {
		return dim.Render(fmt.Sprintf("%d", total))
	}
	return dim.Render(fmt.Sprintf("%d / %d", visible, total))
}

// detailsFootForView returns the details panel's foot. Currently empty:
// the previous draft rendered the view's key hints, but the command bar
// already shows the full keymap and duplicating it on the focus panel
// added visual noise without adding information.
func detailsFootForView(_ views.View) string {
	return ""
}

// renderCmdBody returns the 2-line body content for the command panel.
// Priority order: ex-mode (`:`) > flash banner > default hints row. All
// three return strings exactly 2 rows tall so the cmd panel height stays
// constant (no reflow when entering/leaving ex-mode).
func (m Model) renderCmdBody(v views.View, innerW int) string {
	if innerW < 1 {
		innerW = 1
	}
	if m.commandMode {
		cmds := components.DefaultCommands()
		suggestions := components.FilterCommands(cmds, m.commandInput.Value())
		strip := renderSuggestionsStrip(innerW, suggestions, m.commandSel)
		prompt := renderCommandPrompt(innerW, m.commandInput.View())
		return strip + "\n" + prompt
	}
	if m.flashErr != "" {
		banner := renderFlashBanner(innerW, m.flashErr)
		return banner + "\n" + strings.Repeat(" ", innerW)
	}
	hints := append([]layout.KeyHint{}, v.KeyHints()...)
	hints = append(hints, layout.KeyHint{Key: "?", Label: "help"})
	return layout.CommandBar(innerW, m.commandBarInput(), hints)
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

// renderSuggestionsStrip renders the type-ahead candidates docked above the
// `:` prompt. The selected item is bolded in accent; siblings are dimmed.
// Long lists silently truncate at the right edge — the modal palette
// (ctrl+p) is the discoverability surface for the full set.
// width is the INNER content width (panel border already excluded by caller).
func renderSuggestionsStrip(width int, suggestions []components.Command, selected int) string {
	if width < 1 {
		width = 1
	}
	if len(suggestions) == 0 {
		empty := theme.Faint.Render("no matches — Tab autocompletes, Esc cancels")
		return lipgloss.NewStyle().Width(width).Render(empty)
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
	if w := lipgloss.Width(line); w > width {
		line = lipgloss.NewStyle().MaxWidth(width-2).Render(line) + theme.Faint.Render(" …")
	}
	return lipgloss.NewStyle().Width(width).Render(line)
}

// renderCommandPrompt is the inline ex-mode input row — accent ":" prompt,
// bubbles textinput in the middle, and a couple of hint chips on the right.
// width is the INNER content width (panel border already excluded by caller).
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

	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	line := left + strings.Repeat(" ", gap) + right
	return lipgloss.NewStyle().Width(width).Render(line)
}

// renderFlashBanner shows a transient red banner for unknown-command errors
// from the inline ex-mode. flashClearMsg removes it after flashTTL so it
// doesn't linger past the user's next interaction.
// width is the INNER content width (panel border already excluded by caller).
func renderFlashBanner(width int, err string) string {
	if width < 1 {
		width = 1
	}
	style := lipgloss.NewStyle().
		Foreground(theme.ColorError).
		Bold(true)
	body := style.Render("✕ "+err) + theme.Faint.Render("  (any key to dismiss)")
	return lipgloss.NewStyle().Width(width).Render(body)
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

func paletteNameToView(name string) viewKind {
	switch name {
	case viewNamePods:
		return viewPods
	case viewNameDeployments:
		return viewDeployments
	case viewNameServices:
		return viewServices
	case viewNameSecrets:
		return viewSecrets
	case viewNameConfigMaps:
		return viewConfigMaps
	case viewNameNamespaces:
		return viewNamespaces
	case viewNameNodes:
		return viewNodes
	case viewNamePVCs:
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

// clusterMeta aggregates cluster-wide stats for the nav rail's CLUSTER
// footer and the top bar's right-aligned meta. All sources are best-effort:
// missing data renders as "—" / -1 in the relevant fields.
type clusterMeta struct {
	NodesReady int
	NodesTotal int
	Pods       int
	CPUSamples []float64
	MEMSamples []float64
	CPUPercent int
	MEMPercent int
	Uptime     string
}

func (m Model) clusterMeta() clusterMeta {
	cm := clusterMeta{
		CPUPercent: -1,
		MEMPercent: -1,
	}

	// Pod count comes from the visible pods view total.
	_, total := m.pods.Count()
	cm.Pods = total

	// Node count comes from the nodes view's total. Per-node readiness is
	// not exposed by the View interface today; treat all known nodes as
	// ready until a public ready/total accessor lands.
	// TODO(node-readiness): expose ready breakdown without leaking k8s.io into views.
	_, nTotal := m.nodes.Count()
	if nTotal > 0 {
		cm.NodesReady = nTotal
		cm.NodesTotal = nTotal
	}

	// CPU / MEM aggregate samples + percent are not tracked at the model
	// level today; defaults render "—". Aggregation is a follow-up.
	// TODO(cluster-metrics): aggregate MetricsTickMsg samples into the model.

	return cm
}

// buildID resolves the build identifier shown in the top bar title.
// Preference: main.date short → main.commit short → "dev". main.go passes
// these in via WithBuildInfo at construction time.
func (m Model) buildID() string {
	if d := m.buildInfo.Date; d != "" && d != "unknown" {
		// Trim ISO timestamp to YYMMDD, e.g. "2026-05-11T12:34:56Z" → "260511".
		if len(d) >= 10 {
			return strings.ReplaceAll(d[2:10], "-", "")
		}
		return d
	}
	if c := m.buildInfo.Commit; c != "" && c != "none" {
		if len(c) > 7 {
			return c[:7]
		}
		return c
	}
	return "dev"
}

// BuildInfo carries the ldflags-injected version metadata into the model.
type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

// WithBuildInfo returns a copy of m with build metadata set. main.go calls
// this so the top-bar title can render the version + commit/date without
// importing main's vars directly.
func (m Model) WithBuildInfo(b BuildInfo) Model {
	m.buildInfo = b
	return m
}

// isTopLevelList reports whether the current view is one of the 8 mnemonic
// list views (gates digit + bracket nav so sub-views don't lose their own
// digit handling).
func (m Model) isTopLevelList() bool {
	switch m.current {
	case viewPods, viewDeployments, viewServices, viewSecrets,
		viewConfigMaps, viewNamespaces, viewNodes, viewPVCs:
		return true
	case viewLogs, viewDescribe, viewGenericDescribe:
		return false
	}
	return false
}

// railOrder is the canonical resource-rail order — must match the rail's
// item slice exactly so `[`/`]` cycle through the same set the rail shows.
var railOrder = []string{
	viewNamePods, viewNameDeployments, viewNameServices, viewNameNodes,
	viewNameConfigMaps, viewNameSecrets, viewNameNamespaces, viewNamePVCs,
}

// cyclePrev returns the previous rail entry, wrapping at the start. Returns
// viewNamePods if `current` is unknown.
func cyclePrev(current string) string {
	for i, name := range railOrder {
		if name == current {
			return railOrder[(i-1+len(railOrder))%len(railOrder)]
		}
	}
	return viewNamePods
}

// cycleNext returns the next rail entry, wrapping at the end. Returns
// viewNamePods if `current` is unknown.
func cycleNext(current string) string {
	for i, name := range railOrder {
		if name == current {
			return railOrder[(i+1)%len(railOrder)]
		}
	}
	return viewNamePods
}
