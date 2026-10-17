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

# Phase 1 validation — 2026-10-17

Phase 1 is implemented and validated against a real Plex Media Server, version 1.43.3.10896. Application version: **0.2.0**. No Phase 2 features were implemented. All work remained inside the standalone Afterglow repository.

## Delivered architecture and migration

The former Plex provider stub is replaced with a dedicated protocol adapter and service layer, normalized application models, encrypted credential vault, durable single-worker jobs, reconciliation-safe synchronization and bounded artwork caching. React calls versioned application APIs; Plex XML, credentials and filesystem paths remain backend-only. Scheduling remains untouched.

Migration **002_plex_inventory.sql** adds servers, libraries, media items, normalized metadata entities/relationships, external IDs, media versions/streams, jobs, scan observations, Plex settings and artwork cache metadata. Migration 001 was not edited. Existing Phase 0 configuration was upgraded in place without deleting /config.

Integrated Plex endpoints: server root identity, `/library/sections`, paginated `/library/sections/{key}/all` for movies/shows/seasons/episodes, `/library/metadata/{ratingKey}`, stored relative artwork paths, and Plex account `/api/v2/pins`, `/api/v2/pins/{id}` and `/api/v2/resources`. Strong PIN account login was added at the operator's request; manual-token fallback remains available.

## Final checks

| Check | Result |
|---|---|
| `go test ./...` in Docker backend-test target | PASS — 21 top-level tests plus nested cases |
| `go vet ./...` and `gofmt` check | PASS |
| Frontend ESLint | PASS, zero warnings |
| Frontend Prettier | PASS |
| Vitest | PASS — 13 tests |
| TypeScript/Vite production build | PASS — JS approximately 267 kB / 83 kB gzip |
| Go production build | PASS — CGO disabled |
| Docker production image and main Compose stack | PASS — non-root container, healthy, version 0.2.0 |
| Chromium end-to-end journey | PASS — owner/auth/settings/navigation, Plex fixture sync/artwork, TV hierarchy and restart persistence |
| OpenAPI internal references | PASS — 28 paths, no unresolved references |
| Real credential/key source scan | PASS — 72 source files scanned, no saved token/key found |
| Git whitespace review | PASS |

Backend fixtures cover encryption/tampering/key-loss/version handling, fixed safe errors/redaction, connection identity, duplicate servers, authorization, timeouts and TLS, discovery/selection, all four media types, metadata relationships, GUIDs, versions/streams, cached artwork, full and incremental scans, stable IDs, updates/removal/reappearance, partial traversal and offline safety, cancellation and job lifecycle. HTTP tests enforce OWNER/ADMIN, Origin/CSRF and authenticated artwork access. Account linking tests cover strong PINs, administrator ownership, expiry/cancellation, resource selection and secret boundaries. DNS fallback tests ensure strict hostname eligibility and continued rejection of untrusted certificates before credentials are sent.

Frontend tests cover setup connection success/failure/unauthorized states, masked token handling, offline server state, libraries and sync controls, honest progress, inventory availability, USER access exclusion, official Plex sign-in, and show → season → episode navigation with breadcrumbs. Browser tests additionally exercise actual API/worker/database/cache integration using an isolated fixture container, separate volume and random test credentials. Fixture results are distinct from the live evidence below.

## Real Plex results

The operator completed Plex account authorization, connected their server and selected one Movie and one TV library. Five libraries were discovered: two movie, two show and one artist section. The artist section is honestly marked discovery-only. Other libraries remain unselected.

| Inventory type | Successfully synchronized |
|---|---:|
| Movie | 3,506 |
| Show | 225 |
| Season | 568 |
| Episode | 6,373 |
| **Total** | **10,672** |

Both selected-library jobs succeeded with processed counts matching authoritative totals (3,506 and 7,166). Representative records inspected included a movie with summary/year/runtime, 47 metadata relationships, four external identifiers and one media version; a show with 105 metadata relationships and four external identifiers; a season with its show parent; and an episode with both season/show parents, runtime and media version. The complete inventory contains 83,881 stream records. Nullable fields absent from Plex remain absent rather than invented.

