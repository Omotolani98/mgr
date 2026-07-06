package sshconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestImportSSHConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	data := []byte(`
Host prod-api prod-api-alt
  HostName 203.0.113.10
  User ubuntu
  Port 2222
  IdentityFile ~/.ssh/prod_ed25519

Host *
  User ignored

Host !blocked
  HostName blocked.example.com
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	servers, err := Import(path)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if len(servers) != 2 {
		t.Fatalf("len = %d, want 2", len(servers))
	}
	for _, srv := range servers {
		if srv.Host != "203.0.113.10" {
			t.Fatalf("Host = %q, want 203.0.113.10", srv.Host)
		}
		if srv.Port != 2222 {
			t.Fatalf("Port = %d, want 2222", srv.Port)
		}
		if srv.User != "ubuntu" {
			t.Fatalf("User = %q, want ubuntu", srv.User)
		}
		if srv.IdentityFile == "~/.ssh/prod_ed25519" {
			t.Fatal("IdentityFile was not expanded")
		}
	}
}
