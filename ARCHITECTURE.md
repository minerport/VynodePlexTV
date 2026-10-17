# Afterglow TV — architecture

An independent, self-hosted television experience powered by Plex. This repository has no relationship to or dependency on VyNode Media. Phase 0 established the foundation; Phase 1 adds real Plex inventory services. No schedule or playback behavior is simulated.

## Repository

```text
cmd/server/                     process startup and graceful shutdown
internal/config/                environment configuration
internal/database/migrations/   embedded, ordered SQL migrations
internal/httpapi/               versioned routes, auth, middleware
internal/plex/                  Plex adapter, encrypted server service, inventory/jobs/cache/linking
internal/scheduler/             application-owned future scheduling interface
web/src/                       React TV shell and API client
web/e2e/                       browser functional tests
Dockerfile                     reproducible frontend/backend/runtime stages
compose.yaml                   persistent /config deployment
```

## Boundaries

The Go process owns authentication, authorization, configuration, SQLite and all business logic. React owns presentation, input/focus behavior, and explicit loading/empty/error states. It accesses only `/api/v1`; it never reads SQLite or receives saved Plex tokens. Manual credential entry is submitted once through a password field, while account linking returns only a short-lived Plex authorization URL and sanitized server choices. The production process serves the built SPA and API from one origin. Vite proxies `/api` in development. Native clients can use the same versioned JSON API; their authentication transport will be specified before implementation.

Phase 0 uses standard-library HTTP, modernc SQLite, bcrypt passwords, opaque cookie sessions and structured JSON logging. Session tokens are generated cryptographically; only SHA-256 token digests are stored. Sessions expire after 24 hours. Every authenticated mutation requires an exact configured Origin and a per-session CSRF token. Logout revokes the session. Setup creates the first OWNER transactionally and becomes unavailable afterward. Login attempts are rate limited. Public responses never contain password hashes or session tokens.

Roles are OWNER, ADMIN and USER. Only OWNER/ADMIN may change application settings. Account creation/management beyond first-owner setup is deferred. First-run setup is available to whoever can reach the service, so complete setup on a trusted host before exposing it to the LAN. TLS termination and secure cookies are required when exposed beyond a trusted LAN.

## Storage and migration strategy

SQLite lives at `/config/afterglow.db`, with WAL, foreign keys, a busy timeout, and a single connection for predictable transactional writes. Migrations are embedded numbered SQL files applied transactionally. A `schema_migrations` ledger stores version and checksum: edited applied migrations or a database newer than this binary fail startup. New schema changes require new migrations, never edits to deployed files. Back up the database using SQLite's backup facilities or stop the container before copying the entire config directory. Downgrades require a matching backup; no destructive automatic down migrations.

Migration 001 contains only users, user_preferences, sessions, app_settings and audit_log. Later migrations add plex_servers, plex_libraries, plex_media_items, plex_media_versions; channels, channel_sources, channel_rules, channel_groups; channel_schedules, schedule_blocks, program_entries; viewing_sessions, playback_history, favorites; interstitials and channel_interstitial_rules. Avoid empty speculative tables in the first migration. Domain IDs and Plex source IDs will remain distinct. Canonical times are UTC Unix seconds; UI rendering uses an IANA timezone setting. Future block scheduling must explicitly resolve DST gaps and folds.

## Plex boundary (Phase 1)

`internal/plex.Provider` now expresses identity, complete library discovery, bounded pages, detailed normalized metadata and artwork. `Client` alone understands XML, Plex endpoints and headers. `Service` owns server/library persistence, inventory, jobs and cache. HTTP handlers call the service and enforce admin authorization; raw Plex XML and credentials never reach React. The future scheduler remains untouched and independent of these transport models.

Migration 002 adds server/library/media tables, unified metadata entities plus typed relationships, external IDs, normalized versions/parts/streams, jobs, observed-item markers, Plex settings and artwork cache metadata. Metadata entity identity is scoped to Plex server and kind plus a Plex ID when supplied. Missing IDs fall back to server/kind/name identity; this is not a claim that names are globally unique. People use one entity kind with actor/director/writer relationships. Hierarchy retains source parent/grandparent keys with composite indexes. Filesystem paths are internal only. Unsupported library types remain discoverable but cannot be selected.

AES-256-GCM ciphertext is bound to server ID and key version. The separate persistent key and client identifier are generated once in /config/secrets. Missing encryption material with existing credentials prevents startup. Account linking uses Plex's strong PIN flow; it is opt-in per administrator, holds secrets only in bounded memory, expires in at most ten minutes and validates a resource's machine identity before storing its server-specific token. Cloud account access is needed only during optional account linking; direct local operation and manual setup do not depend on a hosted Afterglow service.

One durable worker claims queued jobs and performs sequential, paginated I/O. A scan validates server identity, library UUID/type/revision, explicit totals, offsets and unique observations. Changed metadata is committed per page. Incremental scans still enumerate all identities, skipping detail requests only when source timestamps match. Final missing-item marking, library timestamps and SUCCEEDED status commit in the same transaction after checking cancellation. Failed/cancelled/interrupted work cannot reach that transaction. Restart cancels RUNNING jobs and clears their observation markers; queued work remains durable. Outages retain inventory, and reconnection restores health through scheduled checks or successful sync.

