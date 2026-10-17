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

## Phase 3 — deterministic scheduling (2026-10-17)

Baseline: clean `9c190bcee037ec37f2d367e6875af5bc1f8f06ca`. Inspected the required architecture/implementation/validation documentation, migrations 001–003, authentication/settings, channel resolver/editor, Plex job lifecycle and time handling. Baseline 28 backend tests, 19 frontend tests and Docker build passed. Phase 2 image retained as `afterglow-tv:phase2`; stopped-container `/config` archive retained in ignored `.local/backups/phase2-config.tar.gz`. A consistent SQLite Phase 2 snapshot is retained in the isolated `afterglow_phase3_baseline` volume. No unrelated repository was touched.

### Automated checks

- Backend: **45 top-level tests / 110 cases including subtests passed** using Go 1.26 in `afterglow-backend-test`. `go vet ./...` and `gofmt` clean. Includes all Phase 0–2 tests.
- Scheduler coverage: exact static loops, show season/episode membership ordering, millisecond durations, continuation, seeded random cycles/coverage/immediate-repeat avoidance, same/different seeds, persisted Smart Random history, type-specific repeat windows, oldest-first relaxation, single/two/1,000-item channels, show/season tie preferences, cancellation, invalid durations and bounded-generation rollback.
- Time/repair coverage: UTC, midnight crossings, spring-forward 23-hour and fall-back 25-hour days; exact start/end and next/previous boundaries; future unavailable/duration/removal/addition/order/gap/overlap repairs; protected historical/current row IDs/times; title/artwork changes; pruned repeat history; disabled/empty channels; duplicate settings/no copied programs; cascade without inventory deletion; durable job success, interrupted-job recovery and Plex/scheduler queue isolation.
- API: authenticated reads, disabled-channel restriction, settings/generate/repair/regenerate authorization for OWNER/ADMIN vs USER, origin/session CSRF, invalid date/time handling and global diagnostics/jobs.
- Migration: actual Phase 2 schema plus owner/session/timezone/channel/source upgraded to migration 004; original data and stable generated seed preserved across a second migration pass. Existing upgrade/checksum tests also pass.
- Frontend: **24 tests passed**, including schedule list/details, episode context, date navigation, configuration, confirmation, indeterminate jobs, empty/error states, Escape navigation and timezone helpers. ESLint, Prettier checks and TypeScript/Vite production build passed.
- Docker production build passed: Go binary and Vite assets; non-root runtime, health check and persistent volume retained. Version **0.4.0**.
- Full Playwright owner journey: **1 end-to-end scenario passed (25.8 seconds test / 44.1 seconds including stack lifecycle)**. Real temporary Docker stack and mock Plex HTTP server exercise setup/login/invalid-login/logout, settings, remote focus, Plex library synchronization/artwork/restart, Phase 2 channels, Phase 3 generation/settings/repair/regeneration confirmation/day preview, and responsive layouts. Fixture metadata now includes realistic durations; the initial missing-duration fixture was correctly excluded by the scheduler rather than receiving an invented runtime. Test stack and its temporary volume were removed afterward.

### Real six-channel timelines

Generated using the existing synchronized inventory and actual runtimes; no production fixtures were introduced. The 90s channel was explicitly configured to Random to cover all three modes. All six source definitions, static positions and channel rows remained unchanged.

| Channel | Policy | Resolved items | Persisted programs at validation |
| --- | --- | ---: | ---: |
| 101 Handpicked Cinema | Sequential | 5 | 301 |
| 102 Show Spotlight | Sequential | 142 | 694 |
| 103 Action Movies | Smart Random | 1,284 | 274 |
| 104 90s Movies | Random | 64 | 277 |
| 105 Action & the 90s | Smart Random | 1,320 | 283 |
| 106 Collection Spotlight | Smart Random | 15 | 268 |

All cover at least 14 days ahead, with retained history starting October 10 and future coverage through October 31. Counts can change as normal retention/extension runs. Independent SQL over **2,097 persisted rows** found **0 gaps/overlaps, 0 invalid durations, 0 missing media references and 0 duplicate sequence positions**. Current/next/previous queries returned adjacent correct intervals for every channel. Real static ordering and show episode context were inspected in the browser.

