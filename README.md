# Afterglow TV

A standalone, self-hosted, TV-first home for Plex-powered linear channels. Independent of VyNode Media.

**0.9.1 beta — Docker/web and standalone Android TV:** Personal ordering, hidden channels, private viewing history, explainable discovery and explicit shared smart-channel templates extend the television experience. Personal discovery never rewrites shared broadcasts. See VALIDATION.md for validation status.

## Install

- [Signed Android TV APK and checksums](https://github.com/minerport/VynodePlexTV/releases/tag/v0.9.1). ARM64; runs independently of Docker.
- [Unraid installation and Community Applications template](UNRAID.md). Catalog submission/review is separate.
- Docker image: `ghcr.io/minerport/vynodeplextv:latest`. Use `docker compose -f compose.release.yaml up -d` after configuring `.env` from `.env.example`.
- [Release notes and limitations](docs/RELEASE_NOTES.md). Licensed under [GPL-3.0](LICENSE).

## Your television

Open **My Channels** to favorite, hide/show, drag channels into your order, or use Move up/down. Choose **Channel Number Order** or **My Order**. Guide, mini-guide, On Now, Starting Soon and personalized discovery respect hidden channels. Numeric tuning still resolves the original global number, including a personally hidden channel. Reset restores ordering and visibility while keeping favorites.

**For You Right Now** and **What’s Good** rank already scheduled current/next programs, with reasons such as Favorite channel, Because you watch Action, or New this week. There is no cloud service, AI inference or Plex-history import. New users receive a favorites/timing/rating fallback. The diagnostic checkbox shows only your own score components.

Viewing history is private to each application account. It records confirmed forward playback, not schedule exposure or station breaks. A meaningful visit requires two minutes; inferred affinities start after three meaningful visits. Pauses, seeks, duplicate heartbeats and long disconnects do not earn watch time. History is retained for 180 days and pruned in the background. **Clear Viewing History** removes your history and derived affinities; favorites, ordering, hidden channels, Plex data and broadcast schedules remain intact. An already-open stream cannot recreate a cleared row.

Owners/admins open **Channels → Create Smart Channel** for Genre, Actor, Director, Decade, Comfort TV, Weekend Movies, Family Night and Seasonal templates. These create normal editable RULE channels after a real local membership preview. Comfort TV requires an explicit show choice. Family Night filters explicit content ratings; review the metadata before use. Weekend Movies is a theme, not a weekend-only clock schedule. Seasonal templates use a synchronized label and inclusive local dates. Channel editing also supports activation windows for explicit holiday rules built with genres, collections, titles or selections. Windows include the year; annual recurrence is not inferred. Current programs may finish after expiration. Shared templates do not use private history.

Personal channels with independent schedules, Unwatched Movies membership, household aggregates and time-of-day inference are deferred. Phase 7 personalizes discovery, not the meaning of a shared channel number.

## Broadcast Presentation

Owners/admins open **Broadcast** to upload assets, create rules and configure each channel. Presentation is opt-in. Upload real MP4/H.264 clips with AAC audio (or no audio), at most 1080p and ten minutes. The server inspects files with ffprobe; it does not transcode them. The default upload limit is 256 MiB, configurable with `PRESENTATION_UPLOAD_MB` (1–1024). Local development requires ffprobe on PATH; the Docker image includes it. Files persist under `/config/presentation`, outside the webroot. Preview and playback are authenticated and support byte ranges.

Assets can be Bumper, Ident, Trailer, Coming Up Next, Custom Clip or Filler. Assign tags and optional inclusive active dates. Plex Media uses an existing short playable inventory item; **Discover Plex trailers** imports only real extras exposed for the supplied local media ID. Hosted extras remain resolved by Plex, with credentials kept server-side. Missing trailers are optional; nothing is scraped or downloaded from external sites by Afterglow.

Rules select asset kind/tag, movie or episode scope, frequency, repeat window, priority and duration ceiling. Choose before programs, between programs or the first safe boundary after a clock-hour crossing. This is not a hard-clock scheduler. Filler uses real eligible clips within the duration ceiling; it does not pad, trim or invent silent gaps. A channel may inherit global rules, disable individual defaults, or use its own rules. Enable presentation, save, then choose **Repair future schedule** to apply at a protected boundary. Background maintenance also detects changed presentation inputs.

Clips consume real time in the persisted timeline. Scheduling Studio displays every entry; the viewer guide stays program-centric. Seeded selection, retained asset history and schedule fingerprints keep future entries stable through restart and reruns. Repeat windows relax to the oldest eligible asset when the pool is exhausted. Seasonal eligibility uses the channel timezone. A referenced asset cannot be deleted until its retained timeline references expire or are repaired away; disable it to exclude future selection.

Channel Presentation also offers a separately uploaded compact logo, a subtle corner bug and a Coming Up Next overlay (0–120 seconds; default 45). The bug uses TV-safe margins and hides with visible controls. Next-up appears only near a live boundary, not during catch-up or paused/behind-live viewing. The branding preview does not change the timeline.

Live transitions keep one viewer session while swapping isolated Plex/local stream resources. A failed optional clip becomes a visible next-up card for the remaining persisted interval, then the main program starts on schedule; future repair excludes confirmed missing assets. Main programming is never silently moved early. Watch From Beginning/catch-up plays the selected main program without prepending presentation. Local clips work without Plex; Plex-backed programming still requires its server.

The application does not impose a total uploaded-storage quota: administrators should monitor `/config`. No video generation, arbitrary media transcoder, exact-hour slots or external trailer scraping is included.

## Recently Added and watching TV

In **Channels**, choose **Create Recently Added Movies** or **Create Recently Added TV**. Creation is explicit and assigns the next free number. These are normal editable rule channels: rename, renumber, choose artwork/group, disable, or change the lookback in the channel editor. Default lookback is 30 elapsed days; suggestions include 7/14/30/60/90, with custom values from 1 to 3,650. TV matches episodes, never shows/seasons/movies. Empty windows leave the channel configured but empty.

Both templates default to Smart Random with recency preference. TV additionally prefers one other show before returning. Schedule settings expose optional recency and show-separation controls; repeat windows take precedence, and separation relaxes when eligible alternatives run out. Every finite eligible inventory can cycle through. Five-minute maintenance repairs newly eligible/aged-out future membership while preserving history and the current program. No Plex connection is required for membership or scheduling.

**Live TV** shows On Now and Recently Added networks. Choose **Watch Live** from home or a channel card. `#/watch/<channel-id>` opens the dedicated player. The backend uses the persisted current program and calculates the scheduled offset. **Watch From Beginning** starts the current item at zero; **Go Live** resolves the current broadcast again, including a new program if the boundary has passed. Pause/seek leaves the schedule running; a difference over ten seconds is shown as behind live. Live playback automatically tunes the next persisted program at the boundary. An early-ended/missing item displays a clear state rather than substituting unrelated content.

Player keys: **↑/Page Up** next channel, **↓/Page Down** previous, **Space** play/pause, **←/→** focus controls, **Enter** activate, **Escape** exit. Type a channel number; it resolves after 1.2 seconds. Controls fade after 6.5 seconds and reappear with pointer activity or control navigation. Browsers may require pressing Play once to allow audio. Last-channel preference is persisted per application user on the server; Plex credentials never enter browser storage.

Plex serves library programming; Afterglow serves uploaded presentation clips. Conservative MP4/H.264/AAC browser compatibility uses Direct Play; other supported single-part versions negotiate Plex's universal media decision then use HLS via the authenticated application proxy. Plex may copy streams or transcode according to its decision. No independent transcoder or direct filesystem access is used. Client resolution is currently capped at 1080p; advanced quality/version/subtitle selection and multipart titles are deferred. Transcoding availability depends on the Plex server/account. A retry resolves a fresh current program and session. An outage does not change inventory/schedules.

Settings includes active application viewers. Sessions are independent by viewer/client, stop on exit/switch/logout, expire after 90 seconds without heartbeat (cleanup every 30 seconds), and are stopped on application restart. Recent stopped records are retained seven days for diagnostics. Media is streamed without disk caching; existing bounded artwork caching remains unchanged. Reverse proxies must permit long streaming responses, Range requests, and buffering settings appropriate for video.

## Docker quick start

Requires Docker with Compose.

```powershell
Copy-Item .env.example .env
docker compose up --build -d
```

Open http://localhost:8080 and create the owner account. Passwords require 12–72 UTF-8 bytes. Keep the service on loopback until setup is complete. Data is stored in the Compose volume `afterglow_config` (Docker name `afterglow_afterglow_config`) at /config and survives restart/rebuild. `docker compose down` preserves data; `down -v` deletes it.

For LAN access set `BIND_ADDRESS=0.0.0.0` and leave `APP_ORIGIN=auto` in `.env`, recreate the service, and open that exact origin. Automatic mode validates same-origin requests against the actual request host and port. An explicit APP_ORIGIN remains available for reverse proxies. Use a TLS reverse proxy and `COOKIE_SECURE=true` outside a trusted LAN. Do not expose first-run setup to an untrusted network.

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

Use the published image and template described in [UNRAID.md](UNRAID.md). For source development, build with `docker build -t afterglow-tv:local .`. Add a container using that image, bridge networking, container port 8080, and a writable host path such as `/mnt/user/appdata/afterglow-tv` mapped to `/config`. The image runs as UID/GID 10001: grant that account write access to the directory. Leave APP_ORIGIN=auto for direct LAN access. No privileged access, GPU or host networking is required. Complete setup on a trusted network. Stop the container before backing up the whole config directory; retain a matching image/version for restore.

## API and configuration

Implemented API: [openapi.yaml](openapi.yaml). Public endpoints: `/api/v1/health`, `/api/v1/ready`, `/api/v1/version`, `/api/v1/auth/status`. Authenticated session data includes a CSRF token; send it in `X-CSRF-Token` with the configured Origin for mutations. Settings writes require OWNER/ADMIN.

Environment variables: `CONFIG_DIR` (default /config), `LISTEN_ADDR` (:8080), `APP_ORIGIN` (auto), `COOKIE_SECURE` (false for local HTTP), `LOG_LEVEL` (INFO), `WEB_DIR` (web/dist). Docker sets WEB_DIR=/app/web. Use `.env.example` for Compose settings. Never commit credentials or local databases.

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

## Create channels

Open **Channels → Create channel** as OWNER or ADMIN. Choose a unique number, name, group and content source. The right-side preview shows current membership, not airtimes. Empty matches produce a warning and can be saved; malformed rules and missing references cannot.

- **Handpicked:** search movies/episodes, filter by type, library, genre or show, select a page or individual items, then drag or use the up/down buttons to reorder. Shows and seasons are containers, never playable members. Up to 1,000 explicit selections are supported. Unavailable selections stay configured and can be inspected with the troubleshooting checkbox.
- **Show:** choose a show with server/library context, all seasons or selected seasons, and whether to include specials. Episodes resolve by numeric season/episode order. Select at least one season for selected-season mode. Unavailable episodes are excluded by default.
- **Collection:** choose a specific Plex collection and library. Same-name collections are kept distinct. Collection membership on a show expands to its episodes.
- **Smart rules:** combine ALL (AND), ANY (OR), and NONE (NOT) groups with typed fields. Pick metadata from searchable lists. Dates use date inputs, numeric ranges use two values, and duration is in minutes. The server independently validates every saved or previewed rule.

Groups have names, slugs and display order. Deleting a nonempty group explicitly moves or ungroups its channels. Channel actions include edit, duplicate, preview, enable/disable and confirmed deletion. Duplicate copies configuration with a new identity/number. Disabled channels retain their source and membership.

Artwork can be automatic, selected Plex media, or an uploaded PNG/JPEG/WebP (including transparency). Uploads are limited to 8 MiB, 8192 pixels per edge and 36 megapixels, with a total 256 MiB upload budget. They live under `/config/uploads/channels` and survive restarts. Unreferenced uploads older than 24 hours are cleaned on subsequent uploads; referenced images shared by duplicate channels are retained. Keep the whole `/config` volume in backups. If Plex artwork cannot be retrieved, a generated channel identity is used.

Membership and counts query SQLite only. Plex being OFFLINE does not remove synchronized content from channel matches. Normally only available movies/episodes in selected, present libraries on enabled servers match; explicit show configuration or the preview troubleshooting option can include unavailable/inactive inventory. This is metadata inspection, not a guarantee that Plex can play an item. Accounts with USER role cannot access channel administration in this phase.

## Phase 2 limits and upgrade

Migration `003_virtual_channels.sql` upgrades the existing database without changing owner accounts, sessions, Plex credentials or inventory identities. Back up `/config` before upgrading; do not delete it. A previous binary cannot open the newer schema: restore its matching backup for rollback.

Rules support eight levels and 64 nodes, with up to 64 values per multi-selection. Metadata options are paginated, previews return at most 100 items, and channel lists are paginated. String matching uses SQLite's case-insensitive behavior (ASCII case folding, not full language-aware search). Missing metadata does not satisfy positive conditions; negation includes missing metadata. Relative date rules evaluate against current UTC time and may change as time passes. Collections and people inherit Phase 1's source identity limitations where Plex omits IDs.

Channel numbers are 1–24 letters/digits/dots/hyphens/underscores, unique case-insensitively across enabled and disabled channels. No favorites semantics, playback policy, randomization, repeat windows or scheduling are implied by membership order. Phase 3 is the separately authorized deterministic scheduling engine.

## Scheduling studio (Phase 3)

Open **Channels → Scheduling studio**, a channel's **Schedule** button, or **Edit → Scheduling settings & preview**. The studio is an administrative chronological preview, not the final television guide. It shows saved programs, precise start/end times, episode context, artwork, coverage, diagnostics and job outcomes. No playback is implemented.

Defaults are explicit and editable: Sequential for Static/Show channels, Smart Random for Rule/Collection channels, 7 past days, 14 future days, application timezone, a persisted anchor seven days before channel creation and a generated seed. Existing channel definitions are preserved by migration 004. Scheduling settings are separate from source definitions; save channel source edits before opening its schedule settings.

- **Sequential:** respects static item positions, show season/episode order, or the existing stable dynamic membership order. Wraps continuously.
- **Random:** a stable seeded permutation repeated in complete cycles; no immediate duplicate when more than one item is eligible. A repaired membership boundary also avoids an immediate duplicate.
- **Smart Random:** first prefers items outside their movie/episode repeat window (14/30 days by default), then the oldest prior air start. If all items fall inside their window, relax to the oldest non-immediate item; a single-item channel necessarily repeats. Unseen/otherwise tied items prefer a different show/season, then use a deterministic seed/position tie-break. Windows use persisted scheduled history, not actual viewing history. Seed changes primarily affect ties and unseen-item choices; an oldest-first small library can retain its order.

**Generate / extend** fills coverage and applies detected invalidation. **Repair future** rebuilds from the earliest affected future program; added membership, changed source/settings or sequential order rebuild from the first future program. **Regenerate future schedule** explicitly rebuilds all future programs after confirmation. Programs that have already started, including the current one, remain unchanged. A missing/changed future duration rebuilds the suffix from that entry so timing stays continuous. Title/artwork-only updates do not invalidate timing (unless a title change changes a Sequential channel's membership ordering). Changes are applied by the next maintenance run or a manual job.

Every timeline is stored as UTC Unix **milliseconds**, with half-open intervals `[start,end)`. The initial anchor supplies a stable origin; an existing timeline's protected cut point takes precedence when the anchor is edited. A later anchor does not insert an artificial gap into an existing schedule. Calendar-day browsing and horizons use the configured IANA timezone and handle 23/25-hour DST days. Repeat windows are elapsed 24-hour days. Raising past retention cannot recover already-pruned historical rows.

One scheduler worker checks enabled channels every five minutes, extends their future coverage, repairs changes and prunes expired rows. It shares the durable job table with Plex but uses separate job types and claims. Jobs show honest indeterminate progress while computing and committed generated/replaced counts afterward. Interrupted transactions roll back; interrupted RUNNING jobs become CANCELLED on restart, while queued jobs survive. Automatic maintenance retries later. The API's shared job-cancellation endpoint is best-effort for queued scheduler work; an already committing transaction may finish first.

Disabled channels retain settings/history/future rows and stop extending. Empty channels stay configured and report EMPTY; unusable durations are excluded with diagnostics. No arbitrary runtime is substituted. Duplicates copy settings with a new seed but no programs. Deleting a channel cascades schedule records without touching inventory. Scheduling operates entirely from SQLite during a Plex outage.

Initial resource limits: 50,000 members/channel, 100,000 generated entries/job, usable duration up to seven days, 0–30 past / 1–30 future horizon days, and 0–365 repeat-window days. Ancient anchors or extremely short clips that exceed the entry budget fail atomically; shorten the horizon or use a suitable initial anchor. The existing single SQLite connection means large generation transactions can briefly delay other database requests. The day API is paginated (up to 500 entries/page); the UI requests 100. USER can read enabled-channel schedules; administration and current artwork proxy URLs remain OWNER/ADMIN-only. Full viewer artwork access belongs with the viewer integration.

No block scheduling, slots, seasonal rules, interstitials, player, streaming, EPG or recommendations were added. The next phase is **Linear Playback and Live Channel Tuning**, only after explicit authorization.

## Phase 5 viewer experience

**Guide** is a viewer interface over persisted broadcast rows. It opens around server time, shows three hours per window, and supports a seven-day-past/fourteen-day-future day strip, Earlier/Later, horizontal scrolling and NOW. Available history depends on schedule retention. Channel numbers/logos stay frozen; cell widths use exact durations. Completely unscheduled, disabled and hidden channels are excluded. A channel with retained schedules but no entries in the selected window shows an explicit empty-window message.

Use **Up/Down** for adjacent channels at approximately the same time, **Left/Right** for neighboring programs, **Enter** for details, and **Escape** to return. Focus scrolls into view and returns to the selected program after details/playback. Removing the last favorite returns focus to NOW. The date strip follows the selected day. Desktop/tablet use the full grid; phones use a narrower frozen channel column with touch-scrollable timeline and filters.

Details distinguish past/current/future entries. Current programs offer Watch Live and Watch From Beginning. Retained past programs offer Watch From Beginning when media remains available; this starts Plex media at zero, without recordings or changes to broadcast history. Future entries have details only. Catch-up is labeled explicitly in the player; Go Live tunes the channel's current broadcast rather than seeking within the historical item.

Personal favorites are available on guide rows, details, home cards and the player. The Favorites filter preserves channel-number order. Persisted administrator groups are independent filters. Favorites and last successfully tuned channel now live in SQLite per application user, survive container recreation and work across that user's devices. Browser last-channel storage is only a compatibility hint; the server's response takes precedence.

During playback, press **G** or choose **Mini-guide** to browse current/next channels; **I** opens current program details. Full Guide opens over the same video. Browsing overlays does not create another stream; tuning replaces the current session. Escape closes the top overlay before exiting the player. Home provides On Now, personal favorites, Starting Soon (next 30 minutes, sorted by airtime), separate Recently Added linear networks, groups and Last Channel.

Guide metadata works with Plex offline; uncached artwork and playback still need Plex. Window data refreshes every 30 seconds; the Now line advances locally from the server-time sample every second. Guide channel pages contain 50 channels (API maximum 100), with row virtualization and three-hour UI windows. The API permits up to 24 hours/10,000 programs per request; smaller windows are required for unusually dense short-clip schedules. Home/mini-guide Starting Soon data currently covers the first 100 channels; the full guide offers channel paging. Optional scheduled-program search and reminders are deferred.

Migration 007 adds personal favorites, typed viewer preferences and playback origin. It preserves prior migrations, credentials, inventory and program rows. API additions are documented in OpenAPI; regenerate Phase 5 additions with `node scripts/update-guide-openapi.mjs`. Phase 6 broadcast presentation/interstitials is not implemented.

## Standalone Android TV development

The native Android application is being built as a self-contained alternative to
Docker: Plex sign-in, inventory, channels and scheduling live on the device.
It does not require an Afterglow server or use a WebView. See [ANDROID_TV.md](ANDROID_TV.md)
for architecture, build/install instructions and honest validation status.

### Appearance
Settings → Appearance offers Tidal, Aurora, Ember, Glacier and OLED palettes,
Cinematic/Compact layouts, and reduced-effects controls in web and Android TV.
Choices are local to the browser/device, not shared account settings. Web honors
system reduced-motion preferences; native focus animation and artwork fades can
be disabled. Appearance never changes channel numbers or broadcast schedules.

### Automatic library updates
Selected libraries are checked incrementally every 15 minutes by default; custom
refresh intervals remain configurable in Plex settings. The original six-hour
default is upgraded to 15 minutes. Successful scans durably queue deduplicated
schedule checks for enabled channels. Rules, collections, shows, and Recently
Added channels resolve new membership locally. Static channels retain explicitly
selected items. The scheduler preserves past/current entries and repairs future
programming only when relevant inputs change. Failed scans never reconcile
missing items or enqueue completion repairs. Existing five-minute scheduler
maintenance remains a recovery backstop and handles Recently Added aging out.
Docker monitors while running; Android checks while its local engine is alive
and checks overdue work on startup. Android process death/device sleep can delay
refresh; this is polling, not an always-on Android background service.

Existing channels created by the old single-show template are upgraded to dedicated Show channels with Sequential scheduling. Their future schedule is repaired after the current entry; prior broadcasts remain unchanged. Sequential repair continues after the last preserved episode, including when membership changes. Mixed-show and Recently Added networks retain their configured distribution policies.

Channel Studio on Android uses searchable 12-item, three-column choice pages. Creation includes content type, guide category, ordering, repeat window, optional year bounds and minimum rating. Existing channels can be renamed/renumbered, categorized, enabled/disabled and rescheduled from Edit; rule channels can be restricted to movies or episodes. Web smart templates expose the same optional filters and scheduling choices; the full web editor remains available for nested rules and artwork uploads.

Guide categories respect explicit channel groups. Ungrouped channels with exclusively movie or episode programming also appear in Movies or TV; mixed networks remain in All unless a group is assigned. APK guide selection survives favorite changes and return navigation. Changing content repairs future programming without interrupting the current entry.

### Plex sign-in connection discovery

Choose **Sign in with Plex**, authorize on Plex, then choose a server. The default **Automatic connection** first probes HTTPS addresses advertised by that account, with at most four concurrent probes, six seconds per probe and a 14-second HTTPS discovery budget. A remote route is tried early so stale LAN/container interfaces cannot consume the entire budget. The first verified reachable endpoint wins; both machine identity and library access must match. The whole connect operation is bounded to 27 seconds. If HTTPS fails, account-advertised local literal private/loopback HTTP addresses are eligible for fallback. Public HTTP addresses, DNS names, relay HTTP, redirects and authentication failures do not trigger automatic downgrade. Manual HTTP/HTTPS selection remains available.

An explicit connection remains available for troubleshooting; HTTP is labeled unencrypted and should only be used on a trusted network. The strict `plex.direct` DNS fallback supports encoded IPv4 and IPv6 addresses while retaining the original hostname for TLS certificate verification. It never disables certificate checks. This cannot bypass a firewall, VPN routing policy, unavailable Plex server or disabled remote access. Automatic discovery runs from the Docker server or the standalone Android device, not the browser that approved sign-in. If the saved address later changes, sign in again to reconnect the existing server identity without replacing inventory. Linking remains short-lived; account authorization does not imply that every advertised server address is reachable.
