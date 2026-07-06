// Package tui implements mgr's Bubble Tea terminal UI.
package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/Omotolani98/mgr/internals/config"
	mgrenv "github.com/Omotolani98/mgr/internals/env"
	"github.com/Omotolani98/mgr/internals/health"
	"github.com/Omotolani98/mgr/internals/inventory"
)

type Deps struct {
	Store  *inventory.FileStore
	Config config.Config
}

type model struct {
	deps      Deps
	width     int
	height    int
	mode      string
	servers   []inventory.Server
	selected  int
	status    string
	errMsg    string
	health    map[string]health.Status
	sshTarget *inventory.Server
}

type serversMsg []inventory.Server
type healthMsg health.Status
type envStatusMsg struct {
	count int
	err   error
}
type errMsg struct{ err error }

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	mutedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	okStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	warnStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
)

func Run(deps Deps) error {
	for {
		initial := newModel(deps)
		final, err := tea.NewProgram(initial).Run()
		if err != nil {
			return err
		}
		m, ok := final.(model)
		if !ok || m.sshTarget == nil {
			return nil
		}
		if err := runSSH(*m.sshTarget); err != nil {
			fmt.Fprintf(os.Stderr, "ssh: %v\n", err)
		}
	}
}

func newModel(deps Deps) model {
	return model{
		deps:   deps,
		mode:   "servers",
		health: map[string]health.Status{},
	}
}

func (m model) Init() tea.Cmd {
	return loadServers(m.deps.Store)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "tab":
			if m.mode == "servers" {
				m.mode = "env"
			} else {
				m.mode = "servers"
			}
		case "r":
			m.status = "reloading servers"
			return m, loadServers(m.deps.Store)
		case "down", "j":
			if m.mode == "servers" && m.selected < len(m.servers)-1 {
				m.selected++
			}
		case "up", "k":
			if m.mode == "servers" && m.selected > 0 {
				m.selected--
			}
		case "c":
			if m.mode == "servers" {
				if srv, ok := m.currentServer(); ok {
					m.status = "checking " + srv.Name
					return m, checkServer(srv)
				}
			} else {
				m.status = "checking foostash"
				return m, checkEnv(m.deps.Config.Foostash)
			}
		case "s":
			if m.mode == "servers" {
				if srv, ok := m.currentServer(); ok {
					m.sshTarget = &srv
					return m, tea.Quit
				}
			}
		}
	case serversMsg:
		m.servers = []inventory.Server(msg)
		if m.selected >= len(m.servers) {
			m.selected = 0
		}
		m.status = fmt.Sprintf("loaded %d server(s)", len(m.servers))
		m.errMsg = ""
	case healthMsg:
		status := health.Status(msg)
		m.health[status.Name] = status
		if status.Reachable {
			m.status = fmt.Sprintf("%s reachable in %s", status.Name, status.Latency.Round(time.Millisecond))
		} else {
			m.status = fmt.Sprintf("%s unreachable", status.Name)
		}
		m.errMsg = ""
	case envStatusMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.status = ""
		} else {
			m.status = fmt.Sprintf("foostash ok, %d secret(s)", msg.count)
			m.errMsg = ""
		}
	case errMsg:
		m.errMsg = msg.err.Error()
		m.status = ""
	}
	return m, nil
}

func (m model) View() tea.View {
	content := m.render()
	v := tea.NewView(content)
	v.AltScreen = true
	v.WindowTitle = "mgr"
	return v
}

func (m model) render() string {
	if m.width == 0 {
		return "Initializing..."
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render("mgr"))
	b.WriteString("  ")
	b.WriteString(mutedStyle.Render(m.modeLabel()))
	b.WriteString("\n\n")
	if m.mode == "env" {
		b.WriteString(m.renderEnv())
	} else {
		b.WriteString(m.renderServers())
	}
	b.WriteString("\n")
	if m.errMsg != "" {
		b.WriteString(errorStyle.Render(m.errMsg))
		b.WriteString("\n")
	} else if m.status != "" {
		b.WriteString(mutedStyle.Render(m.status))
		b.WriteString("\n")
	}
	b.WriteString(mutedStyle.Render("tab switch • j/k move • c check • s ssh • r reload • q quit"))
	return b.String()
}