The browser at `http://localhost:8080/#channels` was exercised at 1600×1000 and 768×1024: global studio, chronological preview, artwork decoding, settings/mode-dependent repeat controls, day navigation, Escape/back focus, editor schedule entry and invalid timezone editing. No JavaScript page errors or tablet horizontal overflow. Screenshots are private ignored evidence in `.local/phase3/` and were visually inspected; media artwork is not committed.

### Determinism, repair and offline evidence

- Recorded future program rows plus settings and fixed-timestamp current/next/previous for all six channels. Container restart and explicit subsequent generation jobs returned identical recorded segments.
- Final 0.4.0 container **force recreation** also preserved all settings, anchors, seeds, a five-day recorded interval and fixed-timestamp current/next/previous values exactly.
- Disposable real-inventory channel: duplicated settings but no programs, generated timeline, removed one static member and added another, repaired future. All historical/current IDs/times stayed identical; removed media disappeared from the repaired future and new media entered it. The disposable channel was deleted afterward; the original six remained intact.
- On the disposable channel, both Random and Smart Random with large real rule membership produced a different future media sequence after changing the seed and explicitly regenerating. Protected history/current entries remained identical.
- Simulated Plex outage by temporarily changing only this application's saved connection URL to an unreachable loopback endpoint. Actual health job failed. Scheduler UI, current lookup and extension from 14 to 15 future days continued using SQLite. Original URL restored in a `finally`-protected workflow; health succeeded using the same encrypted credential. No Plex server or media file was changed.
- Independent post-outage/post-upgrade SQL: **10,672/10,672 inventory identities and availability values unchanged**, same owner/password hash, existing sessions, application settings, encrypted credential/key version, five libraries, four groups, six channels, six source/rule definitions and five static memberships. Migration count is four. Config was never reset.

### Performance

Fresh generation was measured directly inside the Go service on an isolated copy of the real Phase 2 database (not HTTP queue latency). The benchmark performs local membership resolution, generation, transactional writes and retention. Results from this host:

| Channel members / policy | Entries generated before retention | Service duration |
| --- | ---: | ---: |
| 5 / Sequential | 302 | 10.631 ms |
| 142 / Sequential | 698 | 28.618 ms |
| 1,284 / Smart Random | 279 | 155.827 ms |
| 64 / Random | 281 | 12.458 ms |
| 1,320 / Smart Random | 275 | 154.496 ms |
| 15 / Smart Random | 269 | 15.988 ms |

All benchmark integrity checks passed. Measured HTTP enqueue→completion checks were approximately 0.6–2.3 seconds, including the two-second worker polling interval. These are single-host observations, not a throughput guarantee for arbitrary libraries.

### Security and limits

Reviewed scheduling paths for server-side RBAC, CSRF/origin protection, parameterized SQL, local-ID references, safe errors and absence of Plex transport/credentials. A private scan decrypted the existing Plex credential only in memory and checked source against that token and raw/hex/base64 encryption key representations: no matches. No tokens, keys, passwords, databases, caches, screenshots, diagnostic dumps or local configuration are added to source control. Private validation sessions are revoked after use. Backups remain ignored and outside the commit.

The current resource limits and behavior are explicit in README: one worker/SQLite connection, 50,000 members, 100,000 generated entries/job, seven-day maximum media duration, 0–30 past/1–30 future days, and no restoration of pruned historical rows when past retention is increased. Very old anchors/extremely short clips may hit the bounded entry budget and fail atomically. Progress is indeterminate until commit; running cancellation is best-effort. Smart Random uses scheduled—not watched—history, oldest-first with deterministic tie preferences. Small libraries necessarily relax windows. Random uses repeated seeded cycles. User-facing artwork remains a later viewer integration. No playback/full EPG/blocks/interstitials were implemented.

Final rescan check: real incremental Movie and TV jobs succeeded, processing **3,506** and **7,166** records respectively. Subsequent generation jobs preserved the recorded two-day timelines for all six channels byte-for-byte. This directly validates stable schedules through a real Plex rescan with unchanged schedule-relevant inventory.
# Phase 3.1 validation — 2026-10-17

