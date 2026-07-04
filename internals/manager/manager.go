// Package manager provides the resource manager implementation.
package manager

import (
	"fmt"

	"github.com/Omotolani98/mgr/internals/resource"
)

type Manager struct{}

func (m *Manager) Add(r resource.Resource) error {
	fmt.Printf("Added Kind: %s\n", r.GetKind())
	return nil
}

func (m *Manager) Get(kind, name string) (resource.Resource, error) {
	return nil, nil
}

func (m *Manager) List(kind string) []resource.Resource {
	return nil
}

func (m *Manager) Delete(kind, name string) error {
	return nil
}
