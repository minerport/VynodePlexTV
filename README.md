# Afterglow TV

A standalone, self-hosted, TV-first home for future Plex-powered linear channels. Independent of VyNode Media.

**Phase 0:** secure owner setup/login, persistent SQLite, versioned API and a cinematic TV shell. Plex connection, channel creation, guide programming and video playback are intentionally not available yet. No sample media is presented as real content.

## Docker quick start

Requires Docker with Compose.

```powershell
Copy-Item .env.example .env
docker compose up --build -d
```

Open http://localhost:8080 and create the owner account. Passwords require 12–72 UTF-8 bytes. Keep the service on loopback until setup is complete. Data is stored in the Compose volume `afterglow_config` (Docker name `afterglow_afterglow_config`) at /config and survives restart/rebuild. `docker compose down` preserves data; `down -v` deletes it.

For LAN access set `BIND_ADDRESS=0.0.0.0` and `APP_ORIGIN=http://YOUR-HOST:8080` in `.env`, recreate the service, and open that exact origin. APP_ORIGIN must match the browser's scheme, hostname and port. Use a TLS reverse proxy and `COOKIE_SECURE=true` outside a trusted LAN. Do not expose first-run setup to an untrusted network.

```powershell
docker compose logs -f
docker compose restart
Invoke-RestMethod http://localhost:8080/api/v1/health
Invoke-RestMethod http://localhost:8080/api/v1/ready
```

## Development

Requires Node 22+ and Go 1.26 (or Docker for the Go toolchain). In separate terminals:

```powershell
# backend (with Go installed)
$env:CONFIG_DIR = './.local/config'
$env:APP_ORIGIN = 'http://localhost:5173'
go run ./cmd/server

# frontend
cd web
npm ci
npm run dev
```

Vite binds loopback port 5173 and proxies /api to backend port 8080. Alternatively use `docker compose -f compose.dev.yaml up --build` for the backend, then run Vite on the host. The development container persists state in a separate volume.

```powershell
go test ./...
go vet ./...
cd web
npm ci
npm run lint
npm run format:check
npm test
npm run build
npx playwright install chromium
npm run test:e2e
```

Build the application image first with `docker compose build`. Browser tests start a dedicated temporary stack from `afterglow-tv:local` on port 18080 and remove only its test volume. Run them from the `web` directory. See `web/playwright.config.ts`. Run containerized Go tests with `docker build --target backend-test -t afterglow-backend-test .` when Go is unavailable locally.

## Controls

Tab / Shift+Tab and arrow keys move focus across the TV shell. Enter activates; Escape returns to Live TV. Forms retain normal arrow-key editing. Focus indicators and automatic scrolling keep the current target visible. These controls establish the foundation for a future spatial guide; no guide navigation/playback is claimed in Phase 0.

## Unraid

Build the image (`docker build -t afterglow-tv:local .`) on the Unraid host or transfer a built image with `docker save`/`docker load`. Add a container using that image, bridge networking, container port 8080, and a writable host path such as `/mnt/user/appdata/afterglow-tv` mapped to `/config`. The image runs as UID/GID 10001: grant that account write access to the directory. Set APP_ORIGIN to the browser-facing URL. No privileged access, GPU or host networking is required. Complete setup on a trusted network. Stop the container before backing up the whole config directory; retain a matching image/version for restore.

## API and configuration

Implemented API: [openapi.yaml](openapi.yaml). Public endpoints: `/api/v1/health`, `/api/v1/ready`, `/api/v1/version`, `/api/v1/auth/status`. Authenticated session data includes a CSRF token; send it in `X-CSRF-Token` with the configured Origin for mutations. Settings writes require OWNER/ADMIN.

Environment variables: `CONFIG_DIR` (default /config), `LISTEN_ADDR` (:8080), `APP_ORIGIN` (http://localhost:8080), `COOKIE_SECURE` (false for local HTTP), `LOG_LEVEL` (INFO), `WEB_DIR` (web/dist). Docker sets WEB_DIR=/app/web. Use `.env.example` for Compose settings. Never commit credentials or local databases.

See [ARCHITECTURE.md](ARCHITECTURE.md) and [IMPLEMENTATION_PLAN.md](IMPLEMENTATION_PLAN.md) for boundaries and phased scope. Validation evidence is recorded in `VALIDATION.md` when Phase 0 has been exercised.
