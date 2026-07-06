// Package remoteops runs read-only inspection commands on inventory servers.
package remoteops

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/Omotolani98/mgr/internals/inventory"
)

type Result struct {
	Command  string
	Stdout   string
	Stderr   string
	Duration time.Duration
}

type Runner interface {
	Run(context.Context, inventory.Server, string, ...string) (Result, error)
}

type SystemSSHRunner struct{}

type Snapshot struct {
	Server    string
	Uptime    string
	Disk      string
	Memory    string
	Processes string
	CheckedAt time.Time
	Duration  time.Duration
}

const (
	UptimeScript    = `uptime -p 2>/dev/null || uptime`
	DiskScript      = `df -hP /`
	MemoryScript    = `free -h`
	ProcessesScript = `ps -eo pid,comm,%cpu,%mem --sort=-%cpu | head -n 11`
	LogsScript      = `unit=$1; lines=$2; systemctl status --no-pager --lines=0 "$unit" 2>&1; journalctl -u "$unit" -n "$lines" --no-pager 2>&1`
	SnapshotScript  = `printf '== uptime ==\n'; uptime -p 2>/dev/null || uptime; printf '\n== disk ==\n'; df -hP /; printf '\n== memory ==\n'; free -h; printf '\n== processes ==\n'; ps -eo pid,comm,%cpu,%mem --sort=-%cpu | head -n 11`
)

func (r SystemSSHRunner) Run(ctx context.Context, server inventory.Server, command string, args ...string) (Result, error) {
	start := time.Now()
	sshArgs := BuildSSHArgs(server, command, args...)
	cmd := exec.CommandContext(ctx, "ssh", sshArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := Result{
		Command:  command,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: time.Since(start),
	}
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return result, errors.New("ssh not found in PATH")
		}
		if result.Stderr != "" {
			return result, fmt.Errorf("%w: %s", err, strings.TrimSpace(result.Stderr))
		}
		return result, err
	}
	return result, nil
}

func BuildSSHArgs(server inventory.Server, command string, args ...string) []string {
	sshArgs := []string{}
	if server.IdentityFile != "" {
		sshArgs = append(sshArgs, "-i", server.IdentityFile)
	}
	if server.Port != 0 {
		sshArgs = append(sshArgs, "-p", strconv.Itoa(server.Port))
	}
	target := server.Host
	if server.User != "" {
		target = server.User + "@" + server.Host
	}
	sshArgs = append(sshArgs, target, "sh", "-lc", command, "mgr-remote")
	sshArgs = append(sshArgs, args...)
	return sshArgs
}

func Uptime(ctx context.Context, runner Runner, server inventory.Server) (Result, error) {
	return runner.Run(ctx, server, UptimeScript)
}

func Disk(ctx context.Context, runner Runner, server inventory.Server) (Result, error) {
	return runner.Run(ctx, server, DiskScript)
}

func Memory(ctx context.Context, runner Runner, server inventory.Server) (Result, error) {
	return runner.Run(ctx, server, MemoryScript)
}

func Processes(ctx context.Context, runner Runner, server inventory.Server) (Result, error) {
	return runner.Run(ctx, server, ProcessesScript)
}

func Logs(ctx context.Context, runner Runner, server inventory.Server, unit string, lines int) (Result, error) {
	if unit == "" {
		return Result{}, errors.New("unit is required")
	}
	if lines <= 0 {
		lines = 100
	}
	return runner.Run(ctx, server, LogsScript, unit, strconv.Itoa(lines))
}

func DefaultSnapshot(ctx context.Context, runner Runner, server inventory.Server) (Snapshot, error) {
	start := time.Now()
	result, err := runner.Run(ctx, server, SnapshotScript)
	if err != nil {
		return Snapshot{}, err
	}
	sections := splitSections(result.Stdout)
	return Snapshot{
		Server:    server.Name,
		Uptime:    sections["uptime"],
		Disk:      sections["disk"],
		Memory:    sections["memory"],
		Processes: sections["processes"],
		CheckedAt: time.Now().UTC(),
		Duration:  time.Since(start),
	}, nil
}

func (s Snapshot) String() string {
	var b strings.Builder
	writeSection(&b, "uptime", s.Uptime)
	writeSection(&b, "disk", s.Disk)
	writeSection(&b, "memory", s.Memory)
	writeSection(&b, "processes", s.Processes)
	return strings.TrimRight(b.String(), "\n")
}

func splitSections(output string) map[string]string {
	sections := map[string]string{}
	current := ""
	var b strings.Builder
	flush := func() {
		if current == "" {
			return
		}
		sections[current] = strings.TrimSpace(b.String())
		b.Reset()
	}
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "== ") && strings.HasSuffix(line, " ==") {
			flush()
			current = strings.TrimSuffix(strings.TrimPrefix(line, "== "), " ==")
			continue
		}
		if current != "" {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	flush()
	return sections
}

func writeSection(b *strings.Builder, title, value string) {
	if value == "" {
		return
	}
	b.WriteString("== ")
	b.WriteString(title)
	b.WriteString(" ==\n")
	b.WriteString(strings.TrimSpace(value))
	b.WriteString("\n\n")
}
