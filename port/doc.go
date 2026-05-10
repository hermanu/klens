// Package port defines the interfaces that the UI layer depends on for
// interacting with Kubernetes resources. Implementations live in k8s/resources;
// views in ui/views accept only the interface they need. Keeping this package
// free of client-go imports is what allows views to remain testable without a
// real cluster.
package port
