# Phase 3 completion report

Phase 3 adds a persisted deterministic broadcast timeline to the standalone Afterglow TV application. Version 0.4.0 is running at http://localhost:8080. Open Channels → Scheduling studio. No Phase 4 work was implemented.

1. **Implementation:** per-channel scheduling settings, exact-duration program rows, past/future coverage, extension, future repair/regeneration, diagnostics and background maintenance.
2. **Architecture:** a dedicated `internal/scheduler` domain consumes a normalized `internal/channels.ScheduleSnapshot` interface. It never calls Plex or recompiles rules. Plex and scheduling workers partition the shared durable job queue.
3. **Migration:** `004_linear_scheduling.sql` adds settings/programs/state/pruned-history tables plus scheduling job metadata and consistency indexes/constraints. Existing migrations are unchanged.
4. **Policies:** Sequential, deterministic Random, Smart Random. Static ordering and show season/episode ordering come from Phase 2 membership.
5. **Determinism:** persisted anchor, seed, sequence, fingerprints/revisions, history and absolute UTC millisecond rows. Repeated extension preserves valid rows. Random uses complete seeded cycles; Smart Random uses oldest-first persisted air history with deterministic tie breaks.
6. **Repeat windows:** separate movie/episode windows; outside-window preference, oldest non-immediate fallback, unavoidable single-item repeats. Show/season avoidance is a tie preference, not a hard exclusion.
7. **Repair/regeneration:** repair starts at the earliest affected future program; additions/source/settings/order changes invalidate the future suffix. Explicit regeneration replaces the entire future suffix. Both preserve every program that has started, including the current one. Atomic failure leaves the prior timeline intact.
8. **UI:** global studio, channel/editor entry points, chronological day preview, artwork and episode context, configurable settings, advanced seed/anchor, confirmation, job states, coverage and integrity diagnostics. Desktop/tablet and keyboard navigation validated.
9. **Tests:** 45 backend top-level tests / 110 cases passed; 24 frontend tests passed; complete Docker browser journey passed. Go vet, gofmt, ESLint, Prettier, TypeScript/Vite build, Go production build and Docker image build passed.
10. **Real channels:** all six existing channels scheduled from their real 5 / 142 / 1,284 / 64 / 1,320 / 15 members with at least 14 days of future coverage.
11. **Integrity:** 2,097 persisted programs at the recorded check; zero gaps, overlaps, invalid durations, missing references or duplicate sequence positions. Current/next/previous boundaries validated.
12. **Stability:** restart, force recreation, repeat jobs and real incremental Movie/TV rescans preserved recorded timelines. Changed seeds changed Random and Smart Random future sequences on a disposable real-membership channel. Membership repair preserved history/current while adding/removing future membership correctly.
13. **Performance:** fresh service generation on an isolated real-database copy took 10.6–155.8 ms per channel; large Smart Random channels took about 155 ms. HTTP queue completion observations were 0.6–2.3 seconds including worker polling.
14. **Offline Plex:** simulated unreachable connection produced a failed health job while schedule UI, lookup and extension remained functional. Recovery health passed after restoration. All 10,672 inventory identities/availability values survived.
15. **Upgrade/persistence:** owner/password hash, sessions/settings, encrypted credential, five libraries, four groups, six channels/source definitions and static membership preserved. No config reset. Anchors/seeds/programs persisted across restart/recreation. Prior image/config backup retained.
16. **Limits:** one bounded worker and existing single SQLite connection; 50,000 members and 100,000 entries/job; seven-day maximum item duration; past 0–30/future 1–30 days. Running cancellation is best-effort and progress indeterminate until commit. Old pruned history cannot be recreated by increasing retention. Small Smart Random libraries must relax windows; seed changes primarily affect ties. Viewer artwork authorization, full EPG, blocks/slots and playback remain future work.
17. **Files:** see the file inventory below. No private data/cache/config/credentials are committed; source was checked against the actual Plex token and encryption key in memory.
18. **Source control:** completed-phase commit hash is supplied in the final handoff. No push or publication was requested.
19. **Exact next scope:** Phase 4 — Linear Playback and Live Channel Tuning: program-at-now lookup, local-media/Plex resolution, join-in-progress offset, Watch Live/Watch From Beginning, Plex playback URL/session orchestration, browser capabilities, player UI, current/next overlay, fast channel switching and explicit playback failure states. Requires explicit authorization.

## File inventory

New: `internal/database/migrations/004_linear_scheduling.sql`; `internal/channels/schedule.go`; `internal/scheduler/{types,service,queries,jobs,generator_test,service_test}.go`; `internal/httpapi/{scheduler,scheduler_test}.go`; `web/src/scheduler/{api.ts,ScheduleAdmin.tsx,schedule.css,scheduler.test.tsx}`; `web/e2e/schedule-journey.ts`; `scripts/update-scheduler-openapi.mjs`; this report.

Modified: `internal/scheduler/generator.go`; `internal/channels/{service,channels_test}.go`; `internal/database/database_test.go`; `internal/httpapi/api.go`; `internal/plex/jobs.go`; `cmd/server/main.go`; `web/src/channels/{ChannelsPage,ChannelEditor}.tsx`; `web/src/focus.ts`; `web/e2e/{fixture.mjs,foundation.spec.ts}`; `web/{package.json,package-lock.json}`; `Dockerfile`; `openapi.yaml`; `README.md`; `ARCHITECTURE.md`; `IMPLEMENTATION_PLAN.md`; `VALIDATION.md`.

Detailed commands, evidence, policy semantics and limitations are recorded in VALIDATION.md, README.md and ARCHITECTURE.md. Private screenshots and diagnostic evidence remain under ignored `.local/phase3`.