Artwork paths are restricted to relative Plex library paths. Redirects and unexpected content types are rejected. A serialized cache fill avoids duplicate downloads and eviction races; LRU eviction enforces a configurable byte budget. Authentication precedes even browser conditional-image responses. Cached images can survive temporary server failure without implying media availability.

Plex calls use normal TLS verification, five-second connection/TLS limits and twenty-second request limits with at most one short retry for transport/server errors. Authentication failures are not retried. Public errors are fixed safe messages; raw URL errors and upstream bodies are never logged. Frontend status polling is intentionally simpler than introducing SSE without a current need. Real Plex validation remains a distinct gate from deterministic HTTP fixtures.

The transport recognizes only canonical IPv4 plus 32-hex certificate identifier `plex.direct` names for a DNS-failure fallback to the embedded address. The original hostname remains authoritative for TLS SNI, certificate verification and HTTP Host. Other hosts, invalid addresses and non-DNS failures are not rewritten; no public DNS dependency or global resolver change is introduced.

## Scheduler boundary (future phases)

`internal/scheduler.Generator` accepts normalized content, a channel revision/seed, and a UTC horizon. It returns deterministic entries independent of Plex HTTP structures. A persistence service will transactionally append schedules, preserve already-generated unaffected entries, and enforce non-overlap/idempotence. Target retention is seven days past and fourteen days future. Clock injection makes lookup and join-in-progress offsets independently testable. Advanced blocks, timezone-aware slots and interstitial policy are separate later extensions; Phase 0 does not implement scheduling.

## Deployment and operations

A single non-root container serves port 8080 and stores state only under /config. Compose maps a named volume to /config and defaults to loopback access for safe initial setup. Health reports process liveness; readiness checks the database. Version is build-injected. JSON logs support DEBUG/INFO/WARN/ERROR and never log request bodies, cookies or query strings. Administrative setup/settings/server/library/job changes are audited. The OpenAPI document describes implemented routes only. The Plex worker shuts down with the process; its final cancellation is persisted before SQLite closes.

## Channel boundary (Phase 2)

`internal/channels` owns channel/group persistence, source validation, a parameterized rule compiler, deterministic membership resolution, metadata option queries and custom artwork storage. It imports no Plex service or HTTP response types. HTTP routes enforce session, role, Origin and CSRF boundaries, then call this service. React components under `web/src/channels` edit typed definitions and present paginated results; membership predicates and eligibility decisions remain server-side.

Migration 003 adds `channels`, `channel_groups`, `channel_sources`, `channel_static_items` and `channel_uploads`, plus safe audit target/summary columns and metadata lookup indexes. Sources are discriminated structured configurations in `channel_sources.config_json`. Static IDs/positions are normalized in `channel_static_items`; they are not duplicated in JSON. Rule version 1 is stored inside the source configuration, avoiding a redundant rule table. Existing metadata and show relationships remain authoritative. No schedule or program-entry tables are added.

`ResolveChannelMedia`, `CountResolvedMedia` and source previews share a compiler and centralized playable/eligibility predicates. Counts use SQL aggregation rather than loading result objects. Previews count and read one bounded page within a read transaction. Static order is explicit; show order is numeric season/episode; other sources use title/year/local ID with stable ties. Consumers receive normalized `channels.Media`, never raw Plex records. A later scheduler can consume these identities and durations but must introduce its own persistence, seeds, revisions and ordering policy.

The versioned rule AST has group nodes (`AND`, `OR`, one-child `NOT`) and condition nodes (`field`, `operator`, typed `value`). Field/operator allowlists map to fixed SQL fragments; all values use bound parameters. Depth, node, input-size and value-count limits bound work. References must exist in their correct domain. Collection references combine local entity ID and local library ID; labels include server/library context. A metadata selection lookup retrieves saved labels directly, without fetching every actor or collection.

Reference/enum operators are `is`, `is_not`, `in`, `not_in`. Strings support equals/not-equals, contains/not-contains, starts/ends-with. Numbers support equals/not-equals, greater/less comparisons (including equality), and inclusive between. Dates support before/after/between/within-last-days; dates normalize to UTC, and the relative-date clock is injectable. Availability supports is-true/is-false. Fields: library, server, media_type, title, show, season, genre, year, actor, director, writer, studio, collection, label, content_rating, rating, audience_rating, added_at, originally_available_at, duration and availability. `GET /channel-rules/fields` supplies the UI contract. Literal LIKE metacharacters are escaped. Positive NULL comparisons resolve false; negative conditions and NOT complement those results.

Genre/person/label/collection matching includes metadata attached to an episode's parent show, preserving normalized Plex hierarchy without duplicating relationships. Membership uses indexed item/metadata lookups. A correlated OR initially caused broad metadata scans; an indexed IN lookup over the item and its parent show reduced measured previews from seconds to tens of milliseconds.

Channel changes and their audit records commit together. Deletion never deletes Plex inventory. Uploads are content-addressed, fully decoded and bounded before storage, atomically renamed, and never accept user-supplied paths. Uploads and channel artwork are authenticated admin endpoints. The HTTP adapter may ask the existing Plex artwork service for a chosen media image; resolution itself has no network dependency. Artwork failure falls back to an escaped generated SVG; clients never receive a Plex token.