func (m model) renderServers() string {
	if len(m.servers) == 0 {
		return mutedStyle.Render("No servers yet. Add one with `mgr server add NAME --host HOST`.")
	}
	var b strings.Builder
	for i, srv := range m.servers {
		line := fmt.Sprintf("%-22s %-28s %s", srv.Name, endpoint(srv), strings.Join(srv.Tags, ","))
		if status, ok := m.health[srv.Name]; ok {
			if status.Reachable {
				line += "  " + okStyle.Render("up")
			} else {
				line += "  " + warnStyle.Render("down")
			}
		}
		if i == m.selected {
			b.WriteString(selectedStyle.Render("> " + line))
		} else {
			b.WriteString("  " + line)
		}
		b.WriteByte('\n')
	}
	if srv, ok := m.currentServer(); ok {
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render(fmt.Sprintf("selected: user=%s group=%s env=%s identity=%s",
			empty(srv.User), empty(srv.Group), empty(srv.Env), empty(srv.IdentityFile))))
	}
	return b.String()
}

func (m model) renderEnv() string {
	cfg := m.deps.Config.Foostash
	var b strings.Builder
	b.WriteString("Foostash SDK\n\n")
	b.WriteString(fmt.Sprintf("server: %s\n", empty(cfg.ServerURL)))
	b.WriteString(fmt.Sprintf("project: %s\n", empty(cfg.Project)))
	b.WriteString(fmt.Sprintf("env: %s\n", empty(cfg.Env)))
	b.WriteString(fmt.Sprintf("ssh_key: %s\n", empty(cfg.SSHKeyPath)))
	if cfg.MasterKey != "" {
		b.WriteString("master_key: configured inline\n")
	} else {
		envName := cfg.MasterKeyEnv
		if envName == "" {
			envName = "FOOSTASH_MASTER_KEY"
		}
		b.WriteString(fmt.Sprintf("master_key_env: %s\n", envName))
	}
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("Press c to verify access with the Foostash SDK."))
	return b.String()
}

func (m model) modeLabel() string {
	if m.mode == "env" {
		return "Foostash environment"
	}
	return "Servers"
}

func (m model) currentServer() (inventory.Server, bool) {
	if len(m.servers) == 0 || m.selected < 0 || m.selected >= len(m.servers) {
		return inventory.Server{}, false
	}
	return m.servers[m.selected], true
}

func loadServers(store *inventory.FileStore) tea.Cmd {
	return func() tea.Msg {
		servers, err := store.List(inventory.Filter{})
		if err != nil {
			return errMsg{err: err}
		}
		return serversMsg(servers)
	}
}

func checkServer(srv inventory.Server) tea.Cmd {
	return func() tea.Msg {
		return healthMsg(health.Check(srv, 3*time.Second))
	}
}

func checkEnv(cfg config.FoostashConfig) tea.Cmd {
	return func() tea.Msg {
		provider, err := mgrenv.NewFoostashProvider(mgrenv.OptionsFromConfig(cfg))
		if err != nil {
			return envStatusMsg{err: err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		count, err := mgrenv.Status(ctx, provider)
		return envStatusMsg{count: count, err: err}
	}
}

func endpoint(srv inventory.Server) string {
	userHost := srv.Host
	if srv.User != "" {
		userHost = srv.User + "@" + srv.Host
	}
	return fmt.Sprintf("%s:%d", userHost, srv.Port)
}

func empty(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

func runSSH(srv inventory.Server) error {
	args := []string{}
	if srv.IdentityFile != "" {
		args = append(args, "-i", srv.IdentityFile)
	}
	if srv.Port != 0 {
		args = append(args, "-p", strconv.Itoa(srv.Port))
	}
	target := srv.Host
	if srv.User != "" {
		target = srv.User + "@" + srv.Host
	}
	args = append(args, target)

	c := exec.Command("ssh", args...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return errors.New("ssh not found in PATH")
		}
		return err
	}
	return nil
}
