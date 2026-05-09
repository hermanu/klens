// Package k8s contains the Kubernetes client and the informer-based watcher
// that forwards resource-change events to a tea.Program as typed tea.Msg
// values. Each supported resource type has its own *UpdatedMsg; views listen
// for their own message and re-fetch via their service. The actual resource
// service implementations live in the resources subpackage.
package k8s
