# mgr Architecture

`mgr` is a local-first CLI and terminal UI for managing SSH-backed server
inventory and reading environment secrets from Foostash. The current milestone
is intentionally servers-first: it observes and connects to hosts, but does not
provision machines, mutate remote packages, manage containers, or control
Kubernetes.

## Runtime Shape

The binary entrypoint is `cmd/mgr/main.go`, which delegates to the Cobra command
tree in `internals/cli`.

Top-level commands:

- `mgr server`: local server inventory operations.
- `mgr env`: read-only Foostash SDK operations.
- `mgr tui`: Bubble Tea terminal dashboard.

`internals/cli.App` owns the loaded config, resolved local paths, and inventory
store. It initializes lazily in Cobra `PersistentPreRunE`, so commands share the
same setup path without doing work for shell help/completion.

## Local State

Local data lives under the OS user config directory in an `mgr` subdirectory.
When the OS config directory is unavailable, mgr falls back to `~/.config/mgr`.

- `config.yaml`: versioned mgr configuration. It currently stores Foostash SDK
  settings.
- `inventory.yaml`: versioned server inventory managed by
  `internals/inventory.FileStore`.

Both files are written with user-only permissions.

Server records include name, ID, host, SSH port, user, identity file, tags,
group, environment, and timestamps. IDs are generated from the server name when
not provided. Tags are normalized to lowercase, deduplicated, and sorted.

## Server Management

The `internals/inventory` package is the source of truth for server records.
The `internals/sshconfig` package imports simple OpenSSH `Host` blocks into
inventory records and tags them with `ssh-config`.

Health checks in `internals/health` are TCP reachability checks against the
server SSH endpoint. They intentionally do not require remote shell access.

Interactive SSH uses the system `ssh` binary. The TUI exits the alt-screen
before launching SSH, then returns to the dashboard after the SSH process exits.

## Foostash Integration

Foostash access is read-only and implemented through the Go SDK imported as:

```go
github.com/Omotolani98/foostash-go-sdk
```

The SDK code is sourced from the GitHub `foostash` repository's `sdk/go`
subdirectory through the `go.mod` replace directive because the SDK module path
and repository path differ.

Supported operations:

- `env configure`: save SDK settings.
- `env status`: verify access and count secrets.
- `env get`: read one secret.
- `env pull`: render secrets as dotenv.
- `env watch`: watch for SDK snapshots.

Writes such as set, delete, rollback, and vault operations are not part of this
milestone because the current SDK is read-only.

## TUI

The terminal UI lives in `internals/tui` and uses Bubble Tea v2 via the
`charm.land/...` import paths.

The TUI has two modes:

- Servers: list inventory, filter by `/`, move selection, toggle details with
  enter, run health checks, and open SSH.
- Environment: show Foostash configuration, verify SDK access, and display the
  last successful check time and secret count while keeping secret values
  hidden.

The model stores terminal size from `tea.WindowSizeMsg` and uses v2's
declarative `tea.View` return value. Wide terminals render the server list and
detail panel side by side; narrow terminals stack the views.

## Extension Points

Future features should build on the current boundaries:

- Cloud/VPS/local VM providers should feed inventory-like server records behind
  provider interfaces.
- `mgr deploy` should consume inventory and Foostash env data to orchestrate
  isolated VM or service deployments.
- `mgr architect` should read inventory/deploy specs and generate architecture
  diagrams.
- Docker, Docker Swarm, and Kubernetes management should be added as separate
  provider packages instead of expanding the SSH inventory model directly.

## Legacy Scaffolding

`internals/manager` and `internals/resource` still contain an early
Kubernetes-style resource manager prototype. They are not wired into the CLI or
TUI.

`internals/storage` is a generic key/value storage interface and compatibility
implementation. The active server inventory does not use it; inventory has its
own YAML-backed store.
