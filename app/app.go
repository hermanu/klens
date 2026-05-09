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
	viewLogs     // dedicated full-screen log tail; entered via `l` on a pod
	viewDescribe // dedicated full-screen describe; entered via Enter on a pod
)

// Geometry — the modern shell's pane sizes. Adjusted dynamically based on
// terminal width: details drops first below 140 cols, then nav rail below 100.
const (
	navRailWidth  = 22
	detailsWidth  = 44
	topBarHeight  = 3 // 2-row content + 1 bottom border accounted via Panel
	cmdBarHeight  = 1
	chipsHeight   = 1
	minDetailsAt  = 140
	minNavAt      = 100
	cardLineCount = 1
)

// Model is the root Bubble Tea model. It owns all views, the input, the
// palette, and the cluster info shown in the top bar.
type Model struct {
	client       *k8sclient.Client
	services     port.Services
	namespace    string // active namespace filter ("" = all)
	cluster      ClusterInfo

	current     viewKind
	pods        views.PodsView
	deployments views.DeploymentsView
	services_   views.ServicesView
	secrets     views.SecretsView
	configmaps  views.ConfigMapsView
	namespaces  views.NamespacesView
	nodes       views.NodesView
	pvcs        views.PVCsView
	logs        views.LogsView
	describe    views.DescribeView

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
		m.deployments = views.NewDeploymentsView(m.services.Deployments, ns)
		m.services_ = views.NewServicesView(m.services.Svcs, ns)
		m.secrets = views.NewSecretsView(m.services.Secrets, ns)
		m.configmaps = views.NewConfigMapsView(m.services.ConfigMaps, ns)
		m.namespaces = views.NewNamespacesView(m.services.Namespaces)
		m.nodes = views.NewNodesView(m.services.Nodes)
		m.pvcs = views.NewPVCsView(m.services.PVCs, ns)
		m.logs = views.NewLogsView()
		m.describe = views.NewDescribeView(m.services.Pods)

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

	// PodsView asked us to focus the dedicated full-screen logs view.
	if sw, ok := msg.(views.SwitchToLogsMsg); ok {
		m.history = append(m.history, m.current)
		m.logs = m.logs.WithFocus(sw.Namespace, sw.Pod)
		m.current = viewLogs
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

// View composes the modern shell. The palette overlay replaces the central
// area when open (no real overlay support in lipgloss; this matches the
// behaviour klens already had).
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}
	v := m.currentView()

	// Top bar.
	visible, total := v.Count()
	top := layout.TopBar(m.width, layout.TopBarConfig{
		Context:      fallback(m.cluster.Context, "—"),
		Cluster:      fallback(m.cluster.Cluster, "—"),
		User:         fallback(m.cluster.User, "—"),
		K8sVersion:   fallback(m.cluster.K8sVersion, "—"),
		Region:       fallback(m.cluster.Region, "—"),
		KlensVer:     fallback(m.cluster.KlensVer, "dev"),
		Namespace:    "ns:" + fallback(m.namespace, "all"),
		Resource:     v.Title(),
		Live:         m.client != nil,
		VisibleCount: visible,
		TotalCount:   total,
		Totals:       m.totals(),
	})

	showNav := m.width >= minNavAt
	// Sub-views (logs, describe) take the full content area — hide the
	// right details pane so their content isn't squashed.
	showDetails := m.width >= minDetailsAt &&
		m.current != viewLogs && m.current != viewDescribe

	contentH := m.height - topBarHeight - cmdBarHeight - chipsHeight
	if contentH < 1 {
		contentH = 1
	}

	navW := 0
	detW := 0
	if showNav {
		navW = navRailWidth
	}
	if showDetails {
		detW = detailsWidth
	}
	midW := m.width - navW - detW

	chips := layout.FilterChips(midW, v.Chips(), visible, total)
	tableHeight := contentH
	if tableHeight < 1 {
		tableHeight = 1
	}
	tbl := v.Table(midW, tableHeight)

	center := lipgloss.JoinVertical(lipgloss.Left, chips, tbl)

	rows := []string{}
	if showNav {
		rows = append(rows, layout.NavRail(navW, contentH+chipsHeight, v.Title(), m.navItems(), m.clusterMeta()))
	}
	rows = append(rows, center)
	if showDetails {
		rows = append(rows, v.Details(detW, contentH+chipsHeight))
	}
	row := lipgloss.JoinHorizontal(lipgloss.Top, rows...)

	cmd := layout.CommandBar(m.width, m.commandBarInput(), v.KeyHints())

	frame := lipgloss.JoinVertical(lipgloss.Left, top, row, cmd)

	if m.showPalette {
		// Center the palette over a full-screen blank canvas. We deliberately
		// skip a real overlay (lipgloss has no native cell-coordinate
		// substitution) — the palette modal replaces the frame, matching
		// klens's pre-redesign behaviour. No explicit Background here so the
		// modal inherits the terminal's true black like the rest of the chrome.
		modal := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.ColorAccent).
			Padding(0, 1).
			Render(m.palette.View(60))
		return lipgloss.Place(m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			modal,
		)
	}
	return frame
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
	}
	return m.pods
}

// navItems returns the nav-rail entries with current counts.
func (m Model) navItems() []layout.NavItem {
	pV, pT := m.pods.Count()
	dV, dT := m.deployments.Count()
	sV, sT := m.services_.Count()
	secV, secT := m.secrets.Count()
	cmV, cmT := m.configmaps.Count()
	nsV, nsT := m.namespaces.Count()
	noV, noT := m.nodes.Count()
	pvV, pvT := m.pvcs.Count()
	pick := func(filtered, total int) int {
		if filtered != total {
			return filtered
		}
		return total
	}
	return []layout.NavItem{
		{Key: "pods", Label: "Pods", Mnemonic: "1", Count: pick(pV, pT)},
		{Key: "deployments", Label: "Deployments", Mnemonic: "2", Count: pick(dV, dT)},
		{Key: "services", Label: "Services", Mnemonic: "3", Count: pick(sV, sT)},
		{Key: "secrets", Label: "Secrets", Mnemonic: "4", Count: pick(secV, secT)},
		{Key: "configmaps", Label: "ConfigMaps", Mnemonic: "5", Count: pick(cmV, cmT)},
		{Key: "namespaces", Label: "Namespaces", Mnemonic: "6", Count: pick(nsV, nsT)},
		{Key: "nodes", Label: "Nodes", Mnemonic: "7", Count: pick(noV, noT)},
		{Key: "pvcs", Label: "PVCs", Mnemonic: "8", Count: pick(pvV, pvT)},
	}
}

// totals renders the design's right-aligned counter strip in the top bar.
func (m Model) totals() layout.Totals {
	_, p := m.pods.Count()
	_, d := m.deployments.Count()
	_, s := m.services_.Count()
	return layout.Totals{Pods: p, Deployments: d, Services: s}
}

// clusterMeta gathers the bottom-of-rail block. Counts come from views; CPU /
// MEM percentages would need metrics-server aggregation across nodes — left
// at zero (renders as "—" in the rail) until that's wired through.
func (m Model) clusterMeta() layout.ClusterMeta {
	_, total := m.pods.Count()
	_, nodes := m.nodes.Count()
	return layout.ClusterMeta{
		NodesReady: nodes,
		NodesTotal: nodes,
		Pods:       total,
		PodsCap:    total, // we don't track allocatable pods yet
	}
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

