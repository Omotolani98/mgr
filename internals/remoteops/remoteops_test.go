package remoteops

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/Omotolani98/mgr/internals/inventory"
)

type fakeRunner struct {
	command string
	args    []string
	result  Result
	err     error
}

func (f *fakeRunner) Run(ctx context.Context, server inventory.Server, command string, args ...string) (Result, error) {
	f.command = command
	f.args = args
	return f.result, f.err
}

func TestBuildSSHArgs(t *testing.T) {
	server := inventory.Server{
		Host:         "203.0.113.10",
		Port:         2222,
		User:         "ubuntu",
		IdentityFile: "/tmp/key",
	}
	got := BuildSSHArgs(server, "uptime -p", "extra")
	want := []string{"-i", "/tmp/key", "-p", "2222", "ubuntu@203.0.113.10", "sh", "-lc", "uptime -p", "mgr-remote", "extra"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildSSHArgs() = %#v, want %#v", got, want)
	}
}

func TestLogsUsesUnitAndLinesAsRemoteArgs(t *testing.T) {
	runner := &fakeRunner{}
	server := inventory.Server{Name: "api", Host: "127.0.0.1"}

	if _, err := Logs(context.Background(), runner, server, "nginx.service", 25); err != nil {
		t.Fatalf("Logs: %v", err)
	}
	if runner.command != LogsScript {
		t.Fatalf("command = %q, want LogsScript", runner.command)
	}
	wantArgs := []string{"nginx.service", "25"}
	if !reflect.DeepEqual(runner.args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", runner.args, wantArgs)
	}
}

func TestDefaultSnapshotParsesSections(t *testing.T) {
	runner := &fakeRunner{result: Result{Stdout: `== uptime ==
up 2 days

== disk ==
Filesystem Size Used Avail Use% Mounted on
/dev/xvda1 50G 10G 40G 20% /

== memory ==
Mem: 1Gi 512Mi

== processes ==
PID COMMAND %CPU %MEM
1 systemd 0.1 0.2
`}}
	server := inventory.Server{Name: "api", Host: "127.0.0.1"}

	snap, err := DefaultSnapshot(context.Background(), runner, server)
	if err != nil {
		t.Fatalf("DefaultSnapshot: %v", err)
	}
	if snap.Server != "api" {
		t.Fatalf("Server = %q, want api", snap.Server)
	}
	if snap.Uptime != "up 2 days" {
		t.Fatalf("Uptime = %q", snap.Uptime)
	}
	if snap.Disk == "" || snap.Memory == "" || snap.Processes == "" {
		t.Fatalf("snapshot has empty section: %#v", snap)
	}
	if got := snap.String(); got == "" || !strings.Contains(got, "== uptime ==") {
		t.Fatalf("Snapshot.String() = %q", got)
	}
}
