# rclonarr

Homelab backup worker that triggers *arr in-app backups via their HTTP APIs, uploads the resulting archives with embedded [rclone](https://rclone.org), and backs up Komodo via `mongodump`. Config-directory sync remains available as a fallback.

With **`APP_SCHEDULE`** set (5-field cron, e.g. `0 3 * * *`), the process runs one **bootstrap** backup, then stays running and runs backups on that schedule (`restart: unless-stopped` in Compose).

Without **`APP_SCHEDULE`**, it runs once and exits (`0` = success, `1` = failure) — suitable for CronJob / `compose run`.

## Prerequisites

- **Go 1.24+**
- **Docker** (for container runs)
- **rclone** remote configured in `rclone.conf`
- **mongodump** (included in the Docker image; required on host for local runs when backing up Komodo)
- **golangci-lint** and **mockery** (development)

## Architecture

- `internal/core/domain/port` — interfaces
- `internal/core/service` — backup orchestration
- `internal/adapter/rclone` — rclone sync (library)
- `internal/adapter/http/servarr` — Sonarr/Radarr/Prowlarr/Lidarr v3 API (`Backup` command + [system backup list](https://sonarr.tv/docs/api/#/backup/get_api_v3_system_backup))
- `internal/adapter/http/bazarr` — Bazarr `/api/system/backups`
- `internal/adapter/mongo` — `mongodump` wrapper
- `internal/adapter/targets` — enabled target resolution
- `internal/di` — composition root

## Quick start

```bash
go mod download
go test ./...
go build -o bin/rclonarr ./cmd/main.go
```

### Container image (GHCR)

Images are published to GitHub Container Registry on pushes to `main` and version tags (`v*`):

```bash
docker pull ghcr.io/sargastico/rclonarr:latest
```

After the first publish, set the package visibility to **Public** under the repo’s **Packages** settings so pulls work without authentication.

### Local run example (Sonarr API)

```bash
export APP_ENABLED_TARGETS=sonarr
export APP_REMOTE_NAME=b2
export APP_REMOTE_PREFIX=homelab-backups
export APP_RCLONE_CONFIG_PATH=$HOME/.config/rclone/rclone.conf
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
  -v $HOME/.config/rclone/rclone.conf:/config/rclone/rclone.conf:ro \
  -v /path/to/sonarr/config:/backup-src/sonarr:ro \
  -e APP_ENABLED_TARGETS=sonarr \
  -e APP_SONARR_URL=http://sonarr:8989 \
  -e APP_SONARR_API_KEY=your-api-key \
  -e APP_SONARR_BACKUP_MOUNT=/backup-src/sonarr \
  -e APP_RCLONE_CONFIG_PATH=/config/rclone/rclone.conf \
  -e APP_REMOTE_NAME=b2 \
  -e APP_REMOTE_PREFIX=homelab \
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
| Komodo | — | `mongodump` then rclone sync |

When `APP_{APP}_URL`, `APP_{APP}_API_KEY`, and `APP_{APP}_BACKUP_MOUNT` are set, rclonarr triggers the app backup API, reads the new `.zip` from the mounted config tree, and **copies** it to `{remote}:{prefix}/{app}/` under a new timestamped name.

All targets upload **versioned `.zip` archives** (UTC timestamp in the filename, e.g. `sonarr_backup_20260527_031053.zip`) via **copy** only — previous remote backups are kept, which avoids overwriting a good backup with a failed upload.

Config-dir fallback zips the config directory first, then copies the archive. Komodo runs `mongodump`, zips the dump directory, then copies.

## Environment variables

All variables use the `APP_` prefix (via `envconfig`).

### Core

| Variable | Default | Description |
|----------|---------|-------------|
| `APP_ENABLED_TARGETS` | _(empty)_ | Comma-separated list of targets to back up |
| `APP_SCHEDULE` | _(empty)_ | Cron expression (`0 3 * * *`); empty = one-shot. When set: bootstrap backup, then cron |
| `APP_REMOTE_NAME` | _(required)_ | rclone remote name (e.g. `b2`) |
| `APP_REMOTE_PREFIX` | `homelab-backups` | Path prefix on the remote |
| `APP_RCLONE_CONFIG_PATH` | _(empty)_ | Path to `rclone.conf` (sets `RCLONE_CONFIG`) |
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
