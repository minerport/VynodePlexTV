# Implementation plan

## Phase 1 — authorized continuation

Baseline: cf65b5b3e5f315f2d0f2e4aa3f84852d29c8f7b6. Clean tree, documentation, migration/auth/settings code and existing tests were inspected before implementation. Phase 0 tests passed. Two incremental gaps were identified: its Plex interface was skeletal and it had no background runner. Expand the provider and add a durable single-worker queue without rewriting existing authentication or migration 001.

Implemented scope: encrypted server connections and stable client identity; Plex account PIN sign-in (explicitly requested during implementation), manual fallback and resource/server selection; connection health; library discovery/selection; normalized movie/show/season/episode inventory and relationships; full/incremental scans with safe final reconciliation; jobs, cancellation, scheduled refresh and bounded artwork cache; admin UI and media search; upgrade/security/failure/browser tests; OpenAPI and documentation.

Validation gates: Phase 0 upgrade, automated backend/frontend checks, Docker browser journey, real account linking/server identity/discovery/movie+TV sync/artwork/repeat sync/outage/recovery, container restart and security/diff review. Record separate evidence and any blocked live checks in VALIDATION.md before claiming completion. Preserve the Phase 0 image/config backup through validation.

Status: these gates passed on 2026-10-17. Real inventory contains 3,506 movies, 225 shows, 568 seasons and 6,373 episodes. Live DNS/sort compatibility issues and the TV browser grouping correction are documented in VALIDATION.md. The pre-upgrade backup remains available.

Phase 2 remains unstarted. Its exact recommended scope is Virtual Channels and Rule Engine: channel/group CRUD, static/show/collection sources and friendly typed AND/OR/NOT filters over the synchronized local model. Keep deterministic scheduling, smart-random playback ordering, EPG generation and streaming as later work unless separately authorized.

## Phase 0 — authorized scope

1. Inspect the empty standalone directory; document architecture and boundaries before implementation.
2. Initialize Go/React projects, configuration, embedded SQLite migrations and service lifecycle.
3. Implement atomic owner setup, password authentication, expiring server-side sessions, CSRF/origin checks, roles, audit records and settings.
4. Build a cinematic responsive TV shell, deterministic keyboard focus and honest Live TV/Guide/Channels/Plex Setup empty states.
5. Add health/readiness/version routes and OpenAPI documentation.
6. Package with non-root Docker, Compose and persistent /config; document Windows/container development and Unraid installation.
7. Run backend security/database tests, frontend tests/lint/typecheck/build, browser functional tests, Docker build and actual startup checks. Validate owner setup, login/logout, navigation, settings, and persistence after restart.
8. Record evidence and limitations and commit Phase 0. Do not automatically start Phase 1.

## Subsequent phases

1. Plex: encrypted credential storage, manual connection + health, discovery fallback, library selection, normalized paginated metadata/artwork, full/incremental sync and meaningful job state. Validate against a real configured development Plex server. No playback or schedules yet.
2. Channels: CRUD, groups, static/show/basic visual rule sources and artwork.
3. Scheduler: independently tested deterministic sequential/random/smart-random schedules, persisted UTC entries and horizon maintenance.
4. Playback: real Plex playback resolution, current program/offset, restart, channel overlay/surfing and explicit failure states.
5. EPG: timeline, now marker, details, past access, filtering and remote focus/virtualization.
6. Advanced programming: blocks, recurring slots, primetime, repetition and seasonal rules.
7. Experience: mini-guide, favorites, discovery scoring, last channel/startup preferences.
8. Interstitials: optional bumpers, available Plex trailers, custom video and next-up clips.
9. Multiuser/polish: user administration, mapping, kids restrictions, analytics and performance.

Each phase requires tests, formatting/linting and real functional verification. Missing external credentials or devices must be reported as unvalidated, never replaced by production mocks.

## Phase 2 — authorized channel membership implementation

Phase 1 baseline 36ed7266541cc9bc236c13732c06a319df94309e was clean; backend/frontend tests and frontend lint/format/build were checked before implementation. Add migration 003 and a separate internal/channels package without rewriting Plex sync. Domain models, a versioned typed expression tree, strict validator/compiler, paginated resolver/count service, metadata selectors, channel/group persistence and artwork upload handling are separate from HTTP and React. Default resolution includes playable movies/episodes in selected/present libraries with available media; offline server status does not change eligibility. Static references survive unavailability. Episode metadata rules may inherit show relationships, documented and tested explicitly.

