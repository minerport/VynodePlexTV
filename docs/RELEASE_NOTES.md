# Afterglow TV 0.9.1 beta

First public release of Afterglow TV in minerport/VynodePlexTV. Independent of VyNode Media.

- Docker/web: Plex account linking with automatic secure connection selection; library synchronization; configurable channels; deterministic schedules; live playback/catch-up; full guide; personalization; presentation/interstitials; five themes and compact layouts.
- Android TV: native standalone ARM64 APK with its own local programming engine and Plex QR sign-in. No Afterglow Docker server is required. This is not a WebView.
- Unraid: v2 template, persistent appdata, non-root UID/GID 99:100, health check, hosted icon and install/support documentation. CA catalog acceptance is a separate step.

Docker: `ghcr.io/minerport/vynodeplextv:0.9.1` (linux/amd64), also tagged `latest`.

Download the signed `Afterglow-TV-0.9.1-arm64.apk` and verify it against `SHA256SUMS.txt`. Android 8+ and an ARM64 app runtime are required. Shield development testing has occurred; this exact release and other TV/Fire TV hardware are not universally validated.

The signed release uses a persistent release certificate. It cannot update a development/debug-signed installation in place. **Do not uninstall a development build without securing its private app data**; use a separate device for initial release testing if needed. Future releases signed by the same key can upgrade normally. Docker and APK databases are independent.

Known limits: beta software; no claim of physical Unraid, Safari, WAN/load or all-device validation. Local uploaded interstitials require compatible MP4, there is no independent transcoder or total upload quota. Plex availability and capabilities determine playback. Back up appdata before upgrades. This release does not add external IPTV, HDHomeRun or DVR.

Installation: README.md, UNRAID.md and ANDROID_TV.md. Support through this repository's Issues. Never upload credentials, databases or private viewing history in bug reports.
