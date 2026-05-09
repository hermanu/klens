package app

import (
	"fmt"
	"os"
	"strings"

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

// Geometry — the modern shell's pane sizes. The horizontal layout has three
// fixed-height chrome rows above the content area (top bar + divider, nav
// strip, filter chips) and the command bar on the bottom. The right details
// pane drops below 120 cols so the table never gets squeezed under ~80.
const (
	detailsWidth   = 44
	topBarHeight   = 2 // 1 content row + 1 divider
	navStripHeight = 2 // 1 content row + 1 divider so the strip reads as its own band
	cmdBarHeight   = 1
	chipsHeight    = 1
	minDetailsAt   = 120
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

	filterInput   textinput.Model
	filterFocused bool // when true, keystrokes go to filterInput; otherwise to view

	// logTailRef is a pointer to a shared function slot. Both the local Model
	// in main.go and the copy held inside tea.NewProgram dereference the same
	// pointer, so SetLogTailStarter (called after tea.NewProgram, when the
	// watcher exists) propagates to the live model running in the program.
	logTailRef *func(ns string, pods []string, sinceSeconds int64)

	// history is the navigation stack. Drill-downs (Enter on a deployment /
	// service / namespace; `l` on a pod) push the current view; Esc on a
	// drilled view pops back. mnemonic 1-8 always clears the stack so we
	// don't ricochet between unrelated jumps.
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

// persistState writes the current namespace and active resource view to
// ~/.klens/config.yaml so the next launch reopens to the same scope. Called
// after every meaningful state change (mnemonic switch, drill-down, palette
// jump). Best-effort — write errors are swallowed.
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

// New constructs the root model from kubeconfig / config.yaml. Tolerates a
// missing cluster: returns a Model with `client == nil` and the warning is
// logged to stderr.
func New() (Model, error) {
	cfg, err := config.Load("")
	if err != nil {
		return Model{}, fmt.Errorf("load config: %w", err)
	}
	// Empty namespace = list across all namespaces (matches the design's
	// "ns:all" default). Users override per-cluster via ~/.klens/config.yaml.
	ns := cfg.Namespace

	client, clientErr := k8sclient.NewClient(cfg.Kubeconfig)
	if clientErr != nil {
		fmt.Fprintf(os.Stderr, "warn: no k8s cluster: %v\n", clientErr)
	}

	ti := textinput.New()
	ti.Placeholder = "filter… ns:platform status:Running"
	ti.Prompt = ""
	ti.CharLimit = 96

	var logTail func(ns string, pods []string, sinceSeconds int64)
	m := Model{
		client:      client,
		namespace:   ns,
		palette:     components.NewPalette(nil),
		filterInput: ti,
		logTailRef:  &logTail,
		cluster: ClusterInfo{
			KlensVer:   "0.3.0",
			K8sVersion: "—",
			Region:     "—",
			User:       "—",
		},
	}

	if client != nil {
		m.services = buildServices(client)
		m.pods = views.NewPodsView(m.services.Pods, ns)
		// Owner views (deployments, services, nodes) take a PodService alongside
		// their primary service so `l` can resolve matching pods for the
		// multi-pod log fan-out without leaking client-go into the views layer.
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
	}

	// Restore the last-opened view from config so the user re-enters
	// klens on the same screen they left.
	if v := paletteNameToView(cfg.LastView); cfg.LastView != "" {
		m.current = v
	}

	return m, nil
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

// PaletteVisible reports whether the command palette is open.
func (m Model) PaletteVisible() bool { return m.showPalette }

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

	case tea.KeyMsg:
		// `?` toggles the help overlay from any state where the filter is not
		// focused and the palette isn't already up. Defining this here (above
		// updateGlobal) means the toggle works regardless of the current view's
		// keymap and never collides with view-local keys.
		if !m.filterFocused && !m.showPalette && msg.String() == "?" {
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
		return m, nil
	}

	// A non-pod list view (today: PVCs) asked us to focus the GenericDescribe
	// shell with a snapshot of the row's KV preview as the body.
	if sw, ok := msg.(views.SwitchToGenericDescribeMsg); ok {
		m.history = append(m.history, m.current)
		m.genericDescribe = m.genericDescribe.WithFocus(sw.Title, sw.KVs)
		m.current = viewGenericDescribe
		return m, nil
	}

	// PodsView asked us to focus the describe view (Enter on a pod row).
	// WithFocus returns the cmd that fetches the pod's full spec async.
	if sw, ok := msg.(views.SwitchToDescribeMsg); ok {
		m.history = append(m.history, m.current)
		var fetch tea.Cmd
		m.describe, fetch = m.describe.WithFocus(sw.Namespace, sw.Pod)
		m.current = viewDescribe
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
		return m, nil
	}

	// k9s-style drill-down: a non-pod list view (deployments / services /
	// nodes / etc.) asked us to switch to pods filtered by some substring.
	// We push the current view onto the history stack so Esc returns the
	// user to where they came from.
	if d, ok := msg.(views.DrillToPodsMsg); ok {
		m.history = append(m.history, m.current)
		m.current = viewPods
		m.filterInput.SetValue(d.Filter)
		next, c := m.routeToCurrentView(views.FilterMsg{Query: d.Filter})
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
	// Mnemonic 1-8 — switch resource view (always works, even with the filter
	// focused, so power users aren't stuck in the input). Mnemonic switches
	// are intentional jumps, not drill-downs, so we clear the back-history.
	if mnemonic, ok := mnemonicToView(msg.String()); ok {
		m.current = mnemonic
		m.history = nil
		m.filterFocused = false
		m.filterInput.Blur()
		go m.persistState()
		return m, m.reloadCmd()
	}

	// `/` enters filter-focus mode (vim-style). Resets to a clean filter so
	// "/" never appears as part of the query.
	if msg.String() == "/" {
		m.filterFocused = true
		m.filterInput.SetValue("")
		m.filterInput.Focus()
		next, cmd := m.routeToCurrentView(views.FilterMsg{Query: ""})
		nm, _ := next.(Model)
		nm.filterInput = m.filterInput
		nm.filterFocused = true
		return nm, cmd
	}

	// `enter` while typing a filter commits it and exits input mode — the
	// filter stays applied, but j/k now navigate the filtered table again.
	if msg.String() == "enter" && m.filterFocused {
		m.filterFocused = false
		m.filterInput.Blur()
		return m, nil
	}

	// `esc` priorities (in order):
	//   1. exit filter-focus + clear the filter
	//   2. pop the navigation history (drill-back)
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
			m.filterInput.SetValue("")
			next, cmd := m.routeToCurrentView(views.FilterMsg{Query: ""})
			nm, _ := next.(Model)
			nm.filterInput = m.filterInput
			go nm.persistState()
			return nm, cmd
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

	// Default mode: global keys, then route to view. We bind only `:` to the
	// palette — `ctrl+k` was conflicting with terminal emulator shortcuts
	// (Warp uses it for its own palette).
	switch msg.String() {
	case ":":
		m.showPalette = true
		m.palette = components.NewPalette(nil)
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
			if cmd.Name == "quit" {
				return m, tea.Quit
			}
			m.current = paletteNameToView(cmd.Name)
			return m, m.reloadCmd()
		}
		return m, nil
	}
	var teaCmd tea.Cmd
	m.palette, teaCmd = m.palette.Update(msg)
	return m, teaCmd
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

	nav := layout.NavStrip(m.width, layout.NavStripConfig{
		Items:        m.navItems(),
		Current:      v.Title(),
		VisibleCount: visible,
		TotalCount:   total,
	})
	navDiv := lipgloss.NewStyle().
		Foreground(theme.ColorBorder).
		Render(strings.Repeat("─", m.width))
	nav = lipgloss.JoinVertical(lipgloss.Left, nav, navDiv)

	// Sub-views (logs, describe, genericDescribe) take the full content area —
	// hide the right details pane so their content isn't squashed.
	showDetails := m.width >= minDetailsAt &&
		m.current != viewLogs &&
		m.current != viewDescribe &&
		m.current != viewGenericDescribe

	contentH := m.height - topBarHeight - navStripHeight - cmdBarHeight - chipsHeight
	if contentH < 1 {
		contentH = 1
	}

	detW := 0
	if showDetails {
		detW = detailsWidth
	}
	midW := m.width - detW

	chips := layout.FilterChips(midW, v.Chips(), visible, total)
	tbl := v.Table(midW, contentH)

	center := lipgloss.JoinVertical(lipgloss.Left, chips, tbl)

	rows := []string{center}
	if showDetails {
		rows = append(rows, v.Details(detW, contentH+chipsHeight))
	}
	row := lipgloss.JoinHorizontal(lipgloss.Top, rows...)

	// Always advertise `?` in the bottom bar — done at the shell level rather
	// than per-view so adding a new view never requires remembering to add the
	// help hint, and so the position is stable across views.
	hints := append([]layout.KeyHint{}, v.KeyHints()...)
	hints = append(hints, layout.KeyHint{Key: "?", Label: "help"})
	cmd := layout.CommandBar(m.width, m.commandBarInput(), hints)

	frame := lipgloss.JoinVertical(lipgloss.Left, top, nav, row, cmd)

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

// commandBarInput returns the raw textinput.View() — layout.CommandBar prepends
// the "›" + "/" prompt so we don't add it here.
func (m Model) commandBarInput() string {
	return m.filterInput.View()
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

// navItems returns the nav-strip entries with their current totals.
// Each item's Count is the resource's total; the active item's filtered/
// total count is supplied separately via NavStripConfig.VisibleCount.
func (m Model) navItems() []layout.NavItem {
	_, pT := m.pods.Count()
	_, dT := m.deployments.Count()
	_, sT := m.services_.Count()
	_, secT := m.secrets.Count()
	_, cmT := m.configmaps.Count()
	_, nsT := m.namespaces.Count()
	_, noT := m.nodes.Count()
	_, pvT := m.pvcs.Count()
	return []layout.NavItem{
		{Key: "pods", Label: "Pods", Mnemonic: "1", Count: pT},
		{Key: "deployments", Label: "Deployments", Mnemonic: "2", Count: dT},
		{Key: "services", Label: "Services", Mnemonic: "3", Count: sT},
		{Key: "secrets", Label: "Secrets", Mnemonic: "4", Count: secT},
		{Key: "configmaps", Label: "ConfigMaps", Mnemonic: "5", Count: cmT},
		{Key: "namespaces", Label: "Namespaces", Mnemonic: "6", Count: nsT},
		{Key: "nodes", Label: "Nodes", Mnemonic: "7", Count: noT},
		{Key: "pvcs", Label: "PVCs", Mnemonic: "8", Count: pvT},
	}
}

// totals returns the legacy aggregate counter set. The new nav strip carries
// per-resource counts directly, so this is only kept to satisfy the existing
// TopBarConfig field — the top bar itself no longer renders it.
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

func mnemonicToView(s string) (viewKind, bool) {
	switch s {
	case "1":
		return viewPods, true
	case "2":
		return viewDeployments, true
	case "3":
		return viewServices, true
	case "4":
		return viewSecrets, true
	case "5":
		return viewConfigMaps, true
	case "6":
		return viewNamespaces, true
	case "7":
		return viewNodes, true
	case "8":
		return viewPVCs, true
	}
	return viewPods, false
}

func fallback(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

