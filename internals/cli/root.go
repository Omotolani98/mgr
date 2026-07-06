// Package cli defines the mgr command tree.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/Omotolani98/mgr/internals/config"
	mgrenv "github.com/Omotolani98/mgr/internals/env"
	"github.com/Omotolani98/mgr/internals/health"
	"github.com/Omotolani98/mgr/internals/inventory"
	"github.com/Omotolani98/mgr/internals/remoteops"
	"github.com/Omotolani98/mgr/internals/sshconfig"
	"github.com/Omotolani98/mgr/internals/tui"
	"github.com/spf13/cobra"
)

type App struct {
	paths config.Paths
	cfg   config.Config
	store *inventory.FileStore
	ops   remoteops.Runner
}

type envFlags struct {
	serverURL    string
	project      string
	env          string
	sshKeyPath   string
	masterKey    string
	masterKeyEnv string
	timeout      time.Duration
}

func NewRootCmd() *cobra.Command {
	app := &App{}
	root := &cobra.Command{
		Use:           "mgr",
		Short:         "Manage SSH-backed server inventory and environments",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Name() == "help" {
				return nil
			}
			return app.init()
		},
	}

	root.AddCommand(
		newServerCmd(app),
		newEnvCmd(app),
		newTUICmd(app),
	)
	return root
}

func Execute() error {
	return NewRootCmd().Execute()
}

func (a *App) init() error {
	if a.store != nil {
		return nil
	}
	paths, err := config.DefaultPaths()
	if err != nil {
		return err
	}
	cfg, err := config.Load(paths.ConfigPath)
	if err != nil {
		return err
	}
	a.paths = paths
	a.cfg = cfg
	a.store = inventory.NewFileStore(paths.InventoryPath)
	if a.ops == nil {
		a.ops = remoteops.SystemSSHRunner{}
	}
	config.InitConfig()
	return nil
}

func newServerCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Manage local SSH server inventory",
	}
	cmd.AddCommand(
		newServerAddCmd(app),
		newServerListCmd(app),
		newServerGetCmd(app),
		newServerRemoveCmd(app),
		newServerCheckCmd(app),
		newServerSSHCmd(app),
		newServerImportCmd(app),
		newServerOpsCmd(app),
		newServerUptimeCmd(app),
		newServerDiskCmd(app),
		newServerMemoryCmd(app),
		newServerProcessesCmd(app),
		newServerLogsCmd(app),
	)
	return cmd
}

func newServerAddCmd(app *App) *cobra.Command {
	var srv inventory.Server
	var tags []string
	cmd := &cobra.Command{
		Use:   "add NAME",
		Short: "Add or update a server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			srv.Name = args[0]
			srv.Tags = tags
			saved, err := app.store.Upsert(srv)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "saved %s (%s:%d)\n", saved.Name, saved.Host, saved.Port)
			return nil
		},
	}
	cmd.Flags().StringVar(&srv.Host, "host", "", "Server hostname or IP address")
	cmd.Flags().IntVar(&srv.Port, "port", 22, "SSH port")
	cmd.Flags().StringVar(&srv.User, "user", "", "SSH user")
	cmd.Flags().StringVar(&srv.IdentityFile, "identity-file", "", "SSH identity file")
	cmd.Flags().StringArrayVar(&tags, "tag", nil, "Tag; may be repeated")
	cmd.Flags().StringVar(&srv.Group, "group", "", "Server group")
	cmd.Flags().StringVar(&srv.Env, "env", "", "Default environment name")
	_ = cmd.MarkFlagRequired("host")
	return cmd
}

func newServerListCmd(app *App) *cobra.Command {
	var filter inventory.Filter
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List servers",
		RunE: func(cmd *cobra.Command, args []string) error {
			servers, err := app.store.List(filter)
			if err != nil {
				return err
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tHOST\tPORT\tUSER\tGROUP\tENV\tTAGS")
			for _, srv := range servers {
				fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\t%s\t%s\n",
					srv.Name, srv.Host, srv.Port, srv.User, srv.Group, srv.Env, strings.Join(srv.Tags, ","))
			}
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&filter.Group, "group", "", "Filter by group")
	cmd.Flags().StringVar(&filter.Tag, "tag", "", "Filter by tag")
	return cmd
}

func newServerGetCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "get NAME",
		Short: "Show a server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			srv, err := app.store.Get(args[0])
			if err != nil {
				return err
			}
			printServer(cmd.OutOrStdout(), srv)
			return nil
		},
	}
}

func newServerRemoveCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:     "remove NAME",
		Aliases: []string{"rm", "delete"},
		Short:   "Remove a server",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.store.Remove(args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed %s\n", args[0])
			return nil
		},
	}
}

func newServerCheckCmd(app *App) *cobra.Command {
	var timeout time.Duration
	cmd := &cobra.Command{
		Use:   "check NAME",
		Short: "Check TCP reachability for a server's SSH endpoint",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			srv, err := app.store.Get(args[0])
			if err != nil {
				return err
			}
			status := health.Check(srv, timeout)
			out := cmd.OutOrStdout()
			if status.Reachable {
				fmt.Fprintf(out, "%s reachable at %s in %s\n", status.Name, status.Address, status.Latency.Round(time.Millisecond))
				return nil
			}
			fmt.Fprintf(out, "%s unreachable at %s: %s\n", status.Name, status.Address, status.Error)
			return nil
		},
	}
	cmd.Flags().DurationVar(&timeout, "timeout", 3*time.Second, "Connection timeout")
	return cmd
}

func newServerSSHCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "ssh NAME",
		Short: "Open an SSH session to a server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			srv, err := app.store.Get(args[0])
			if err != nil {
				return err
			}
			return runSSH(cmd.Context(), srv, os.Stdin, os.Stdout, os.Stderr)
		},
	}
}

func newServerImportCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import servers from external sources",
	}
	var path string
	sshConfigCmd := &cobra.Command{
		Use:   "ssh-config",
		Short: "Import Host entries from OpenSSH config",
		RunE: func(cmd *cobra.Command, args []string) error {
			servers, err := sshconfig.Import(path)
			if err != nil {
				return err
			}
			count, err := app.store.Import(servers)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "imported %d server(s)\n", count)
			return nil
		},
	}
	sshConfigCmd.Flags().StringVar(&path, "path", sshconfig.DefaultPath(), "SSH config path")
	cmd.AddCommand(sshConfigCmd)
	return cmd
}

func newServerOpsCmd(app *App) *cobra.Command {
	var timeout time.Duration
	cmd := &cobra.Command{
		Use:   "ops NAME",
		Short: "Run a read-only server inspection snapshot",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			srv, err := app.store.Get(args[0])
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()
			snap, err := remoteops.DefaultSnapshot(ctx, app.ops, srv)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), snap.String())
			return nil
		},
	}
	cmd.Flags().DurationVar(&timeout, "timeout", 20*time.Second, "Remote command timeout")
	return cmd
}

func newServerUptimeCmd(app *App) *cobra.Command {
	return newServerRemoteCmd(app, "uptime NAME", "Show remote uptime", 10*time.Second, remoteops.Uptime)
}

func newServerDiskCmd(app *App) *cobra.Command {
	return newServerRemoteCmd(app, "disk NAME", "Show remote root filesystem usage", 10*time.Second, remoteops.Disk)
}

func newServerMemoryCmd(app *App) *cobra.Command {
	return newServerRemoteCmd(app, "memory NAME", "Show remote memory usage", 10*time.Second, remoteops.Memory)
}

func newServerProcessesCmd(app *App) *cobra.Command {
	return newServerRemoteCmd(app, "processes NAME", "Show top remote processes by CPU", 10*time.Second, remoteops.Processes)
}

func newServerLogsCmd(app *App) *cobra.Command {
	var timeout time.Duration
	var unit string
	var lines int
	cmd := &cobra.Command{
		Use:   "logs NAME --unit UNIT",
		Short: "Show remote systemd unit status and recent journal lines",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			srv, err := app.store.Get(args[0])
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()
			result, err := remoteops.Logs(ctx, app.ops, srv, unit, lines)
			if err != nil {
				return err
			}
			writeRemoteResult(cmd.OutOrStdout(), result)
			return nil
		},
	}
	cmd.Flags().StringVar(&unit, "unit", "", "systemd unit name")
	cmd.Flags().IntVar(&lines, "lines", 100, "Journal lines to read")
	cmd.Flags().DurationVar(&timeout, "timeout", 20*time.Second, "Remote command timeout")
	_ = cmd.MarkFlagRequired("unit")
	return cmd
}