Baseline `b63c502` was clean. A stopped-container `/config` archive and a consistent Phase 3 database snapshot were retained privately before migration 005. Existing backend tests and 24 frontend tests passed before implementation. Updated backend tests/vet/format checks pass, including lookback boundary/type isolation, canonical templates, distribution/coverage/continuation, relaxation, and aging/reappearance repair. Frontend: 25 tests pass; production build and lint pass.

Real UI-created channels 107 Recently Added Movies and 108 Recently Added TV use the default 30-day window: **31 movies; 394 episodes across 33 shows**. Generated 285 movie programs and 688 TV programs, both beyond 14 future days. No crossover, gaps, overlaps, invalid durations, duplicate positions, or missing references. TV has 29 same-show adjacent pairs among 687 boundaries; separation is a preference subordinate to episode repeat eligibility, rather than a guarantee when unseen/eligible alternatives are exhausted.

All eight channels retained identical sampled three-day future sequences after container restart and generation rerun. All six original fingerprints are identical. Snapshot held 2,099 programs (normal maintenance had extended the previously reported 2,097); 2,098 remain byte-identical and one legitimately aged outside seven-day retention. No baseline entry within retention was lost. All 10,672 inventory identities and availability values, owner, saved sessions/settings, encrypted credentials, five library selections, six original channels/sources, groups and static membership survived the upgrade.


# Phase 4 validation — 2026-10-17

Completed Phase 3.1 first, then linear playback. Version 0.5.0. Migration 005 adds opt-in recency/show-distribution settings; 006 adds application viewing sessions. Scheduling remains authoritative and Plex-specific transport stays behind the Plex adapter. No Phase 5 work was implemented.

## Automated and build evidence

- Backend: **60 top-level tests / 144 cases**, all passing; Go vet and gofmt passed.
- Frontend: **35 tests**, all passing; ESLint, Prettier, TypeScript and production Vite build passed.
- Existing Docker browser regression: **1 complete journey passed**, including owner authentication, remote navigation, Plex/channel/schedule administration and container persistence.
- Production Go binary and Docker image built; actual Compose stack healthy, ready and serving version 0.5.0.
- Dependency audit: zero npm vulnerabilities. Player/HLS is lazy-loaded; its 602 kB / 189 kB gzip chunk triggers Vite's advisory size warning.

## Real browser playback

Microsoft Edge, actual Plex media and decoded video frames were used, not mocked playback. Final-image observations:

| Channel/program | Observed Plex delivery | Requested offset | Actual position / live position | Browser startup observation |
|---|---|---:|---:|---:|
| 101 / 10 Cloverfield Lane | Direct Stream via HLS | 5240.827 s | 5244.341 / 5245.016 s | 4.67 s |
| 102 / Pay It Forward | Direct Stream via HLS | 263.583 s | 267.121 / 267.253 s | 4.40 s |
| 103 / Dune | Transcode via HLS | 6524.072 s | 6529.114 / 6529.422 s | 5.90 s |

Startup observations include a deliberate three-second decoded-playback observation interval; they are not precise time-to-first-frame benchmarks. Plex decision responses identify Direct Stream versus Transcode. A temporary static validation channel also decoded **Sing: Thriller** and **It Won't Hurt Too Much** using Direct Play MP4, then was deleted without changing inventory.

Movie and episode Watch From Beginning requested zero and played at approximately 1.7 seconds after observation. Go Live returned within 0.38 seconds (movie) and 0.26 seconds (episode) of the current schedule. Five-channel switching covered Static, Show, Rule, Recently Added Movies and Recently Added TV with matching persisted programs and real frames. Numeric entry, arrow channel switching, pause/behind-live and Escape exit passed.

The temporary channel's first slot ended at UTC epoch-ms 1792275672212. Automatic next-program tune occurred at 1792275672377: **165 ms after the boundary**, requesting 165 ms into the correct next persisted episode. Decoded frames confirmed transition. No timeline edits or unrelated replacement media were used.

## Persistence, integrity, outage and performance

