# Implementation plan

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
