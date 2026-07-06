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
	deps           Deps
	width          int
	height         int
	mode           string
	servers        []inventory.Server
	selected       int
	filtering      bool
	filter         string
	detail         bool
	status         string
	errMsg         string
	health         map[string]health.Status
	envSecretCount int
	envCheckedAt   time.Time
	sshTarget      *inventory.Server
}

type serversMsg []inventory.Server
type healthMsg health.Status
type envStatusMsg struct {
	count     int
	checkedAt time.Time
	err       error
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
		if m.filtering {
			return m.updateFilter(msg)
		}
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc":
			if m.detail {
				m.detail = false
			}
		case "tab":
			if m.mode == "servers" {
				m.mode = "env"
			} else {
				m.mode = "servers"
			}
			m.detail = false
			m.filtering = false
		case "/":
			if m.mode == "servers" {
				m.filtering = true
				m.detail = false
			}
		case "r":
			m.status = "reloading servers"
			return m, loadServers(m.deps.Store)
		case "down", "j":
			if m.mode == "servers" && m.selected < len(m.filteredServers())-1 {
				m.selected++
			}
		case "up", "k":
			if m.mode == "servers" && m.selected > 0 {
				m.selected--
			}
		case "enter":
			if m.mode == "servers" {
				if _, ok := m.currentServer(); ok {
					m.detail = !m.detail
				}
			}
		case "c":
			if m.mode == "servers" {
				if srv, ok := m.currentServer(); ok {
					m.status = "checking " + srv.Name
					m.errMsg = ""
					return m, checkServer(srv)
				}
			} else {
				m.status = "checking foostash"
				m.errMsg = ""
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
		m.clampSelected()
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
			m.envSecretCount = msg.count
			m.envCheckedAt = msg.checkedAt
			m.status = fmt.Sprintf("foostash ok, %d secret(s)", msg.count)
			m.errMsg = ""
		}
	case errMsg:
		m.errMsg = msg.err.Error()
		m.status = ""
	}
	return m, nil
}

func (m model) updateFilter(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.filtering = false
		m.filter = ""
	case "enter":
		m.filtering = false
	case "backspace":
		if len(m.filter) > 0 {
			rs := []rune(m.filter)
			m.filter = string(rs[:len(rs)-1])
		}
	default:
		if text := msg.Key().Text; text != "" {
			m.filter += text
		}
	}
	m.clampSelected()
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
	b.WriteString(mutedStyle.Render(m.helpText()))
	return b.String()
}

func (m model) renderServers() string {
	servers := m.filteredServers()
	if len(m.servers) == 0 {
		return mutedStyle.Render("No servers yet. Add one with `mgr server add NAME --host HOST`.")
	}
	if len(servers) == 0 {
		return warnStyle.Render(fmt.Sprintf("No servers match %q.", m.filter))
	}
	list := m.renderServerList(servers)
	detail := m.renderServerDetail()
	if m.width >= 100 && !m.detail {
		return joinColumns(list, detail, m.width)
	}
	if m.detail {
		return detail
	}
	return list + "\n\n" + detail
}

