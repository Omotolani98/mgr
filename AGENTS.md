# AGENTS.md

This file provides guidance to coding agents when working with this repository.

## Commands

```bash
go build ./cmd/mgr        # build the mgr binary
go run ./cmd/mgr <args>   # run without installing, e.g. go run ./cmd/mgr server list
go test ./...             # all tests
go test ./internals/inventory -run TestUpsert   # single package / single test
go vet ./...              # vet
```

Module path is `github.com/Omotolani98/mgr`; the only binary is `mgr` (`cmd/mgr`). Go 1.26.2.

`go.mod` has a `replace` redirecting `foostash-go-sdk` to the `sdk/go` subdir of the `foostash` repo — the import path and repo path differ on purpose; don't "fix" it.

## Architecture

`mgr` is a CLI + TUI for managing a local SSH server inventory, running read-only server inspection, and reading secrets from a Foostash server. `cmd/mgr/main.go` is a thin shell that calls `cli.Execute()`.

**Command tree (`internals/cli/root.go`)** — Cobra, four top-level groups:
- `server` — CRUD over local inventory (`add`, `list`, `get`, `remove`, `check`, `ssh`, `import ssh-config`) plus read-only remote ops (`ops`, `uptime`, `disk`, `memory`, `processes`, `logs`).
- `provider` — discovery sources (`list`, `sync SOURCE`) that upsert external inventory into local storage.
- `env` — read-only Foostash access (`configure`, `status`, `get`, `pull`, `watch`).
- `tui` — launches the Bubble Tea UI.

`App` holds `config.Paths`, loaded `config.Config`, and an `inventory.FileStore`. It is lazily initialized once in `PersistentPreRunE` via `App.init()` (guarded by `a.store != nil`), so every subcommand runs against a ready store. `env` subcommands build a Foostash provider on demand from saved config merged with per-invocation flags (`App.foostashProvider`).

**Data model & persistence** — two YAML files under `os.UserConfigDir()/mgr/` (falls back to `~/.config/mgr/`), both written `0o600`:
- `inventory.yaml` — `inventory.FileStore`, a versioned list of `Server` records. Keyed by `Name` **or** `ID`; `Upsert` dedups on either. IDs are slugified from the name, tags are lowercased/deduped/sorted, servers are sorted by name on save. Provider-managed records can carry `Source`, `SourceID`, and `LastSeenAt`. Missing file = empty inventory (not an error).
- `config.yaml` — `config.Config` (currently just `FoostashConfig`). Missing file yields `DefaultConfig()`.

**Providers (`internals/provider`)** — discovery abstraction for inventory sources. `Provider` implementations return `inventory.Server` values, and `mgr provider sync SOURCE` upserts them. The first provider is `ssh-config`; add future cloud/VPS/local VM discovery here rather than overloading `server add`.

**Foostash secrets (`internals/env`)** — `Provider` interface wrapping the SDK client. Master key resolution order: inline `MasterKey`, else the env var named by `MasterKeyEnv` (default `FOOSTASH_MASTER_KEY`). `RenderDotenv` validates keys and rejects secret values containing newlines. Prefer `--master-key-env` over inline keys.

**Health (`internals/health`)** — `Check` is a plain TCP `DialTimeout` against `host:port`; used by both `server check` and the TUI.

**Remote ops (`internals/remoteops`)** — read-only Linux inspection through the system `ssh` binary. Keep this boundary non-mutating: uptime, disk, memory, process listing, systemd logs, and combined snapshots are in scope; package installs, service restarts, file writes, provisioning, and deploy orchestration are not. The package exposes a `Runner` interface so tests can fake SSH and a Go SSH backend can be added later.

**TUI (`internals/tui`)** — Bubble Tea **v2** under the `charm.land/...` module paths (not the older `github.com/charmbracelet/bubbletea`). Two modes (`servers`/`env`), toggled with tab. Server mode supports filtering, details, health checks, read-only ops snapshots, and SSH. Note the SSH pattern in `Run`: to open an interactive session the model sets `sshTarget` and quits; `Run` then execs `ssh` outside the alt-screen and re-enters the program in a loop.

**ssh-config import (`internals/sshconfig`)** — hand-rolled OpenSSH config parser producing `inventory.Server`s tagged `ssh-config`; skips wildcard/`Match` blocks and expands `~`. Prefer invoking it through `internals/provider` for new code.

### Unwired scaffolding — read before touching

Two subsystems exist in the tree but are **not** connected to the CLI. Don't assume they're live:

- `internals/storage` (`Storage` interface, `InMemoryStorage`, `storage.FileStore`) — a generic key/value store, separate from `inventory.FileStore`. The active inventory path does **not** use it. This is the in-progress work on the `feat/storage-options` branch. `storage.FileStore` still reaches into `config.Home` / `config.STOREFILENAME` — the `config.Home` global and `config.InitConfig()` exist only as a compatibility shim for this older code.
- `internals/manager` + `internals/resource` (`Manager`, `Pod`, `Service`, `Metadata`) — a stubbed Kubernetes-style resource manager. Most methods are no-ops/`nil` returns. Not referenced by `cli` or `tui`.

When adding server/secret features, work through `inventory` / `env` / `cli`. Only touch `storage` / `manager` / `resource` if the task is explicitly about that scaffolding.
