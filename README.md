# rclonarr

Homelab backup worker that triggers *arr in-app backups via their HTTP APIs, uploads the resulting archives with the [Proton Drive CLI](https://proton.me/support/drive-cli), and backs up Komodo via `mongodump` and PostgreSQL via `pg_dump`. Config-directory sync remains available as a fallback.

With **`APP_SCHEDULE`** set (5-field cron, e.g. `0 3 * * *`), the process runs one **bootstrap** backup, then stays running and runs backups on that schedule (`restart: unless-stopped` in Compose).

Without **`APP_SCHEDULE`**, it runs once and exits (`0` = success, `1` = failure) — suitable for CronJob / `compose run`.

## Prerequisites

- **Go 1.24+**
- **Docker** (for container runs)
- **[Proton Drive CLI](https://proton.me/download/drive/cli/index.html)** authenticated (`proton-drive auth login`) — included in the Docker image
- **mongodump** / **pg_dump** (included in the Docker image; required on host for local DB backups)
- On headless Linux: `libsecret` + `dbus-x11` (or set `APP_PROTON_DRIVE_DBUS=true`)
- **golangci-lint** and **mockery** (development)

## Architecture

- `internal/core/domain/port` — interfaces
- `internal/core/service` — backup orchestration
- `internal/adapter/protondrive` — Proton Drive CLI upload wrapper
- `internal/adapter/http/servarr` — Sonarr/Radarr/Prowlarr/Lidarr v3 API (`Backup` command + [system backup list](https://sonarr.tv/docs/api/#/backup/get_api_v3_system_backup))
- `internal/adapter/http/bazarr` — Bazarr `/api/system/backups`
- `internal/adapter/mongo` — `mongodump` wrapper
- `internal/adapter/postgres` — `pg_dump -Fc` wrapper
- `internal/adapter/targets` — enabled target resolution
- `internal/di` — composition root

## Quick start

```bash
go mod download
go test ./...
go build -o bin/rclonarr ./cmd/main.go
```

### Authenticate Proton Drive

On a workstation:

```bash
# Download CLI: https://proton.me/download/drive/cli/index.html
proton-drive auth login
# Headless Linux:
# dbus-run-session -- proton-drive auth login
```

In Docker / Komodo, prefer **`APP_SCHEDULE`** so the container stays running. If no session exists (or it expires), rclonarr starts `proton-drive auth login` and prints the sign-in URL in the container logs — look for:

```text
PROTON DRIVE AUTH REQUIRED — open this URL in a browser to sign in
auth_url=https://account.proton.me/desktop/login?...
```

Open that URL on any device, complete login, and backups continue. No exec into the container required.

### Container image (GHCR)

Images are published to GitHub Container Registry on pushes to `main` and version tags (`v*`):

```bash
docker pull ghcr.io/sargastico/rclonarr:latest
```

After the first publish, set the package visibility to **Public** under the repo’s **Packages** settings so pulls work without authentication.

With **`APP_SCHEDULE` set**, the process stays running across auth prompts — check Komodo/container logs for `auth_url=` when a session is needed (see above).

### Local run example (Sonarr API)

```bash
export APP_ENABLED_TARGETS=sonarr
export APP_REMOTE_PREFIX=/my-files/homelab-backups
export APP_PROTON_DRIVE_BIN=$HOME/bin/proton-drive
export APP_SONARR_URL=http://localhost:8989
export APP_SONARR_API_KEY=your-api-key
# Mount the same path as Sonarr's /config (Backups/ lives under it)
export APP_SONARR_BACKUP_MOUNT=/path/to/sonarr/config

./bin/rclonarr
echo $?  # 0 = success
```

### Docker run (Sonarr API)

```bash
# or: docker build -t rclonarr:latest .

docker run --rm \
  -v /path/to/sonarr/config:/backup-src/sonarr:ro \
  -e APP_ENABLED_TARGETS=sonarr \
  -e APP_SONARR_URL=http://sonarr:8989 \
  -e APP_SONARR_API_KEY=your-api-key \
  -e APP_SONARR_BACKUP_MOUNT=/backup-src/sonarr \
  -e APP_REMOTE_PREFIX=/my-files/homelab \
  ghcr.io/sargastico/rclonarr:latest
```

See [docker-compose.example.yml](docker-compose.example.yml) for multi-instance setups (*arr on one host, Komodo on another).

## Supported targets

| Target | API backup | Fallback |
|--------|------------|----------|
| Sonarr | `POST /api/v3/command` (`Backup`) → `GET /api/v3/system/backup` | `APP_SONARR_CONFIG_PATH` sync |
| Radarr | same v3 API | `APP_RADARR_CONFIG_PATH` sync |
| Prowlarr | same v3 API | `APP_PROWLARR_CONFIG_PATH` sync |
| Lidarr | same v3 API | `APP_LIDARR_CONFIG_PATH` sync |
| Bazarr | `POST /api/system/backups` | `APP_BAZARR_CONFIG_PATH` sync |
| Profilarr | — | config dir sync only |
| Navidrome / Seerr / Soulsync | — | config dir sync only |
| Komodo | — | `mongodump` then Proton Drive upload |
| Postgres | — | `pg_dump -Fc` then Proton Drive upload |

When `APP_{APP}_URL`, `APP_{APP}_API_KEY`, and `APP_{APP}_BACKUP_MOUNT` are set, rclonarr triggers the app backup API, reads the new `.zip` from the mounted config tree, and **uploads** it to `{REMOTE_PREFIX}/{app}/` under a new timestamped name.

All targets upload **versioned `.zip` / `.dump` archives** (UTC timestamp in the filename, e.g. `sonarr_backup_20260527_031053.zip`). After each successful target backup, older versioned files in that remote folder are moved to Proton Drive trash when they exceed `APP_RETENTION_DAYS` (default 30). If no backup remains inside that window, nothing is deleted.

Config-dir fallback zips the config directory first, then uploads the archive. Komodo runs `mongodump`, zips the dump directory, then uploads. Postgres runs `pg_dump --format=custom` (single DB → versioned `.dump`, or all DBs → versioned `.zip`), then uploads.

## Environment variables

All variables use the `APP_` prefix (via `envconfig`).

### Core

| Variable | Default | Description |
|----------|---------|-------------|
| `APP_ENABLED_TARGETS` | _(empty)_ | Comma-separated list of targets to back up |
| `APP_SCHEDULE` | _(empty)_ | Cron expression (`0 3 * * *`); empty = one-shot. When set: bootstrap backup, then cron |
| `APP_REMOTE_PREFIX` | `/my-files/homelab-backups` | Proton Drive folder for backups |
| `APP_RETENTION_DAYS` | `30` | Trash versioned backups older than N days after a successful target run; if nothing is within the window, keep all. `0` disables |
| `APP_PROTON_DRIVE_BIN` | `proton-drive` | Path to the Proton Drive CLI binary |
| `APP_PROTON_DRIVE_DBUS` | `false` | Wrap each CLI call with `dbus-run-session` (headless Linux without a shared session bus) |
| `APP_COMMAND_POLL_INTERVAL_MS` | `1000` | Poll interval while waiting for API backups |
| `APP_COMMAND_TIMEOUT_SEC` | `300` | Max wait for backup command / Bazarr job |

### Per-app API (Servarr v3: Sonarr, Radarr, Prowlarr, Lidarr)

| Variable | Description |
|----------|-------------|
| `APP_{APP}_URL` | Base URL (e.g. `http://sonarr:8989`) |
| `APP_{APP}_API_KEY` | API key (`X-Api-Key` header); if empty, read from `config.xml` / `config.yaml` under `BACKUP_MOUNT` |
| `APP_{APP}_BACKUP_MOUNT` | Host path mounted at the app’s `/config` (backup zips + optional API key file) |

### Per-app API (Bazarr)

Same pattern: `APP_BAZARR_URL`, `APP_BAZARR_API_KEY`, `APP_BAZARR_BACKUP_MOUNT` (directory containing `bazarr_backup_v*.zip`).

### Per-app config paths (fallback)

| Variable | Target |
|----------|--------|
| `APP_SONARR_CONFIG_PATH` | sonarr |
| `APP_RADARR_CONFIG_PATH` | radarr |
| `APP_PROWLARR_CONFIG_PATH` | prowlarr |
| `APP_PROFILARR_CONFIG_PATH` | profilarr |
| `APP_LIDARR_CONFIG_PATH` | lidarr |
| `APP_BAZARR_CONFIG_PATH` | bazarr |
| `APP_NAVIDROME_CONFIG_PATH` | navidrome |
| `APP_SEERR_CONFIG_PATH` | seerr |
| `APP_SOULSYNC_CONFIG_PATH` | soulsync |

### Komodo / MongoDB

| Variable | Default | Description |
|----------|---------|-------------|
| `APP_KOMODO_MONGO_URI` | _(empty)_ | Full MongoDB URI (preferred) |
| `APP_KOMODO_MONGO_ADDRESS` | _(empty)_ | Host:port if URI not set |
| `APP_KOMODO_MONGO_USERNAME` | _(empty)_ | MongoDB user |
| `APP_KOMODO_MONGO_PASSWORD` | _(empty)_ | MongoDB password |
| `APP_KOMODO_MONGO_DB_NAME` | `komodo` | Database name |
| `APP_KOMODO_DUMP_DIR` | _(empty)_ | Fixed dump dir; if empty, a temp dir is used |
| `APP_KOMODO_DUMP_EXTRA_ARGS` | _(empty)_ | Extra `mongodump` flags |
| `APP_MONGODUMP_PATH` | `mongodump` | Path to `mongodump` binary |

### PostgreSQL

| Variable | Default | Description |
|----------|---------|-------------|
| `APP_POSTGRES_URI` | _(empty)_ | Full PostgreSQL URI (preferred), e.g. `postgresql://user:pass@db:5432/app` |
| `APP_POSTGRES_HOST` | _(empty)_ | Host if URI not set |
| `APP_POSTGRES_PORT` | `5432` | Port if URI not set |
| `APP_POSTGRES_USER` | _(empty)_ | User if URI not set |
| `APP_POSTGRES_PASSWORD` | _(empty)_ | Password if URI not set (passed via `PGPASSWORD`) |
| `APP_POSTGRES_DB_NAME` | _(empty)_ | Database name if URI not set (required unless `APP_POSTGRES_ALL_DATABASES=true`) |
| `APP_POSTGRES_ALL_DATABASES` | `false` | Dump every non-template DB (`-Fc` each) into a versioned zip |
| `APP_POSTGRES_DUMP_DIR` | _(empty)_ | Fixed dump dir; if empty, a temp dir is used |
| `APP_POSTGRES_DUMP_EXTRA_ARGS` | _(empty)_ | Extra `pg_dump` flags |
| `APP_PG_DUMP_PATH` | `pg_dump` | Path to `pg_dump` binary |
| `APP_PSQL_PATH` | `psql` | Path to `psql` (used to list DBs when dumping all) |

### Observability (optional)

| Variable | Default | Description |
|----------|---------|-------------|
| `APP_ENABLE_OTEL` | `false` | Export OpenTelemetry traces |
| `APP_OTEL_EXPORTER_ENDPOINT` | `localhost:4317` | OTLP gRPC endpoint |
| `APP_ENABLE_PROFILING` | `false` | Reserved (not implemented) |
| `APP_PYROSCOPE_SERVER_ADDRESS` | `http://localhost:4040` | Reserved |
| `APP_DEVELOPMENT` | `true` | Development mode |
| `APP_DEBUG` | `true` | Debug logging |
| `APP_ENVIRONMENT` | `local` | Environment label |
| `APP_RELEASE_VERSION` | `rclonarr` | Service version for telemetry |

## Multi-container deployment

Run separate container instances with non-overlapping `APP_ENABLED_TARGETS` when apps live on different hosts:

```yaml
# Host 1 — *arr stack
APP_ENABLED_TARGETS: sonarr,radarr,prowlarr,lidarr,bazarr

# Host 2 — Komodo
APP_ENABLED_TARGETS: komodo
APP_KOMODO_MONGO_URI: mongodb://user:pass@mongo:27017/komodo
```

## Development

```bash
# Lint
golangci-lint run ./...

# Generate mocks
mockery

# Tests
go test ./...
```

## License

See [LICENSE](LICENSE).
