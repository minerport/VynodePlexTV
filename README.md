# Afterglow TV

A standalone, self-hosted, TV-first home for future Plex-powered linear channels. Independent of VyNode Media.

**Phase 1:** Plex account sign-in, encrypted server connections, library discovery, synchronized movie/TV metadata, artwork caching, durable background jobs and an administration media browser. Owner authentication and the cinematic TV shell remain intact. Virtual channels, schedules and playback are intentionally not implemented. No sample media is presented as real content.

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

## Connect Plex

Sign in as OWNER/ADMIN and open **Plex Setup → Servers → Sign in with Plex → Open Plex sign-in**. Complete authentication on Plex's own website, return to Afterglow, and choose a server and connection. Prefer a reachable local HTTPS connection when available. If a listed address is unreachable from the Docker container, choose another advertised connection. The application validates both server identity and library access before saving. Existing machine identities reconnect rather than duplicate.

Plex account sign-in needs Internet access to Plex's account service during linking. Routine health, inventory synchronization and artwork use the configured Plex server directly; there is no hosted Afterglow service. Passwords are entered only on Plex. Linking lasts up to ten minutes, is bound to the application administrator, and keeps account/resource credentials only in expiring server memory. Only the chosen server's token is encrypted and persisted. Restarting the app cancels pending sign-in flows but does not invalidate saved server connections. You may revoke Afterglow from Plex's Authorized Devices page when needed.

Some DNS resolvers reject Plex's private-address `plex.direct` records. After a DNS failure, Afterglow can connect to the IPv4 address encoded in Plex's canonical hostname, retaining the original HTTPS hostname, SNI and certificate verification. This fallback does not change router DNS or disable TLS validation. Choose the server's reachable LAN address; advertised virtual bridge interfaces may not be reachable from another machine. IPv6-only DNS failures require a working resolver or another advertised connection.

**Manual fallback:** choose Add Plex server and enter an HTTP(S) URL, token, and optional name. See [Plex's token instructions](https://support.plex.tv/articles/204059436-finding-an-authentication-token-x-plex-token/) if needed. Never paste tokens into chat, logs or source files. Edit/reconnect accepts a blank token to retain the encrypted credential. Default HTTPS validation is always enabled; there is no insecure TLS switch. Redirecting server URLs are rejected so credentials cannot follow a redirect to another origin.

Docker's `localhost` refers to the container. Use the Plex server's LAN hostname/address or a reachable advertised connection. No port is added to a complete URL. URL credentials, query strings and fragments are rejected. This is an administrator-controlled local service: private network addresses are intentionally supported.

## Select and synchronize libraries

Connection setup queues **Refresh libraries**. Under Libraries, select Movie/TV sections, then use **Sync changes**, **Full refresh**, or **Sync selected**. Other section types are discovered and labeled unsupported for synchronization. Selection changes retain existing inventory. Disconnect erases the stored token, disables future work and deselects libraries, while retaining media IDs and metadata. A confirmation is required in the UI. Finish/cancel active jobs before disconnecting or editing.

Movies, shows, seasons and episodes retain server/rating-key identity, hierarchy, ratings, runtimes, summaries, genres, people/roles, collections, labels, external GUIDs, media parts and streams when supplied by Plex. Local IDs persist across repeated scans and removal/reappearance. Filesystem paths remain internal and are omitted from all current API responses. The Media browser supports server/library/type/title/availability filters and show/season drill-down. It is an administrative inspection view, not a player.

The default browser lists **Movies & shows**. Open a show to see its seasons, then open a season to see episodes in numeric order. Breadcrumbs return to the show or the original library view; keyboard focus follows the new list. Explicit season/episode type filters remain available for inventory inspection and episode-title searches.

Incremental scans traverse the complete paginated inventory, but only request detailed metadata for new/changed items (or items without usable update timestamps). Full scans also refresh unchanged details. Background defaults: health every 5 minutes, incremental sync every 6 hours, full refresh at least daily, artwork maintenance hourly. Configure intervals and the cache budget under Sync settings. Selected libraries are eligible for automatic sync immediately. Jobs run one at a time to bound Plex load; detail requests are sequential, with metadata committed per page rather than per field.

**Inventory safety:** timeout, authentication failure, cancellation, malformed/inconsistent pagination, changed library identity/revision or app interruption never reconciles missing media. Only a completed traversal with consistent totals and unique observed identities can mark unseen items unavailable. Rows are never hard-deleted. Safe page updates may survive a failed/cancelled scan. Jobs interrupted by application restart become CANCELLED and can be safely rerun. Disappearing library sections are marked not present without making their media unavailable.

The Jobs tab shows durable queue/running/success/failure/cancellation status, real processed/total counts and the current item. Null totals are indeterminate. Completed job history is retained for 30 days; the API lists the latest 100. The interface polls every four seconds while visible.

## Secrets, artwork and backups

Plex credentials use AES-256-GCM with a random nonce and server-bound additional authenticated data. Ciphertext and key-version metadata live in SQLite; `/config/secrets/plex-key-v1` stores the separate 32-byte key. `/config/secrets/plex-client-id` stores the stable application identifier. Files are created with mode 0600 inside a 0700 directory where supported. The key is independent of owner passwords. Losing/corrupting the key fails closed rather than silently replacing it. Rotation is versioned for future implementation; there is no rotation UI yet.

**Back up the complete /config directory, including secrets.** Stop the container before copying, or use a coordinated SQLite backup. A database without its matching key cannot decrypt saved tokens. Cache contents can be discarded, but preserve the database and secrets together. Existing migration 001 is unchanged; migration 002 upgrades Phase 0 in place. Do not run multiple application workers against one volume. Restore a matching old database/image backup for rollback rather than downgrading an upgraded database.

Artwork is served by authenticated OWNER/ADMIN endpoints. The browser never sees a Plex token or upstream image URL. Cached image bytes and private upstream validators live under `/config/cache/artwork` and SQLite. Images are revalidated after 24 hours; an existing cached copy may be served when Plex is offline. Default disk budget is 512 MB, with oldest-accessed entries evicted after writes and by hourly maintenance. Allowed image formats are JPEG/PNG/WebP/GIF, capped at 16 MB per response; SVG/HTML and arbitrary external artwork URLs are rejected. Current Phase 1 metadata/artwork APIs are admin-only.

## API regeneration and tests

Run `node scripts/update-plex-openapi.mjs` from the repository root after `npm ci` in web to regenerate the Phase 1 contract additions. All writes use existing application sessions, exact-Origin validation and per-session CSRF tokens. Plex upstream authorization errors return safe 502 responses, distinct from application-session 401 responses.

The browser test uses an isolated test-only Plex fixture container with a randomly generated token. It exercises the real application HTTP/sync/cache stack and a container restart; it does not prove compatibility with a real Plex server. The normal application image/Compose configuration contains no fixture service. Live Plex evidence is separately recorded in VALIDATION.md. Unit tests use HTTP fixtures for failures, cancellation and inventory safety; they never require your Plex account.