Real movie and show posters/backgrounds were fetched through the backend, including JPEG and WebP. The operator confirmed artwork was present, then identified that the initial default list mixed TV episodes into the top level. This was corrected: the default now shows movies/shows, show selection opens seasons, season selection opens episodes, numeric order is used, breadcrumbs restore the original scope, and keyboard focus follows the loaded list. Backend, frontend and browser regression tests pass for the revised grouping.

### Compatibility issues found and fixed during live validation

1. The local resolver returned NXDOMAIN for the server's private-address `plex.direct` hostname even though the LAN endpoint was reachable. The transport now falls back to the IPv4 encoded in Plex's strict canonical hostname only after DNS failure. Original HTTPS hostname, SNI and certificate verification remain intact. Real HTTPS identity retrieval and operator reconnection succeeded. No router or Plex settings were changed.
2. The real Plex server rejected `sort=ratingKey:asc` with HTTP 500. `sort=titleSort:asc` returned HTTP 200 and is now used; the fixture rejects unsupported sorting. Failed attempts did not reconcile media as missing. Their FAILED job history remains visible as truthful diagnostic history.

### Repeat scan, outage and recovery

A second main-stack scan of both selected libraries succeeded. There are **zero duplicate server/rating-key identities**. Comparing the completed first-scan snapshot with the post-repeat, post-restart database confirmed all **10,672 local IDs, Plex keys, server associations and availability values** were identical.

A consistent SQLite snapshot and copied encryption/cache material were placed in a separate temporary validation volume. Only that copy's Plex endpoint was changed to an unreachable loopback port. Its health check and both real-inventory sync jobs failed as expected. A digest over every local ID/key/availability tuple remained unchanged. Restoring the original URL allowed health and both incremental scans against the real Plex server to succeed, again with an identical digest. The actual Plex server and main application's connection were not disrupted by the outage simulation. Mid-traversal failures and cancellation are additionally exercised by deterministic mock tests; real media was not deleted to manufacture a removal test.

### Upgrade and restart persistence

The Phase 0 backup was compared directly with the upgraded main database: owner ID/username/password hash/role, the pre-existing session/CSRF values, and application settings matched. The migration fixture independently tests the same upgrade boundary.

The actual main container was recreated with the final image after live synchronization. Checks confirmed:

- Health/readiness HTTP 200 and version 0.2.0.
- Owner, current sessions and settings persisted.
- The server's encrypted credential bytes and key version were unchanged; a fresh post-restart real health job succeeded.
- All five library rows and selection states persisted.
- All 10,672 synchronized rows retained identity and availability.
- Real movie/show posters and backgrounds remained retrievable from the persistent cache.

## Security and source review

Plex tokens are AES-256-GCM encrypted with random nonces and server-bound authenticated data. The separate /config key and stable client identifier use restrictive filesystem permissions. No account password is collected by Afterglow. Account/PIN secrets are temporary backend memory; normal responses contain no Plex tokens. HTTPS verification remains enabled, redirects are blocked, writes require session/role/Origin/CSRF checks, and artwork paths are restricted to stored Plex library paths. Raw transport errors, tokens and passwords are not logged. Real token/key byte representations were checked against source files without printing the secrets.

Databases, keys, backups, caches, screenshots and validation helpers are ignored or held in Docker volumes, not committed. The pre-upgrade backup remains in ignored local storage for recovery. Disposable fixture stacks and temporary validation volumes are cleaned up after validation.

## Files and scope

Added the Phase 1 migration and upgrade test; Plex client/auth/metadata/service/library/inventory/sync/job/artwork/encryption modules and tests; HTTP Plex routes/tests; Plex admin, account sign-in and browser components/styles/API hooks/tests; and an OpenAPI generation script. Updated server startup, Docker version/test stack, frontend dependencies and browser tests, README, ARCHITECTURE, IMPLEMENTATION_PLAN, OpenAPI and this validation record. The commit's file list is the authoritative complete inventory.

## Known limitations and next phase

- Movie and show sections synchronize; music/other section types are discovery-only.
- One worker serializes jobs and detail requests. A large sync can delay queued health/discovery jobs. A single application instance must own /config.
- Incremental sync traverses inventory pages and fetches detail when Plex's update timestamp changes; periodic full refresh handles metadata not reflected by timestamps. Plex does not provide a transactional snapshot; count/revision/duplicate checks conservatively reject observed concurrent changes.
- IPv4 canonical Plex DNS fallback is supported; IPv6-only resolution failures need a functioning resolver or another advertised connection.
- Metadata and artwork are admin-only in this phase. Plex profile mapping, key-rotation UI and self-signed TLS overrides are deferred. Fallback metadata identity uses server/type/name only where Plex omits an identifier, so ambiguous same-name entities cannot always be distinguished.
- Artwork retrieval is validated; no playback, transcoding, channel, schedule or guide engine exists yet.

