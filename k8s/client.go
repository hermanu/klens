package k8s

import (
	"sort"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Client bundles the typed and dynamic Kubernetes API clients with the REST
// config used to build them.
type Client struct {
	Kube    kubernetes.Interface
	Dynamic dynamic.Interface
	Config  *rest.Config
}

// NewClient builds a Client from the default kubeconfig. kubeconfigPath overrides
// KUBECONFIG and the default discovery path; an empty string uses the default.
func NewClient(kubeconfigPath string) (*Client, error) {
	return NewClientForContext(kubeconfigPath, "")
}

// NewClientForContext is NewClient with an explicit context override. Empty
// `contextName` falls back to the kubeconfig's current-context (matching
// NewClient's behaviour). Used by the startup picker when current-context
// is missing or the user wants to switch clusters.
func NewClientForContext(kubeconfigPath, contextName string) (*Client, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfigPath != "" {
		loadingRules.ExplicitPath = kubeconfigPath
	}
	overrides := &clientcmd.ConfigOverrides{}
	if contextName != "" {
		overrides.CurrentContext = contextName
	}
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		overrides,
	).ClientConfig()
	if err != nil {
		return nil, err
	}
	kube, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &Client{Kube: kube, Dynamic: dyn, Config: cfg}, nil
}

// Contexts returns all available kubeconfig contexts (sorted) plus the
// current-context name. Returns an empty slice + empty current with no error
// if kubeconfig is loadable but has no contexts at all.
func Contexts() (contexts []string, current string, _ error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	raw, err := rules.Load()
	if err != nil {
		return nil, "", err
	}
	names := make([]string, 0, len(raw.Contexts))
	for name := range raw.Contexts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, raw.CurrentContext, nil
}

// ContextInfo bundles context + cluster + user names extracted from kubeconfig.
type ContextInfo struct {
	Context string
	Cluster string
	User    string
}

// CurrentContextInfo returns the cluster + user references for the current
// kubeconfig context. Used by the top bar.
func CurrentContextInfo() (ContextInfo, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	raw, err := rules.Load()
	if err != nil {
		return ContextInfo{}, err
	}
	info := ContextInfo{Context: raw.CurrentContext}
	if ctx, ok := raw.Contexts[raw.CurrentContext]; ok && ctx != nil {
		info.Cluster = ctx.Cluster
		info.User = ctx.AuthInfo
	}
	return info, nil
}
