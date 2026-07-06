package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/Omotolani98/mgr/internals/health"
	"github.com/Omotolani98/mgr/internals/inventory"
	"github.com/Omotolani98/mgr/internals/remoteops"
)

func TestServerNavigationBounds(t *testing.T) {
	m := testModel()

	m = updateModel(t, m, key("j"))
	m = updateModel(t, m, key("j"))
	if m.selected != 1 {
		t.Fatalf("selected = %d, want 1", m.selected)
	}

	m = updateModel(t, m, key("k"))
	m = updateModel(t, m, key("k"))
	if m.selected != 0 {
		t.Fatalf("selected = %d, want 0", m.selected)
	}
}

func TestFilterMatchesAndClears(t *testing.T) {
	m := testModel()

	m = updateModel(t, m, key("/"))
	for _, r := range "prod" {
		m = updateModel(t, m, key(string(r)))
	}
	if !m.filtering {
		t.Fatal("filtering = false, want true")
	}
	if m.filter != "prod" {
		t.Fatalf("filter = %q, want prod", m.filter)
	}
	if got, ok := m.currentServer(); !ok || got.Name != "prod-api" {
		t.Fatalf("currentServer = %#v, %v; want prod-api", got, ok)
	}

	m = updateModel(t, m, special(tea.KeyEscape))
	if m.filtering || m.filter != "" {
		t.Fatalf("filtering/filter = %v/%q, want false/empty", m.filtering, m.filter)
	}
}

func TestNoMatchRendering(t *testing.T) {
	m := testModel()
	m.filter = "missing"

	out := m.renderServers()
	if !strings.Contains(out, `No servers match "missing"`) {
		t.Fatalf("renderServers() = %q, want no-match message", out)
	}
}

func TestDetailToggleAndRender(t *testing.T) {
	m := testModel()

	m = updateModel(t, m, special(tea.KeyEnter))
	if !m.detail {
		t.Fatal("detail = false, want true")
	}
	out := m.renderServers()
	for _, want := range []string{"Details", "identity:", "created:"} {
		if !strings.Contains(out, want) {
			t.Fatalf("renderServers() missing %q: %q", want, out)
		}
	}

	m = updateModel(t, m, special(tea.KeyEscape))
	if m.detail {
		t.Fatal("detail = true, want false after esc")
	}
}

func TestHealthResultDisplay(t *testing.T) {
	m := testModel()
	m = updateModel(t, m, healthMsg(health.Status{
		Name:      "prod-api",
		Reachable: true,
		Latency:   12 * time.Millisecond,
		CheckedAt: time.Now().UTC(),
	}))

	out := m.renderServers()
	if !strings.Contains(out, "health: up 12ms") {
		t.Fatalf("renderServers() = %q, want health detail", out)
	}
	if !strings.Contains(m.status, "prod-api reachable") {
		t.Fatalf("status = %q, want reachable status", m.status)
	}
}

func TestEnvStatusDisplay(t *testing.T) {
	checkedAt := time.Date(2026, 7, 6, 18, 0, 0, 0, time.UTC)
	m := testModel()
	m.mode = "env"

	m = updateModel(t, m, envStatusMsg{count: 4, checkedAt: checkedAt})
	out := m.renderEnv()
	for _, want := range []string{"last_check: 2026-07-06T18:00:00Z", "secret_count: 4"} {
		if !strings.Contains(out, want) {
			t.Fatalf("renderEnv() missing %q: %q", want, out)
		}
	}
}

func TestOpsSnapshotDisplay(t *testing.T) {
	checkedAt := time.Date(2026, 7, 6, 18, 30, 0, 0, time.UTC)
	m := testModel()

	m = updateModel(t, m, opsMsg{
		server: "prod-api",
		snap: remoteops.Snapshot{
			Server:    "prod-api",
			Uptime:    "up 1 day",
			Disk:      "disk output",
			Memory:    "memory output",
			Processes: "process output",
			CheckedAt: checkedAt,
		},
	})

	out := m.renderServers()
	for _, want := range []string{"ops_checked: 2026-07-06T18:30:00Z", "== uptime ==", "disk output"} {
		if !strings.Contains(out, want) {
			t.Fatalf("renderServers() missing %q: %q", want, out)
		}
	}
}

func TestOpsErrorDisplay(t *testing.T) {
	m := testModel()
	m = updateModel(t, m, opsMsg{server: "prod-api", err: errTest("ssh failed")})
	if m.errMsg != "ssh failed" {
		t.Fatalf("errMsg = %q, want ssh failed", m.errMsg)
	}
}

func TestTrimOutput(t *testing.T) {
	got := trimOutput("abcdef", 3)
	if got != "abc\n..." {
		t.Fatalf("trimOutput = %q", got)
	}
}

func testModel() model {
	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)
	return model{
		width:  120,
		height: 40,
		mode:   "servers",
		servers: []inventory.Server{
			{
				Name:         "prod-api",
				Host:         "203.0.113.10",
				Port:         22,
				User:         "ubuntu",
				IdentityFile: "/tmp/prod_ed25519",
				Group:        "prod",
				Env:          "prod",
				Tags:         []string{"api", "prod"},
				CreatedAt:    now,
				UpdatedAt:    now,
			},
			{
				Name:      "dev-db",
				Host:      "127.0.0.1",
				Port:      2222,
				Group:     "dev",
				Env:       "dev",
				Tags:      []string{"db"},
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
		health: map[string]health.Status{},
		ops:    map[string]remoteops.Snapshot{},
	}
}

func updateModel(t *testing.T, m model, msg tea.Msg) model {
	t.Helper()
	updated, _ := m.Update(msg)
	next, ok := updated.(model)
	if !ok {
		t.Fatalf("updated model type = %T, want tui.model", updated)
	}
	return next
}

func key(text string) tea.KeyPressMsg {
	r := []rune(text)
	code := rune(0)
	if len(r) > 0 {
		code = r[0]
	}
	return tea.KeyPressMsg(tea.Key{Text: text, Code: code})
}

func special(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code})
}

type errTest string

func (e errTest) Error() string { return string(e) }