UI work replaces only the Channels placeholder: channel cards, source-specific editor, nested rule builder, paginated pickers/previews, groups and artwork. Reuse session/Origin/CSRF and focus behavior. Back up Phase 1 config/image, test migration and all existing behavior, validate representative channels against actual synchronized inventory, measure queries, test offline resolution and Docker persistence, document and commit. Do not implement schedules, EPG or playback.

### Phase 2 implementation status

Implemented migration 003, the isolated local channel service, typed static/show/collection/rule sources, bounded visual rule editing, indexed previews/counts, groups, duplication, enable/disable, artwork uploads/fallbacks, authorization and transactional audit records. Preserved Phase 0/1 behavior. Six real-inventory channels were created and browser-tested; independent SQL verified counts. Container restart, database upgrade and simulated Plex outage/recovery passed. Final commands and evidence are recorded in VALIDATION.md.

### Next authorized boundary

At the Phase 2 checkpoint, the next recommended scope was: deterministic linear schedule generation, persisted program entries, sequential/random/smart-random order, no-repeat windows, configurable horizon, stable seeds/revisions, idempotent extension jobs, block-scheduling foundation and schedule preview. Keep actual playback/EPG/channel surfing outside that phase unless separately authorized.

## Phase 3 — authorized scheduling implementation

Clean baseline 9c190bcee037ec37f2d367e6875af5bc1f8f06ca inspected; 28 backend and 19 frontend tests and Docker build passed. Preserve Phase 2 resolver and extend it with a bounded normalized membership snapshot, without rule duplication. Add migration 004, persisted millisecond UTC timelines/settings/checkpoints/history and scheduler jobs in the shared durable queue. Separate queue consumers by job type so Plex never claims scheduling jobs. Pure deterministic policies feed transactional extension/repair with a protected current/past boundary. Add admin settings/day preview/diagnostics, authenticated enabled-channel reads, extensive deterministic/time/repair tests and real six-channel upgrade, outage and restart validation. No playback or full EPG.

### Phase 3 implementation status

Implemented and validated migration 004, deterministic policies, persisted UTC-millisecond timelines/settings/history/checkpoints, current/next/previous queries, safe suffix repair/regeneration, retention/extension, isolated bounded scheduler jobs, diagnostics and the scheduling studio. Existing Phase 0–2 behavior passed the complete regression suite. Six real channels, precise timeline integrity, seed changes, membership repair, outage recovery, container restart/recreation, actual artwork and upgrade preservation were validated. See VALIDATION.md for counts, timings and limits.

### Next phase — explicit authorization required

Phase 4: **Linear Playback and Live Channel Tuning**. Reuse persisted program-at-now/current/next queries; resolve the referenced local media through the Plex service, compute live offsets, orchestrate supported Plex URLs/sessions, handle browser capabilities, add Watch Live/Watch From Beginning, player/current-next overlay, channel switching and explicit unavailable/offline/transcode failure states. Preserve the scheduler's existing current/past stability contract. No Phase 4 code has been started.

## Phase 3.1 and Phase 4 — authorized continuation

Inspect clean b63c502 and preserve existing schedules/inventory. First add explicit Recently Added system templates backed by ordinary persisted RULE channels with typed recent-window configuration, and opt-in generic Smart Random recency/show-distribution controls. Keep default-off policy extensions fingerprint-compatible with Phase 3. Validate distinct movie/episode membership, real counts, horizons, integrity, outage and restart before playback work.

Then add an isolated playback service consuming persisted scheduler queries and a Plex transport adapter. Use authenticated same-origin streaming so Plex tokens remain server-side; support direct compatible files and Plex HLS where needed. Add per-user/client sessions, lifecycle/heartbeats, live/start/go-live offsets, full-screen viewer, boundary transitions, channel switching, local On Now metadata, admin active viewers, tests and real browser validation. Do not implement the full EPG, mini-guide, external IPTV, recording, recommendations or interstitials. Add new migrations without changing applied files. Retain the Phase 3 image/config backup until validation completes.

## Phase 4 completed

Phase 3.1 and Phase 4 are implemented and validated; see VALIDATION.md for real inventory, decoded playback, offsets, transition, outage and restart evidence. The earlier Phase 3 next-phase paragraph is historical. No Phase 5 implementation has started.

Next authorized scope should be **Phase 5: Full Electronic Program Guide and TV Browsing Experience**: virtualized channel/time grid, current-time line, past/future browsing, program details, tune/catch-up actions, remote focus, groups/favorites and mini-guide foundation. Preserve persisted schedule authority and authenticated playback proxy. Await explicit instruction.