func (m model) renderServerList(servers []inventory.Server) string {
	var b strings.Builder
	if m.filtering {
		b.WriteString(selectedStyle.Render("filter: " + m.filter))
		b.WriteString("\n\n")
	} else if m.filter != "" {
		b.WriteString(mutedStyle.Render("filter: " + m.filter))
		b.WriteString("\n\n")
	}
	for i, srv := range servers {
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
	return strings.TrimRight(b.String(), "\n")
}

func (m model) renderServerDetail() string {
	if srv, ok := m.currentServer(); ok {
		var b strings.Builder
		b.WriteString(titleStyle.Render("Details"))
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("name: %s\n", srv.Name))
		b.WriteString(fmt.Sprintf("target: %s\n", endpoint(srv)))
		b.WriteString(fmt.Sprintf("identity: %s\n", empty(srv.IdentityFile)))
		b.WriteString(fmt.Sprintf("group: %s\n", empty(srv.Group)))
		b.WriteString(fmt.Sprintf("env: %s\n", empty(srv.Env)))
		b.WriteString(fmt.Sprintf("tags: %s\n", empty(strings.Join(srv.Tags, ","))))
		if !srv.CreatedAt.IsZero() {
			b.WriteString(fmt.Sprintf("created: %s\n", srv.CreatedAt.Format(time.RFC3339)))
		}
		if !srv.UpdatedAt.IsZero() {
			b.WriteString(fmt.Sprintf("updated: %s\n", srv.UpdatedAt.Format(time.RFC3339)))
		}
		if status, ok := m.health[srv.Name]; ok {
			state := "down"
			if status.Reachable {
				state = "up"
			}
			b.WriteString(fmt.Sprintf("health: %s %s", state, status.Latency.Round(time.Millisecond)))
			if status.Error != "" {
				b.WriteString(" " + status.Error)
			}
			b.WriteByte('\n')
		} else {
			b.WriteString("health: unchecked\n")
		}
		return strings.TrimRight(b.String(), "\n")
	}
	return ""
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
	if !m.envCheckedAt.IsZero() {
		b.WriteString(fmt.Sprintf("last_check: %s\n", m.envCheckedAt.Format(time.RFC3339)))
		b.WriteString(fmt.Sprintf("secret_count: %d\n", m.envSecretCount))
	}
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("Press c to verify access. Secret values stay hidden."))
	return b.String()
}

func (m model) modeLabel() string {
	if m.mode == "env" {
		return "Foostash environment"
	}
	return "Servers"
}

func (m model) currentServer() (inventory.Server, bool) {
	servers := m.filteredServers()
	if len(servers) == 0 || m.selected < 0 || m.selected >= len(servers) {
		return inventory.Server{}, false
	}
	return servers[m.selected], true
}

func (m *model) clampSelected() {
	servers := m.filteredServers()
	if len(servers) == 0 {
		m.selected = 0
		return
	}
	if m.selected >= len(servers) {
		m.selected = len(servers) - 1
	}
	if m.selected < 0 {
		m.selected = 0
	}
}

func (m model) filteredServers() []inventory.Server {
	query := strings.ToLower(strings.TrimSpace(m.filter))
	if query == "" {
		return m.servers
	}
	out := make([]inventory.Server, 0, len(m.servers))
	for _, srv := range m.servers {
		haystack := strings.ToLower(strings.Join([]string{
			srv.Name,
			srv.Host,
			srv.User,
			srv.Group,
			srv.Env,
			strings.Join(srv.Tags, " "),
		}, " "))
		if strings.Contains(haystack, query) {
			out = append(out, srv)
		}
	}
	return out
}

func (m model) helpText() string {
	if m.filtering {
		return "type filter • enter apply • esc clear • ctrl+c quit"
	}
	if m.mode == "env" {
		return "tab servers • c check foostash • q quit"
	}
	if m.detail {
		return "enter list • esc list • c check • s ssh • q quit"
	}
	return "tab env • / filter • j/k move • enter detail • c check • s ssh • r reload • q quit"
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
		return envStatusMsg{count: count, checkedAt: time.Now().UTC(), err: err}
	}
}

func joinColumns(left, right string, width int) string {
	gap := "  "
	leftWidth := width/2 - len(gap)
	if leftWidth < 40 {
		leftWidth = 40
	}
	leftLines := strings.Split(left, "\n")
	rightLines := strings.Split(right, "\n")
	maxLines := len(leftLines)
	if len(rightLines) > maxLines {
		maxLines = len(rightLines)
	}
	var b strings.Builder
	for i := 0; i < maxLines; i++ {
		l := ""
		if i < len(leftLines) {
			l = leftLines[i]
		}
		r := ""
		if i < len(rightLines) {
			r = rightLines[i]
		}
		b.WriteString(padRight(l, leftWidth))
		b.WriteString(gap)
		b.WriteString(r)
		if i < maxLines-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func padRight(value string, width int) string {
	if len(value) >= width {
		return value
	}
	return value + strings.Repeat(" ", width-len(value))
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