**Recommended Phase 2: Virtual Channels and Rule Engine only.** Add channel/group CRUD and migrations, typed source/rule definitions, independently tested AND/OR/NOT matching over normalized inventory, static/show/collection/genre/era/person/recent sources, ordering and repeat-policy configuration, a visual rule builder, source previews, and polished channel creation/editing. Exclude unavailable/unselected source content from new matches while preserving historical references. Keep schedule generation, EPG and playback for their explicitly authorized later phases.

# Phase 2 validation — 2026-10-17

## Baseline, scope and upgrade

Started from clean Phase 1 commit `36ed7266541cc9bc236c13732c06a319df94309e` in this standalone repository. Read the project documents, migrations, authentication/settings implementation, inventory models and focus handling. Baseline backend and all 13 existing frontend tests passed before implementation. No VyNode repository was modified.

Kept a complete stopped-container Phase 1 config archive in ignored `.local/backups/phase1-config.tar.gz`, tagged the old image `afterglow-tv:phase1`, and made a consistent SQLite snapshot for comparison. Migration 003 applied to the actual existing `/config` volume through normal Compose startup. It adds channel configuration and upload metadata only; no scheduler/EPG/playback tables or behavior.

## Automated validation

- `docker build --target backend-test -t afterglow-backend-test .`: PASS. Runs `go test ./...`, `go vet ./...`, and verifies no gofmt changes. There are 28 top-level backend tests, including golden rule subcases.
- `npm test`: PASS, 19 tests across five files; all Phase 0/1 tests retained.
- `npm run lint`: PASS with zero warnings.
- `npm run format:check`: PASS.
- `npm run build`: PASS, TypeScript and Vite production build.
- `docker compose build`: PASS, production Go build plus React build, final non-root image version 0.3.0.
- `npm run test:e2e`: PASS. The isolated fixture Docker stack exercises owner setup/login/logout, Plex connection/discovery/synchronization, existing TV hierarchy, settings, artwork, four channel types, visual nested NOT, preview counts, duplication, disable/delete, browser navigation and container persistence. Screenshots cover desktop and tablet layouts. Test containers/network/volume are removed by teardown.

Backend additions cover typed fields/operators, nested AND/OR/NOT, invalid fields/value types/references, depth/node caps, injection strings, NULL behavior, relative UTC dates with an injected clock, inherited show metadata, numeric episode ordering/specials/selected seasons, static order/pagination/retained unavailable references, counts, offline resolution, CRUD/duplicate/unique numbers, groups, uploads, fallback escaping, audit records, RBAC and CSRF. A Phase 1 schema upgrade test preserves existing owner/session/settings and validates rerun safety. HTTP tests enforce anonymous/USER restrictions and validate real service behavior behind the handlers.

Initial development checks caught fixture/test expectations after atomic duplication auditing and a browser label-selector mismatch; these were corrected before the passing runs. Real custom-artwork testing also caught root-owned files created by a local validation helper, not the app's upload path. Those helper-created files/directories were assigned to the application's UID; the final browser test uses the actual upload control and non-root server path.

## Real synchronized inventory and independent counts

Created six persistent channels from the real, already synchronized Plex inventory. No fixture media was inserted into production.

| Channel | Source | Matching playable items | Measured count + first-page preview |
| --- | --- | ---: | ---: |
| 101 Handpicked Cinema | Static, five real movies | 5 | 8.8 ms |
| 102 Show Spotlight | A real multi-season show, specials excluded | 142 | 27.7 ms |
| 103 Action Movies | Movie AND Action genre | 1,284 | 24.7 ms |
| 104 90s Movies | Movie AND year 1990–1999 | 64 | 7.3 ms |
| 105 Action & the 90s | (Action OR 1990–1999) AND NOT episode | 1,320 | 22.7 ms |
| 106 Collection Spotlight | An existing Plex collection | 15 | 16.3 ms |

