# Phase 0 validation — 2026-10-17

## Delivered

The initially empty directory is now an independent Git repository. No other application or repository was edited. Working identity: **Afterglow TV**, version **0.1.0**.

Implemented Go HTTP service, React/TypeScript/Vite frontend, persistent SQLite with checksum-verified transactional migrations, atomic first-owner creation, bcrypt password hashing, expiring hashed session tokens, login/logout, Origin and CSRF protection, OWNER/ADMIN settings authorization, audit records, security headers, rate limiting, JSON logs and graceful shutdown. Added public health/readiness/version endpoints and OpenAPI documentation.

The authenticated TV shell contains Live TV, Guide, Channels, Plex Setup and Settings. The first four are intentionally honest foundation/empty states. Name/timezone settings are functional and persisted. Responsive dark cinematic styling, visible focus, arrow/Enter/Escape navigation, native form editing and focus scrolling are implemented. Plex and scheduler interfaces are defined but have no implementations or production mocks.

## Final checks

| Check | Result |
|---|---|
| Containerized `go test ./...` | PASS — six tests covering configuration, migrations and HTTP auth/security/persistence |
| `go vet ./...` | PASS |
| `gofmt` check | PASS |
| Frontend ESLint | PASS, zero warnings |
| Frontend Prettier check | PASS |
| Vitest | PASS — four tests for focus behavior, first-run and offline states |
| TypeScript + Vite production build | PASS; JS approximately 242 kB / 76 kB gzip |
| Go production binary build | PASS, CGO disabled |
| Docker multi-stage image | PASS — `afterglow-tv:local`, non-root UID/GID 10001 |
| Browser functional test | PASS — Chromium, one comprehensive journey |
| Dependency audit | npm reported zero vulnerabilities; production dependency audit also passed |
| Compose live startup | PASS — `afterglow-app-1` healthy on loopback port 8080 |
| `/api/v1/health` | 200, `status=ok` |
| `/api/v1/ready` | 200, `status=ready` |
| `/api/v1/version` | 200, `version=0.1.0` |
| Fresh main installation | `setupRequired=true`; no owner credentials preconfigured |

The browser test creates a random-password owner on a separate test stack, verifies incorrect-password rejection, signs in, visits every page, navigates by arrow/Enter/Escape, confirms native input arrow behavior, saves the television name and America/New_York timezone, restarts the container, and verifies persisted session/settings. It then signs out, reloads, signs back in using the persisted account, and checks tablet/mobile overflow and browser runtime errors. All passed. Its disposable volume was removed; the main app's volume was preserved and remains ready for the user's setup.

Backend tests additionally cover setup races (exactly one owner), setup lockout, unauthorized settings reads, wrong Origin, missing CSRF, USER-role write rejection, invalid timezone, session expiry/revocation, hashed token storage, cookie flags, audit records, database reopen, migration idempotence/checksum/newer-schema rejection, malformed JSON and rate limiting.

Desktop (1440 px), tablet (768 px) and mobile (390 px) screenshots were visually reviewed. No clipping or horizontal page overflow was found. Local captures are in `web/test-results/live-desktop.png`, `live-tablet.png` and `live-mobile.png` (test artifacts excluded from Git).

Initial checks exposed a Go ServeMux method/path conflict, frontend test discovery including Playwright files, an effect lint issue and an accessible-heading test mismatch. These were corrected before the successful final checks; no failing behavior was replaced by placeholders.

## Files created

- Architecture and delivery: `ARCHITECTURE.md`, `IMPLEMENTATION_PLAN.md`, `README.md`, `VALIDATION.md`, `openapi.yaml`.
- Backend: `go.mod`, `go.sum`, `cmd/server/main.go`, configuration/database/HTTP services and tests under `internal/`, migration `001_foundation.sql`, Plex provider and scheduler generator interfaces.
- Frontend: `web/package.json`, lockfile, Vite/TypeScript/ESLint/Playwright configuration, HTML entrypoint, API client, TV shell, theme/styles, reusable focus utilities, unit tests and browser test lifecycle under `web/src` and `web/e2e`.
- Deployment/repository: `Dockerfile`, `compose.yaml`, `compose.dev.yaml`, `compose.test.yaml`, `.env.example`, `.dockerignore`, `.gitignore`, `.gitattributes`.

## Known limits

- Phase 0 only: no Plex connection, media sync, channels, scheduling, playback, EPG data, interstitials or background job runner. No fake Plex data.
- Browser keyboard behavior is validated; physical Android TV/Fire TV remotes and actual televisions are not. The focus traversal is ordered shell navigation, not a future spatial EPG grid.
- Live testing used Docker Desktop Linux containers and Chromium on Windows, not Unraid hardware, Safari or Firefox. Unraid instructions are provided but not live-tested.
- Go was unavailable on the Windows host; its tests, formatting verification and build used the Go 1.26 container toolchain.
- Only first-owner creation is exposed; additional user administration, Plex profile mapping and password recovery are later work. Preserve the config volume and account credentials.
- Single application process per SQLite volume. Rate limits are in-process and reset at restart; forwarded client IP headers are deliberately not trusted. A reverse proxy may cause users to share one rate-limit bucket.
- Default deployment is trusted-local HTTP with secure cookies disabled. TLS termination and COOKIE_SECURE=true must be configured for non-local deployment. Complete initial owner setup before opening network access.
- ESLint 9 emits an upstream support/deprecation notice during dependency installation; lint succeeds and npm reported no vulnerabilities. No external service or CDN is required at runtime.

## Exact recommended Phase 1 scope

1. Add migrations for Plex servers, selected libraries, normalized media items/versions, sync cursors and durable background-job state.
2. Implement the Plex provider adapter with timeouts, typed errors, encrypted tokens backed by a persistent local key, manual connection, connection health and discovery/manual fallback.
3. Build actual Plex Setup and library selection screens; never return server tokens to the browser.
4. Implement paginated full and incremental sync, newly added/deleted item reconciliation, retry/recovery, manual/scheduled refresh and meaningful job status.
5. Add authenticated artwork proxy/cache and required metadata, preserving server/library/rating keys and Plex authority.
6. Validate against explicitly configured real development Plex credentials; test offline/reconnect, pagination and incomplete-sync deletion safety. Keep fixtures test-only.

Do not implement channels, scheduling, playback or advanced programming during Phase 1. Phase 1 has not started.