Final comparison against the private Phase 3 snapshot: all **10,672 identities and availability values** preserved, along with owner/password hash, two existing login sessions, settings, encrypted Plex credential, five libraries, six original channel definitions/sources, four groups and five static members. Six migrations applied. All six original schedule fingerprints unchanged. Of 2,099 baseline program rows, 2,095 remained byte-identical at final verification; four legitimately expired under seven-day retention. **No baseline program within retention was lost.**

Final inventory had **3,068 programs across eight channels** after routine retention. Zero gaps/overlaps, duration inconsistencies, duplicate sequence positions, missing media references or Recently Added movie/episode crossover. Recently Added validation: **31 movies**, **394 episodes / 33 shows**, 14+ days coverage, deterministic restart/rerun; 29 same-show adjacent pairs among 687 initial TV boundaries. Repeat eligibility takes priority and distribution relaxes safely for limited alternatives.

Force-recreating the final container preserved all eight sampled future schedules byte-for-byte and cleared an intentionally active application viewing session. Real movie/episode playback subsequently succeeded with retained credentials. New anchors/settings/seeds/channel identities persist.

A reversible application-only unreachable Plex address simulated outage; no external Plex service was stopped. All eight On Now entries and local schedule administration/generation remained available. Player showed a sanitized connection failure; restoring the stored address and pressing Retry resumed decoded video. Inventory availability stayed unchanged.

Five tune API resolutions measured **1320, 54, 64, 65, 52 ms** (first cold request versus subsequent warm requests, including Plex negotiation). Ten local eight-channel On Now queries measured **8–12 ms**. These are local development observations, not load-test guarantees.

## Security and limitations

Session ownership, server-side viewer/admin authorization, CSRF/Origin validation, disabled-channel rejection, bounded upstream resources, Range handling, HLS URI allow-listing and token redaction have tests. Browser URLs contain opaque application resource IDs, never Plex tokens. Credentials remain encrypted and server-side. Repository scan against the actual decrypted Plex token and encryption-key encodings found no matches. Private databases, caches, credentials and diagnostics remain ignored. Validation login is revoked after use.

During troubleshooting, an unrelated webhook API key from a Plex diagnostic log appeared in tool output. The user was informed that it should be rotated. It was not copied into source or this report. Subsequent diagnostics used specific whitelisted playback fields.

Current limits: conservative single-part MP4/H.264/AAC Direct Play; otherwise Plex decision/HLS, max 1080p. Manual quality/version/subtitle selection and multipart playback are deferred. Browser autoplay may require Play. Native TV hardware, Safari, remote WAN behavior and household load testing were not validated. Repeat/show separation is a preference when windows cannot all be satisfied. Recently Added windows are rolling UTC durations. The full EPG, past-program catch-up browsing, mini-guide, favorites and richer TV browsing belong to Phase 5. No claim is made that every Plex codec/version can play.

## Phase 4 changed files

- `ARCHITECTURE.md`
- `Dockerfile`
- `IMPLEMENTATION_PLAN.md`
- `README.md`
- `VALIDATION.md`
- `cmd/server/main.go`
- `internal/channels/artwork.go`
- `internal/channels/recent.go`
- `internal/channels/recent_test.go`
- `internal/channels/resolver.go`
- `internal/channels/schedule.go`
- `internal/channels/service.go`
- `internal/channels/types.go`
- `internal/database/database_test.go`
- `internal/database/migrations/005_recent_channels.sql`
- `internal/database/migrations/006_playback_sessions.sql`
- `internal/httpapi/api.go`
- `internal/httpapi/channels.go`
- `internal/httpapi/playback.go`
- `internal/httpapi/playback_test.go`
- `internal/httpapi/plex.go`
- `internal/playback/service.go`
- `internal/playback/service_test.go`
- `internal/playback/sessions.go`
- `internal/playback/types.go`
- `internal/plex/metadata.go`
- `internal/plex/playback.go`
- `internal/plex/playback_test.go`
- `internal/plex/types.go`
- `internal/scheduler/distribution.go`
- `internal/scheduler/distribution_test.go`
- `internal/scheduler/generator.go`
- `internal/scheduler/service.go`
- `internal/scheduler/service_test.go`
- `internal/scheduler/types.go`
- `openapi.yaml`
- `scripts/update-playback-openapi.mjs`
- `web/package-lock.json`
- `web/package.json`
- `web/src/App.tsx`
- `web/src/channels/ChannelEditor.tsx`
- `web/src/channels/ChannelsPage.tsx`
- `web/src/channels/api.ts`
- `web/src/channels/channels.test.tsx`
- `web/src/playback/Player.tsx`
- `web/src/playback/ViewerHome.tsx`
- `web/src/playback/api.ts`
- `web/src/playback/controller.ts`
- `web/src/playback/playback.test.tsx`
- `web/src/playback/player-ui.test.tsx`
- `web/src/playback/player.css`
- `web/src/scheduler/ScheduleAdmin.tsx`
- `web/src/scheduler/api.ts`