Direct SQL independently confirmed movie total 3,506, Action 1,284, decade 64, nested union 1,320 and show 142. Static preview preserves the five selected identities and order. Repeated previews return identical results, and count APIs agree with preview counts. These are local measurements on this machine, not general performance guarantees. An initial correlated-OR relationship query measured 1.6–1.7 seconds; rewriting it to use indexed item/parent identities reduced the same real queries to tens of milliseconds without changing golden or real results. Pages are bounded and counts do not load media objects.

Real Chromium browser validation on `http://localhost:8080/#channels` verified all six cards and counts, static and nested-rule editing/saving, duplicate, disable, confirmed deletion, group create/rename/delete, uploaded transparent PNG, rendered artwork and tablet layout without horizontal overflow. Screenshots were inspected locally and are excluded from Git. The test used a temporary expiring owner session without changing the owner password; it is revoked and removed after validation.

## Docker restart and inventory preservation

Restarted the actual application container and compared complete channel responses, group responses and custom artwork bytes. Channel identities, source JSON/rules, static membership/order, disabled duplicate state, groups and the uploaded logo persisted. The temporary duplicate/group were deleted after the checks; the six useful channels remain.

Comparing the upgraded database to the consistent pre-upgrade snapshot confirmed:

- 10,672 inventory rows before and after; every local ID, Plex rating key, source server and availability value matched.
- Zero duplicate Plex media identities; 83,881 existing stream rows retained.
- All five library identities/selections persisted.
- The owner/password hash, existing sessions and settings persisted.
- The encrypted server credential and key version were unchanged. A fresh post-restart health check authenticated successfully using that credential.

## Plex outage and recovery

Temporarily changed this application's saved Plex endpoint to an unreachable loopback port, keeping the real Plex server and other applications untouched. Ran an actual health job and confirmed FAILED status. While disconnected, the channel-list API and real browser loaded, and all six source previews returned the same counts from SQLite. Existing inventory identities and availability remained unchanged. Restored the original connection in a finally-protected cleanup path; a new health job SUCCEEDED. This is a controlled connection-outage simulation, not a claim that the physical Plex server was shut down.

## Security and source review

Channel administration, metadata selectors, preview and uploads require OWNER/ADMIN server-side. Writes also require the existing Origin/CSRF checks. Rule SQL uses fixed field/operator fragments and bound values, with bounded tree and pagination sizes. Static references accept only playable existing media. Audit events contain actor, target, UTC timestamp and a safe action summary; channel changes and audits commit atomically. Deletion retains Plex inventory.

PNG/JPEG/WebP uploads are decoded and checked for size/dimensions, use SHA-256 filenames under `/config`, have a bounded storage budget and never accept a filesystem path. SVG uploads are rejected; the generated fallback SVG escapes channel text. Shared images remain referenced after duplication/deletion. Artwork endpoints authenticate before serving content and never include Plex credentials in URLs. The existing token/key representations were scanned against source without printing them: PASS. Private database snapshots, credentials, generated uploads, cached artwork, browser artifacts and validation helpers remain ignored/untracked.

## Files and limitations

Added `internal/channels/` (models, rules, service, resolver, metadata, groups, artwork and tests), migration 003, HTTP channel routes/tests, `web/src/channels/` (API, builder, pickers, previews, groups, cards, styles and tests), the browser channel journey, and a channel OpenAPI generator. Updated API routing, migration tests, app navigation/focus, API error details, Docker/package versions, E2E fixture/journey and project documentation. The commit file list is authoritative.

Limits are deliberate: 1,000 explicit static items, 64 rule nodes/eight levels, 64 multi-values, 100 preview items per request, 8 MiB/image and 256 MiB upload storage. SQLite string folding is ASCII-oriented. Relative-date results can change with UTC time. Missing positive metadata is false; negation includes missing values. Collection/person identity quality follows the Plex metadata received. Administration remains OWNER/ADMIN-only; household channel favorites and viewer playback remain future work. One app instance owns the volume. Keyboard/browser navigation was tested; this is not physical TV-remote hardware validation.

**Phase 3 recommendation: Deterministic Linear Scheduling Engine only.** Add generated program entries, stable seeds/revisions, sequential/random/smart-random policies, no-repeat windows, schedule horizon and idempotent extension jobs, block-scheduling foundations and schedule previews. Do not begin Phase 3 without explicit authorization. No Phase 3 implementation was included.
