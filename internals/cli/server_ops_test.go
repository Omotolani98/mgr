package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Omotolani98/mgr/internals/inventory"
	"github.com/Omotolani98/mgr/internals/remoteops"
	"github.com/spf13/cobra"
)

type fakeOpsRunner struct {
	command string
	args    []string
	result  remoteops.Result
	err     error
}

func (f *fakeOpsRunner) Run(ctx context.Context, server inventory.Server, command string, args ...string) (remoteops.Result, error) {
	f.command = command
	f.args = args
	return f.result, f.err
}

func TestServerUptimeCommand(t *testing.T) {
	runner := &fakeOpsRunner{result: remoteops.Result{Stdout: "up 1 day\n"}}
	app := testApp(t, runner)
	cmd := newServerUptimeCmd(app)

	out, err := executeCommand(cmd, "local")
	if err != nil {
		t.Fatalf("execute uptime: %v", err)
	}
	if strings.TrimSpace(out) != "up 1 day" {
		t.Fatalf("output = %q, want uptime", out)
	}
	if runner.command != remoteops.UptimeScript {
		t.Fatalf("command = %q, want UptimeScript", runner.command)
	}
}

func TestServerOpsCommand(t *testing.T) {
	runner := &fakeOpsRunner{result: remoteops.Result{Stdout: `== uptime ==
up 1 day
== disk ==
disk output
== memory ==
memory output
== processes ==
process output
`}}
	app := testApp(t, runner)
	cmd := newServerOpsCmd(app)

	out, err := executeCommand(cmd, "local")
	if err != nil {
		t.Fatalf("execute ops: %v", err)
	}
	for _, want := range []string{"== uptime ==", "disk output", "memory output", "process output"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q: %q", want, out)
		}
	}
}

func TestServerLogsCommandArgs(t *testing.T) {
	runner := &fakeOpsRunner{result: remoteops.Result{Stdout: "logs\n"}}
	app := testApp(t, runner)
	cmd := newServerLogsCmd(app)

	if _, err := executeCommand(cmd, "local", "--unit", "nginx.service", "--lines", "12"); err != nil {
		t.Fatalf("execute logs: %v", err)
	}
	if runner.command != remoteops.LogsScript {
		t.Fatalf("command = %q, want LogsScript", runner.command)
	}
	if got := strings.Join(runner.args, ","); got != "nginx.service,12" {
		t.Fatalf("args = %q, want unit and lines", got)
	}
}

func TestServerRemoteCommandMissingServer(t *testing.T) {
	app := testApp(t, &fakeOpsRunner{})
	cmd := newServerDiskCmd(app)

	if _, err := executeCommand(cmd, "missing"); err == nil {
		t.Fatal("expected missing server error")
	}
}

func testApp(t *testing.T, runner remoteops.Runner) *App {
	t.Helper()
	store := inventory.NewFileStore(filepath.Join(t.TempDir(), "inventory.yaml"))
	if _, err := store.Upsert(inventory.Server{Name: "local", Host: "127.0.0.1", Port: 22}); err != nil {
		t.Fatalf("upsert server: %v", err)
	}
	return &App{store: store, ops: runner}
}

func executeCommand(cmd *cobra.Command, args ...string) (string, error) {
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}
