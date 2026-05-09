package app

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hermanu/klens/config"
	k8sclient "github.com/hermanu/klens/k8s"
	"github.com/hermanu/klens/k8s/resources"
	"github.com/hermanu/klens/port"
	"github.com/hermanu/klens/ui/components"
	"github.com/hermanu/klens/ui/views"
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
)

// Model is the root Bubble Tea model. It owns all views and routes messages.
type Model struct {
	client      *k8sclient.Client
	namespace   string
	current     viewKind
	pods        views.PodsView
	deployments views.DeploymentsView
	services    views.ServicesView
	secrets     views.SecretsView
	configmaps  views.ConfigMapsView
	namespaces  views.NamespacesView
	nodes       views.NodesView
	pvcs        views.PVCsView
	palette     components.Palette
	showPalette bool
	width       int
	height      int
}

// New builds the root model. Non-empty overrides take precedence over the
// config file: kubeconfigOverride replaces cfg.Kubeconfig, namespaceOverride
// replaces cfg.Namespace. Pass empty strings to fall back to the config.
func New(kubeconfigOverride, namespaceOverride string) (Model, error) {
	cfg, err := config.Load("")
	if err != nil {
		return Model{}, fmt.Errorf("load config: %w", err)
	}
	if kubeconfigOverride != "" {
		cfg.Kubeconfig = kubeconfigOverride
	}
	if namespaceOverride != "" {
		cfg.Namespace = namespaceOverride
	}
	ns := cfg.Namespace
	if ns == "" {
		ns = "default"
	}

	client, clientErr := k8sclient.NewClient(cfg.Kubeconfig)
	if clientErr != nil {
		fmt.Fprintf(os.Stderr, "warn: no k8s cluster: %v\n", clientErr)
	}

	m := Model{
		client:    client,
		namespace: ns,
		palette:   components.NewPalette(nil),
	}

	if client != nil {
		svcs := buildServices(client)
		m.pods = views.NewPodsView(svcs.Pods, ns)
		m.deployments = views.NewDeploymentsView(svcs.Deployments, ns)
		m.services = views.NewServicesView(svcs.Svcs, ns)
		m.secrets = views.NewSecretsView(svcs.Secrets, ns)
		m.configmaps = views.NewConfigMapsView(svcs.ConfigMaps, ns)
		m.namespaces = views.NewNamespacesView(svcs.Namespaces)
		m.nodes = views.NewNodesView(svcs.Nodes)
		m.pvcs = views.NewPVCsView(svcs.PVCs, ns)
	}

	return m, nil
}

func buildServices(client *k8sclient.Client) port.Services {
	return port.Services{
		Pods:        resources.NewPodSvc(client.Kube),
		Deployments: resources.NewDeploymentSvc(client.Kube),
		Svcs:        resources.NewServiceSvc(client.Kube),
		Secrets:     resources.NewSecretSvc(client.Kube),
		ConfigMaps:  resources.NewConfigMapSvc(client.Kube),
		Namespaces:  resources.NewNamespaceSvc(client.Kube),
		Nodes:       resources.NewNodeSvc(client.Kube),
		PVCs:        resources.NewPVCSvc(client.Kube),
	}
}

// Client returns the underlying k8s client (may be nil if no cluster is available).
func (m Model) Client() *k8sclient.Client { return m.client }

// Namespace returns the active namespace.
func (m Model) Namespace() string { return m.namespace }

// PaletteVisible reports whether the command palette overlay is open.
func (m Model) PaletteVisible() bool { return m.showPalette }

func (m Model) Init() tea.Cmd {
	return func() tea.Msg { return k8sclient.PodsUpdatedMsg{} }
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m.broadcastSize(msg)

	case tea.KeyMsg:
		if m.showPalette {
			return m.updatePalette(msg)
		}
		return m.updateGlobal(msg)
	}

	// Route watcher messages and resource-specific messages to the current view.
	return m.routeToCurrentView(msg)
}

func (m Model) updateGlobal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case ":":
		m.showPalette = true
		m.palette = components.NewPalette(nil)
		return m, nil
	case "q", "ctrl+c":
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

func (m Model) broadcastSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	var err error
	m.pods, _ = updateView(m.pods, msg)
	m.deployments, _ = updateView(m.deployments, msg)
	m.services, _ = updateView(m.services, msg)
	m.secrets, _ = updateView(m.secrets, msg)
	m.configmaps, _ = updateView(m.configmaps, msg)
	m.namespaces, _ = updateView(m.namespaces, msg)
	m.nodes, _ = updateView(m.nodes, msg)
	m.pvcs, _ = updateView(m.pvcs, msg)
	_ = err
	return m, nil
}

func (m Model) routeToCurrentView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.current {
	case viewPods:
		m.pods, cmd = m.pods.Update(msg)
	case viewDeployments:
		m.deployments, cmd = m.deployments.Update(msg)
	case viewServices:
		m.services, cmd = m.services.Update(msg)
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

func (m Model) View() string {
	if m.showPalette {
		return m.paletteOverlay()
	}
	switch m.current {
	case viewPods:
		return m.pods.View()
	case viewDeployments:
		return m.deployments.View()
	case viewServices:
		return m.services.View()
	case viewSecrets:
		return m.secrets.View()
	case viewConfigMaps:
		return m.configmaps.View()
	case viewNamespaces:
		return m.namespaces.View()
	case viewNodes:
		return m.nodes.View()
	case viewPVCs:
		return m.pvcs.View()
	}
	return m.pods.View()
}

func (m Model) paletteOverlay() string {
	return "\n\n" + m.palette.View(60)
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

// updateView is a generic helper that routes any tea.Msg to a view's Update method.
// Each view type must be listed explicitly because Go generics don't support method sets.
func updateView[V interface{ Update(tea.Msg) (V, tea.Cmd) }](v V, msg tea.Msg) (V, tea.Cmd) {
	return v.Update(msg)
}
