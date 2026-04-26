# Prow

**Prow** is a unified control plane for SecOps teams: connect tools, normalize data to a common schema (PCS), and operate with auditability. This repository is an early **Phase 0A** scaffold.

## Two binaries

| Binary | Role |
|--------|------|
| **`prow`** | Thin **analyst CLI**: config profiles, HTTP calls to `prowd`, human-friendly output. |
| **`prowd`** | **Server**: HTTP API, lab SQLite storage, auth, and (later) connectors, audit, agent, web UI. |

The CLI does **not** embed the database, connector engine, or API server. **`prowd` is the source of truth** for events and configuration in server/lab modes.

### Architecture boundary (important)

**`prow` must remain a thin client** and **must not** import server-only packages such as `internal/api`, `internal/store`, `internal/connectors`, `internal/audit`, `internal/agent`, `internal/web`, `internal/consent`, or `internal/daemon`.

Automation and integrations should target **`prowd`’s HTTP API** (or a future public SDK), not the CLI process.

This rule is enforced in CI by an import-boundary test under `cmd/prow`.

## Lab mode

**Lab mode** runs `prowd` on your machine with **SQLite** and a **local bearer token**—no Postgres, OpenSearch, or Wazuh required. The **`prow`** CLI points at `http://localhost:7777` (default bind) and uses the token from `prowd init --lab`.

- Config: `~/.prow/prowd-config.yaml` (server), `~/.prow/config.yaml` (CLI).
- Data: `~/.prow/prowd.db`, token in `~/.prow/prowd.token`.
- Phase 0A stores the CLI token **in the config file** for convenience; production should use an OS keychain (see `prow login` TODO in code).

## Quickstart: Windows (PowerShell)

From the repo root (with [Go](https://go.dev/dl/) installed):

```powershell
go build -o prow.exe ./cmd/prow
go build -o prowd.exe ./cmd/prowd

.\prowd.exe init --lab
.\prowd.exe serve --config $env:USERPROFILE\.prow\prowd-config.yaml
```

In a **second** terminal:

```powershell
$tok = (Get-Content $env:USERPROFILE\.prow\prowd.token).Trim()
.\prow.exe login http://localhost:7777 --token $tok
.\prow.exe doctor
.\prow.exe alerts
.\prow.exe alerts --json
```

Optional API checks (server terminal still running):

```powershell
curl.exe -s -H "Authorization: Bearer $tok" http://127.0.0.1:7777/health
curl.exe -s -H "Authorization: Bearer $tok" http://127.0.0.1:7777/v1/events
```

## Quickstart: macOS / Linux

```bash
go build -o prow ./cmd/prow
go build -o prowd ./cmd/prowd

./prowd init --lab
./prowd serve --config "$HOME/.prow/prowd-config.yaml"
```

Second terminal:

```bash
TOKEN=$(cat ~/.prow/prowd.token)
./prow login http://localhost:7777 --token "$TOKEN"
./prow doctor
./prow alerts
./prow alerts --json
```

```bash
curl -s -H "Authorization: Bearer $TOKEN" http://127.0.0.1:7777/health
curl -s -H "Authorization: Bearer $TOKEN" http://127.0.0.1:7777/v1/events
```

## Available commands

### `prowd` (server)

| Command | Description |
|---------|-------------|
| `prowd init --lab` | Create `~/.prow/`, SQLite DB, lab config, admin token, seed demo PCS events. |
| `prowd serve --config <path>` | Start HTTP API (default config path: `~/.prow/prowd-config.yaml`). |

### `prow` (CLI)

| Command | Description |
|---------|-------------|
| `prow login <url> --token <t> [--profile name]` | Save server URL and token for a profile (Phase 0A: plaintext in `~/.prow/config.yaml`). |
| `prow doctor` | Validate config and call `/health` and `/version` on the current profile. |
| `prow alerts` | List events from `GET /v1/events` (table output). |
| `prow alerts --json` | Same, as JSON (also if `output.format: json` in config). |

Global flag: **`--profile`** (default profile name: `default`).

## Development

```bash
go mod tidy
go test ./...
go build -o prow ./cmd/prow
go build -o prowd ./cmd/prowd
```

## Scope (Phase 0A)

**In:** two binaries, lab init/serve, SQLite + seeded events, bearer auth, minimal PCS `Event`, CLI login/doctor/alerts, import-boundary guard, CI.

**Not yet:** Wazuh or other connectors, agent, Postgres/OpenSearch, web UI, hash-chained audit, production keychain storage for tokens.

## License

TBD.