## Phase 5 — Full EPG and TV browsing (2026-10-17)

Baseline: clean Phase 4 commit `46c2564515c13c75948fb1957cd0a1e273f33cf4`. Read the architecture, plan, validation, migrations 001–006, channel/scheduler/playback boundaries and frontend focus/player code before implementation. Baseline backend (60 tests / 144 cases), frontend (35 tests), production builds and Docker passed. A stopped-container /config backup and Phase 4 image were retained privately before migration; no /config reset.

### Implementation and upgrade

Migration 007 adds per-user favorites, last-channel preferences and a compatible playback origin field. The guide is an indexed SQLite-only domain with bounded window queries, batch metadata, half-open intersections, server time, group/favorite filters and defensive interval handling. Catch-up resolves the retained scheduler program ID, starts at zero and never changes broadcast history. Future, unavailable, hidden and disabled program access is rejected. Existing scheduler and Plex transport behavior remain intact.

The TV guide includes a frozen channel column, proportional program widths, virtualized rows, three-hour windows, past/future day navigation, Now line, details, group/favorite filtering and remote focus memory. Player mini-guide, full guide and details overlays retain the existing player/session. Home includes On Now, personal favorites, Starting Soon, last channel, groups and both Recently Added networks.

Real upgrade comparison preserved owner, existing authentication sessions/settings, encrypted Plex credentials, five libraries, all 10,672 media identities/availability values, four groups, eight channels, rules and five explicit static members. All eight schedule fingerprints were unchanged. Of 3,070 snapshot programs, 3,068 remained identical; two were legitimately pruned beyond configured past retention. No retained historical/current row was lost. Subsequent maintenance produced 3,071 programs. Integrity checks: zero gaps, overlaps, invalid durations, duplicate positions or missing media references. Recently Added remained 31 movies and 394 episodes from 33 shows, without type crossover.

### Automated validation

- Backend: 68 top-level tests / 152 cases passed; Go vet and gofmt passed. Includes guide intersections, classification/progress, groups/favorites, disabled/empty behavior, details, timezone/DST windows, user isolation, last channel and catch-up identity/authorization/history safety; all earlier tests retained.
- Frontend: 48 tests across nine files passed; ESLint, Prettier, TypeScript and Vite production build passed. Tests cover guide geometry/state, directional focus/day/Now navigation, details/actions, filters/favorites, mini-guide, home and existing playback.
- Production Go binary and Docker image 0.6.0 built successfully; actual Compose stack healthy/readiness passing.
- Isolated Docker browser regression: one complete owner journey passed (43.1 seconds), including setup, login, remote navigation, logout and container persistence; isolated test volumes removed afterward.
- Real layout regression found padding widened a clipped 1.55-pixel program to 26 pixels. Fixed narrow-cell padding and used an inset border; browser measurements now match timeline widths, including narrow edge fragments.

### Real browser and viewer validation

Validated all eight enabled channels together: static, show, collection, rule, decade/nested rules, Recently Added Movies and Recently Added TV. Desktop guide, horizontal timeline, frozen channel column, vertical arrow scrolling, Now line, future details and past/current actions passed. Seven downward moves preserved program focus and scrolled 357 pixels. Returning from playback restored the exact selected program. At 390×844, no page overflow; selected day remains visible after resizing. Artwork decoded on home cards and details. Large-catalog virtualization is covered structurally/tests; real validation used the eight configured channels, not hundreds of synthetic channels.

