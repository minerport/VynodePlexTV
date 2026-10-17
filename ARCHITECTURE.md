# Afterglow TV — architecture

An independent, self-hosted television experience powered by Plex. This repository has no relationship to or dependency on VyNode Media. Phase 0 establishes the runnable foundation; no Plex, schedule, or playback behavior is simulated.

## Repository

```text
cmd/server/                     process startup and graceful shutdown
internal/config/                environment configuration
internal/database/migrations/   embedded, ordered SQL migrations
internal/httpapi/               versioned routes, auth, middleware
internal/plex/                  application-owned Phase 1 interface
internal/scheduler/             application-owned future scheduling interface
web/src/                       React TV shell and API client
web/e2e/                       browser functional tests
Dockerfile                     reproducible frontend/backend/runtime stages
compose.yaml                   persistent /config deployment
```

## Boundaries

The Go process owns authentication, authorization, configuration, SQLite and all future business logic. React owns presentation, input/focus behavior, and explicit loading/empty/error states. It accesses only `/api/v1`; it never reads SQLite or receives Plex secrets. The production process serves the built SPA and API from one origin. Vite proxies `/api` in development. Native clients can use the same versioned JSON API; their authentication transport will be specified before implementation.

Phase 0 uses standard-library HTTP, modernc SQLite, bcrypt passwords, opaque cookie sessions and structured JSON logging. Session tokens are generated cryptographically; only SHA-256 token digests are stored. Sessions expire after 24 hours. Every authenticated mutation requires an exact configured Origin and a per-session CSRF token. Logout revokes the session. Setup creates the first OWNER transactionally and becomes unavailable afterward. Login attempts are rate limited. Public responses never contain password hashes or session tokens.

Roles are OWNER, ADMIN and USER. Only OWNER/ADMIN may change application settings. Account creation/management beyond first-owner setup is deferred. First-run setup is available to whoever can reach the service, so complete setup on a trusted host before exposing it to the LAN. TLS termination and secure cookies are required when exposed beyond a trusted LAN.

## Storage and migration strategy

SQLite lives at `/config/afterglow.db`, with WAL, foreign keys, a busy timeout, and a single connection for predictable transactional writes. Migrations are embedded numbered SQL files applied transactionally. A `schema_migrations` ledger stores version and checksum: edited applied migrations or a database newer than this binary fail startup. New schema changes require new migrations, never edits to deployed files. Back up the database using SQLite's backup facilities or stop the container before copying the entire config directory. Downgrades require a matching backup; no destructive automatic down migrations.

Migration 001 contains only users, user_preferences, sessions, app_settings and audit_log. Later migrations add plex_servers, plex_libraries, plex_media_items, plex_media_versions; channels, channel_sources, channel_rules, channel_groups; channel_schedules, schedule_blocks, program_entries; viewing_sessions, playback_history, favorites; interstitials and channel_interstitial_rules. Avoid empty speculative tables in the first migration. Domain IDs and Plex source IDs will remain distinct. Canonical times are UTC Unix seconds; UI rendering uses an IANA timezone setting. Future block scheduling must explicitly resolve DST gaps and folds.

## Plex boundary (Phase 1)

`internal/plex.Provider` expresses health, library discovery, paginated normalized media synchronization and artwork access using application-owned types. The HTTP/XML Plex adapter, token storage and source-specific parsing stay behind that interface. A provider is scoped to a configured server; every library and item retains its server ID and Plex source key. Only lightweight required metadata is persisted. Full sync uses a completion marker before deleting missing records; interrupted sync cannot delete valid media. Incremental scans periodically reconcile with a full scan. Authentication, retries, timeouts and errors belong to the adapter. Plex remains authoritative. Tokens must be encrypted at rest with a persistent key in /config before credentials are introduced. Real configurable development credentials are required to validate Phase 1; fixtures supplement rather than replace live checks.

## Scheduler boundary (future phases)

`internal/scheduler.Generator` accepts normalized content, a channel revision/seed, and a UTC horizon. It returns deterministic entries independent of Plex HTTP structures. A persistence service will transactionally append schedules, preserve already-generated unaffected entries, and enforce non-overlap/idempotence. Target retention is seven days past and fourteen days future. Clock injection makes lookup and join-in-progress offsets independently testable. Advanced blocks, timezone-aware slots and interstitial policy are separate later extensions; Phase 0 does not implement scheduling.

## Deployment and operations

A single non-root container serves port 8080, stores state only under /config and requires no cloud service. Compose maps a named volume to /config and defaults to loopback access for safe initial setup. Health reports process liveness; readiness checks the database. Version is build-injected. JSON logs support DEBUG/INFO/WARN/ERROR and never log request bodies, cookies or query strings. Administrative setup/settings changes are audited. The OpenAPI document describes only implemented routes. Background jobs and SSE updates arrive with their actual consumers in subsequent phases.
