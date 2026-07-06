package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Omotolani98/mgr/internals/remoteops"
)

func TestProviderListCommand(t *testing.T) {
	cmd := newProviderListCmd()

	out, err := executeCommand(cmd)
	if err != nil {
		t.Fatalf("provider list: %v", err)
	}
	if !strings.Contains(out, "ssh-config") {
		t.Fatalf("output = %q, want ssh-config provider", out)
	}
}

func TestProviderSyncSSHConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	data := []byte(`
Host prod-api
  HostName 203.0.113.10
  User ubuntu
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write ssh config: %v", err)
	}
	app := testApp(t, &fakeOpsRunner{result: remoteops.Result{}})
	cmd := newProviderSyncCmd(app)

	out, err := executeCommand(cmd, "ssh-config", "--path", path)
	if err != nil {
		t.Fatalf("provider sync: %v", err)
	}
	if !strings.Contains(out, "synced 1 server(s) from ssh-config") {
		t.Fatalf("output = %q, want sync count", out)
	}
	server, err := app.store.Get("prod-api")
	if err != nil {
		t.Fatalf("get imported server: %v", err)
	}
	if server.Source != "ssh-config" || server.SourceID != "prod-api" || server.LastSeenAt.IsZero() {
		t.Fatalf("server source metadata = %#v", server)
	}
}

func TestServerListSourceFilter(t *testing.T) {
	app := testApp(t, &fakeOpsRunner{result: remoteops.Result{}})
	server, err := app.store.Get("local")
	if err != nil {
		t.Fatalf("get local: %v", err)
	}
	server.Source = "manual"
	if _, err := app.store.Upsert(server); err != nil {
		t.Fatalf("upsert local: %v", err)
	}
	cmd := newServerListCmd(app)

	out, err := executeCommand(cmd, "--source", "manual")
	if err != nil {
		t.Fatalf("server list: %v", err)
	}
	if !strings.Contains(out, "SOURCE") || !strings.Contains(out, "manual") {
		t.Fatalf("output = %q, want source column and manual server", out)
	}
}
