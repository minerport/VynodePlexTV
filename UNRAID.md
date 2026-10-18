# Unraid installation

Afterglow TV is a standalone Plex-powered television app. Its repository name is VynodePlexTV; it does not depend on VyNode Media.

## Install

The Community Applications template is [templates/afterglow-tv.xml](templates/afterglow-tv.xml). Inclusion in the Apps catalog requires separate Community Applications review; a GitHub release does not imply acceptance.

Until listed, download the raw XML into `/boot/config/plugins/dockerMan/templates-user/my-afterglow-tv.xml` on Unraid, then select it from **Docker → Add Container → Template**. Download only from this repository. The image is `ghcr.io/minerport/vynodeplextv:latest`; pin `:0.9.1` for this beta release.

1. Map `/config` to `/mnt/user/appdata/afterglow-tv` (prefer a cache/appdata pool).
2. Choose the host port, default 8080.
3. **Set Browser Origin** to exactly the URL used in your browser, including scheme and port, without a trailing slash. The default `http://tower:8080` only works when you actually browse that hostname. Using an IP instead requires changing this setting to that IP. An origin mismatch prevents authenticated writes by design.
4. The template runs as **99:100**, Unraid's usual nobody/users IDs. The appdata directory must be writable by these IDs. It needs no privileged mode, host networking, GPU or Plex filesystem mounts. Do not add unsupported PUID/PGID variables.
5. Start, create the owner account on your trusted LAN, sign in with Plex, select libraries and synchronize. Choose automatic secure connection discovery first.

The image defaults to 10001:10001 outside this Unraid template. If importing an existing installation, stop it first and ensure the target user can read/write its complete appdata tree. Never reset or delete existing appdata to fix permissions.

## HTTPS, backup and upgrades

For a reverse proxy, set APP_ORIGIN to the browser-facing HTTPS origin and COOKIE_SECURE=true. TLS to Plex stays verified. Do not expose the first-run owner setup publicly.

Stop the container before backing up **all** of appdata, including encryption material. Keep the matching image tag. Migrations run on startup; downgrading an image may require restoring its matching backup. Do not delete appdata for an upgrade. Secrets and cache remain outside the image.

Health check: `GET /api/v1/ready`. Support: [GitHub Issues](https://github.com/minerport/VynodePlexTV/issues). Report version, environment and safe error summaries; never attach your database, tokens, encryption keys or private viewing history.

## Community Applications submission

This repository follows the official [CA starter](https://github.com/unraid/unraid-community-apps-starter): root `ca_profile.xml`, one v2 Docker XML under `templates`, public image, hosted icon, project/support links, exact raw TemplateURL and GPL-3.0 license. A project link is supplied; no fictional Unraid forum support thread is listed.

Maintainer: open [CA submission](https://ca.unraid.net/submit), submit `https://github.com/minerport/VynodePlexTV`, then run **Validate** and **Scan**. Resolve any scanner feedback and complete review. Automated local XML checks and an Unraid-style UID/path smoke test are not a physical Unraid validation or CA approval.

The Android APK is distributed through GitHub Releases, not the Docker Apps catalog. It runs independently on ARM64 Android TV devices; no Docker server is required.

The template supplies the icon and WebUI automatically. Docker images also include
`net.unraid.docker.icon` and `net.unraid.docker.webui` labels for manual/Compose
installations. Existing containers need an image update and recreation to inherit
new image labels; restarting alone does not change container metadata. An explicit
container label overrides the image default. Unraid may cache an existing icon.