Completed Home → Guide → Watch Live → mini-guide → browse five channels → tune → full Guide → past movie/episode details → Watch From Beginning → Go Live → Exit → Favorites → Starting Soon → Last Channel. Mini-guide switching left exactly one active application viewer session; full guide did not spawn a second stream. Decoded video frames verified actual playback rather than only successful API responses.

Past movie catch-up requested zero and observed position 0.146 seconds; past episode observed 0.008 seconds. Both used retained past program identities. Go Live returned to current broadcast with decoded video. Recently Added Movies current restart observed 0.140 seconds; Recently Added TV 0.006 seconds, both requested zero and subsequently tuned live successfully. Future details offered no incorrect playback action. Existing live-offset and stream negotiation tests remained passing; Phase 4's Direct Play/Direct Stream/transcode evidence remains recorded above.

Two owner favorites persisted and filtered correctly. A temporary USER began with no favorites, favorited its own channel, did not alter the owner's favorites and received 403 on admin active-session access. Validation user/session material was private and removed after checks.

### Offline, persistence and performance

Temporarily changed only this application's Plex endpoint to an unreachable loopback port, then restored it in a finally path. Did not change Plex itself or inspect Plex diagnostic logs. Guide past/current/future data, details, On Now, Starting Soon, favorites and mini-guide metadata continued working from SQLite. Playback failed cleanly; after restoration Retry produced decoded video. Inventory identities/availability remained unchanged.

Recreated the real container. Compared a sampled three-hour guide window: program identities, media and timestamps identical. Favorites and exact last-channel preference persisted; schedules, owner and usable encrypted Plex connection remained intact. Authentication and playback worked again. No extra playback stream was created by browsing overlays.

Ten local HTTP samples, eight real channels (includes HTTP overhead; not load-test promises):

| Query | Observed latency |
| --- | --- |
| Three-hour guide | 8.8–11.7 ms |
| Current/next | 11.1–13.7 ms |
| Starting Soon | 7.2–9.5 ms |
| Program details | 6.6–8.0 ms |

Initial local browser guide render measured about 190 ms. A first current/next implementation was slower; selecting two rows per channel before metadata joins removed the unnecessary full-horizon work. No additional guide cache introduced. Static data refreshes every 30 seconds; the server-synchronized Now line updates separately.

### Security, limitations and scope

Source scan compared 148 source files against actual Plex credential material in memory without printing it: passed. No private database, cache, key, cookie, token, screenshot or diagnostics file is tracked. Normal APIs/artwork keep Plex credentials server-side; existing Origin/CSRF/RBAC checks apply. Favorites always derive user identity from the authenticated session. Catch-up performs the same visibility/media checks as live tuning.

An unrelated previously exposed webhook key remains unrotated because its owning service administration location has not been provided. No old secret was retrieved, reused, persisted or included in this phase, and the broad diagnostic method that exposed it was not reused. This external follow-up remains outstanding; it is not represented as completed.

Limits: guide queries are capped at 24 hours, 100 channels and 10,000 programs per request; UI uses three-hour windows and 50-channel pages. Mini-guide and Starting Soon currently request the first 100 channels. Very narrow clipped cells are intentionally unreadable until focused/details opened. Retention bounds catch-up availability. No physical TV remote, Safari, WAN or household load validation was performed. Existing conservative single-part/1080p playback and no manual subtitle/version controls remain. Vite reports the existing lazily loaded HLS player bundle size advisory. No full native client, DVR, external IPTV or Phase 6 interstitial features were implemented.

### Files changed

New: migration 007; `internal/guide/service.go` and tests; `internal/httpapi/guide.go`; `web/src/guide/` API, timeline, grid/cells, dialogs, details, mini-guide, CSS and tests; OpenAPI update script.

Updated: database migration tests; HTTP API registration/playback and tests; playback service/types and tests; scheduler retained-program lookup; App/focus; player/controller/API/home and player UI tests; browser foundation regression; OpenAPI; Docker/package version; README, ARCHITECTURE, IMPLEMENTATION_PLAN and this validation record.

Recommended next scope, requiring a new instruction: Phase 6 Broadcast Presentation and Interstitials — optional bumpers/trailers/custom clips, coming-up-next, per-channel branding/presentation rules and deterministic interstitial placement/transition presentation. Phase 6 has not started.
