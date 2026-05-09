package k8s

import (
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Client struct {
	Kube    kubernetes.Interface
	Dynamic dynamic.Interface
	Config  *rest.Config
}

func NewClient(kubeconfigPath string) (*Client, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfigPath != "" {
		loadingRules.ExplicitPath = kubeconfigPath
	}
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		&clientcmd.ConfigOverrides{},
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

// Contexts returns all available kubeconfig contexts.
func Contexts() ([]string, string, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	raw, err := rules.Load()
	if err != nil {
		return nil, "", err
	}
	var names []string
	for name := range raw.Contexts {
		names = append(names, name)
	}
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
