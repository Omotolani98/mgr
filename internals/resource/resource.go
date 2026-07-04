// Package resource provides the resource interface.
package resource

type Resource interface {
	GetName() string
	GetKind() string
}
