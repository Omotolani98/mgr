// Package provider defines inventory discovery sources.
package provider

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/Omotolani98/mgr/internals/inventory"
	"github.com/Omotolani98/mgr/internals/sshconfig"
)

type ServerProvider interface {
	Name() string
	Description() string
	Discover(context.Context) ([]inventory.Server, error)
}

type Registry struct {
	providers map[string]ServerProvider
}

func NewRegistry(providers ...ServerProvider) *Registry {
	r := &Registry{providers: map[string]ServerProvider{}}
	for _, provider := range providers {
		r.Register(provider)
	}
	return r
}

func DefaultRegistry() *Registry {
	return NewRegistry(NewSSHConfigProvider(""))
}

func (r *Registry) Register(provider ServerProvider) {
	if provider == nil {
		return
	}
	r.providers[provider.Name()] = provider
}

func (r *Registry) Get(name string) (ServerProvider, error) {
	provider, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrProviderNotFound, name)
	}
	return provider, nil
}

func (r *Registry) List() []ServerProvider {
	out := make([]ServerProvider, 0, len(r.providers))
	for _, provider := range r.providers {
		out = append(out, provider)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name() < out[j].Name()
	})
	return out
}

var ErrProviderNotFound = errors.New("provider not found")

type SSHConfigProvider struct {
	path string
}

func NewSSHConfigProvider(path string) *SSHConfigProvider {
	return &SSHConfigProvider{path: path}
}

func (p *SSHConfigProvider) Name() string {
	return "ssh-config"
}

func (p *SSHConfigProvider) Description() string {
	return "OpenSSH config Host entries"
}

func (p *SSHConfigProvider) Discover(ctx context.Context) ([]inventory.Server, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	servers, err := sshconfig.Import(p.path)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	for i := range servers {
		servers[i].Source = p.Name()
		if servers[i].SourceID == "" {
			servers[i].SourceID = servers[i].Name
		}
		servers[i].LastSeenAt = now
	}
	return servers, nil
}