func newServerRemoteCmd(app *App, use, short string, defaultTimeout time.Duration, fn func(context.Context, remoteops.Runner, inventory.Server) (remoteops.Result, error)) *cobra.Command {
	var timeout time.Duration
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			srv, err := app.store.Get(args[0])
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()
			result, err := fn(ctx, app.ops, srv)
			if err != nil {
				return err
			}
			writeRemoteResult(cmd.OutOrStdout(), result)
			return nil
		},
	}
	cmd.Flags().DurationVar(&timeout, "timeout", defaultTimeout, "Remote command timeout")
	return cmd
}

func newEnvCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "env",
		Short: "Read Foostash environments through the Go SDK",
	}
	cmd.AddCommand(
		newEnvConfigureCmd(app),
		newEnvStatusCmd(app),
		newEnvGetCmd(app),
		newEnvPullCmd(app),
		newEnvWatchCmd(app),
	)
	return cmd
}

func newEnvConfigureCmd(app *App) *cobra.Command {
	var flags envFlags
	cmd := &cobra.Command{
		Use:   "configure",
		Short: "Save Foostash SDK settings",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := app.cfg
			mergeFoostashConfig(&cfg.Foostash, flags)
			if cfg.Foostash.MasterKeyEnv == "" {
				cfg.Foostash.MasterKeyEnv = "FOOSTASH_MASTER_KEY"
			}
			if err := config.Save(app.paths.ConfigPath, cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "saved foostash config to %s\n", app.paths.ConfigPath)
			return nil
		},
	}
	addEnvConfigFlags(cmd, &flags)
	return cmd
}

func newEnvStatusCmd(app *App) *cobra.Command {
	var flags envFlags
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Check Foostash SDK access",
		RunE: func(cmd *cobra.Command, args []string) error {
			provider, opts, err := app.foostashProvider(flags)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), timeoutOrDefault(flags.timeout))
			defer cancel()
			count, err := mgrenv.Status(ctx, provider)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "foostash ok: project=%s env=%s secrets=%d server=%s\n", opts.Project, opts.Env, count, opts.ServerURL)
			return nil
		},
	}
	addEnvReadFlags(cmd, &flags)
	return cmd
}

func newEnvGetCmd(app *App) *cobra.Command {
	var flags envFlags
	cmd := &cobra.Command{
		Use:   "get KEY",
		Short: "Read one Foostash secret",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			provider, _, err := app.foostashProvider(flags)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), timeoutOrDefault(flags.timeout))
			defer cancel()
			value, err := provider.Get(ctx, args[0])
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), value)
			return nil
		},
	}
	addEnvReadFlags(cmd, &flags)
	return cmd
}

func newEnvPullCmd(app *App) *cobra.Command {
	var flags envFlags
	cmd := &cobra.Command{
		Use:   "pull",
		Short: "Pull Foostash secrets as dotenv",
		RunE: func(cmd *cobra.Command, args []string) error {
			provider, _, err := app.foostashProvider(flags)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), timeoutOrDefault(flags.timeout))
			defer cancel()
			secrets, err := provider.Pull(ctx)
			if err != nil {
				return err
			}
			data, err := mgrenv.RenderDotenv(secrets)
			if err != nil {
				return err
			}
			_, err = cmd.OutOrStdout().Write(data)
			return err
		},
	}
	addEnvReadFlags(cmd, &flags)
	return cmd
}

func newEnvWatchCmd(app *App) *cobra.Command {
	var flags envFlags
	var interval time.Duration
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Watch Foostash secrets for changes",
		RunE: func(cmd *cobra.Command, args []string) error {
			provider, opts, err := app.foostashProvider(flags)
			if err != nil {
				return err
			}
			ch, err := provider.Watch(cmd.Context(), interval)
			if err != nil {
				return err
			}
			for snap := range ch {
				fmt.Fprintf(cmd.OutOrStdout(), "%s project=%s env=%s secrets=%d\n",
					snap.PulledAt.Format(time.RFC3339), opts.Project, opts.Env, len(snap.Secrets))
			}
			return nil
		},
	}
	addEnvReadFlags(cmd, &flags)
	cmd.Flags().DurationVar(&interval, "interval", 30*time.Second, "Watch interval")
	return cmd
}

func newTUICmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Open the mgr terminal UI",
		RunE: func(cmd *cobra.Command, args []string) error {
			return tui.Run(tui.Deps{
				Store:  app.store,
				Config: app.cfg,
				Ops:    app.ops,
			})
		},
	}
}

func (a *App) foostashProvider(flags envFlags) (mgrenv.Provider, mgrenv.FoostashOptions, error) {
	opts := mgrenv.OptionsFromConfig(a.cfg.Foostash)
	if flags.serverURL != "" {
		opts.ServerURL = flags.serverURL
	}
	if flags.project != "" {
		opts.Project = flags.project
	}
	if flags.env != "" {
		opts.Env = flags.env
	}
	if flags.sshKeyPath != "" {
		opts.SSHKeyPath = flags.sshKeyPath
	}
	if flags.masterKey != "" {
		opts.MasterKey = flags.masterKey
	}
	if flags.masterKeyEnv != "" {
		opts.MasterKeyEnv = flags.masterKeyEnv
	}
	provider, err := mgrenv.NewFoostashProvider(opts)
	return provider, opts, err
}

func mergeFoostashConfig(cfg *config.FoostashConfig, flags envFlags) {
	if flags.serverURL != "" {
		cfg.ServerURL = flags.serverURL
	}
	if flags.project != "" {
		cfg.Project = flags.project
	}
	if flags.env != "" {
		cfg.Env = flags.env
	}
	if flags.sshKeyPath != "" {
		cfg.SSHKeyPath = flags.sshKeyPath
	}
	if flags.masterKey != "" {
		cfg.MasterKey = flags.masterKey
	}
	if flags.masterKeyEnv != "" {
		cfg.MasterKeyEnv = flags.masterKeyEnv
	}
}

func addEnvConfigFlags(cmd *cobra.Command, flags *envFlags) {
	cmd.Flags().StringVar(&flags.serverURL, "server-url", "", "Foostash server URL")
	cmd.Flags().StringVar(&flags.project, "project", "", "Foostash project")
	cmd.Flags().StringVar(&flags.env, "env", "", "Foostash environment")
	cmd.Flags().StringVar(&flags.sshKeyPath, "ssh-key", "", "Path to unencrypted SSH private key")
	cmd.Flags().StringVar(&flags.masterKey, "master-key", "", "Base64 Foostash master key; prefer --master-key-env")
	cmd.Flags().StringVar(&flags.masterKeyEnv, "master-key-env", "", "Environment variable containing the Foostash master key")
}

func addEnvReadFlags(cmd *cobra.Command, flags *envFlags) {
	addEnvConfigFlags(cmd, flags)
	cmd.Flags().DurationVar(&flags.timeout, "timeout", 10*time.Second, "Foostash request timeout")
}

func timeoutOrDefault(timeout time.Duration) time.Duration {
	if timeout == 0 {
		return 10 * time.Second
	}
	return timeout
}

func printServer(w io.Writer, srv inventory.Server) {
	fmt.Fprintf(w, "name: %s\n", srv.Name)
	fmt.Fprintf(w, "id: %s\n", srv.ID)
	fmt.Fprintf(w, "host: %s\n", srv.Host)
	fmt.Fprintf(w, "port: %d\n", srv.Port)
	if srv.User != "" {
		fmt.Fprintf(w, "user: %s\n", srv.User)
	}
	if srv.IdentityFile != "" {
		fmt.Fprintf(w, "identity_file: %s\n", srv.IdentityFile)
	}
	if srv.Group != "" {
		fmt.Fprintf(w, "group: %s\n", srv.Group)
	}
	if srv.Env != "" {
		fmt.Fprintf(w, "env: %s\n", srv.Env)
	}
	if len(srv.Tags) > 0 {
		fmt.Fprintf(w, "tags: %s\n", strings.Join(srv.Tags, ","))
	}
}

func writeRemoteResult(w io.Writer, result remoteops.Result) {
	out := strings.TrimRight(result.Stdout, "\n")
	if out != "" {
		fmt.Fprintln(w, out)
	}
}

func runSSH(ctx context.Context, srv inventory.Server, stdin io.Reader, stdout, stderr io.Writer) error {
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

	c := exec.CommandContext(ctx, "ssh", args...)
	c.Stdin = stdin
	c.Stdout = stdout
	c.Stderr = stderr
	if err := c.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return errors.New("ssh not found in PATH")
		}
		return err
	}
	return nil
}
