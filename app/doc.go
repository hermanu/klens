// Package app contains the root Bubble Tea model that owns every resource view
// and routes messages between them. It is the only place that wires concrete
// service implementations from k8s/resources into views via the port.Services
// bundle. View structs are value types and Update returns new values; the
// generic updateView helper exists because Go method sets cannot be expressed
// as a constraint.
package app
