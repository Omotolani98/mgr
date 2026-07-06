package provider

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRegistryListAndGet(t *testing.T) {
	registry := DefaultRegistry()

	providers := registry.List()
	if len(providers) != 1 || providers[0].Name() != "ssh-config" {
		t.Fatalf("providers = %#v, want ssh-config", providers)
	}
	if _, err := registry.Get("ssh-config"); err != nil {
		t.Fatalf("get ssh-config: %v", err)
	}
	if _, err := registry.Get("missing"); !errors.Is(err, ErrProviderNotFound) {
		t.Fatalf("get missing err = %v, want ErrProviderNotFound", err)
	}
}

func TestSSHConfigProviderDiscover(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	data := []byte(`
Host prod-api
  HostName 203.0.113.10
  User ubuntu
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	provider := NewSSHConfigProvider(path)
	servers, err := provider.Discover(context.Background())
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(servers) != 1 {
		t.Fatalf("len = %d, want 1", len(servers))
	}
	server := servers[0]
	if server.Source != "ssh-config" {
		t.Fatalf("Source = %q, want ssh-config", server.Source)
	}
	if server.SourceID != "prod-api" {
		t.Fatalf("SourceID = %q, want prod-api", server.SourceID)
	}
	if server.LastSeenAt.IsZero() {
		t.Fatal("LastSeenAt is zero")
	}
}
