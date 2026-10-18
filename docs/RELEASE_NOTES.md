# Afterglow TV 0.9.2 beta

This release includes all confirmed fixes since 0.9.1:

- Android TV: correct HLS stream detection on extensionless playback URLs. The physical Shield operator confirmed previously failing channels and mini-guide switching now play. Automatic reconnects are bounded.
- Docker/Unraid: PNG icon and automatic icon/WebUI metadata; installations default to latest.
- Setup: automatic same-origin LAN browser handling, including mapped ports. Existing fixed APP_ORIGIN overrides must be changed to auto once. CSRF protection remains enforced.
- Plex: verified HTTPS first, with local private-address HTTP fallback for account-advertised connections. Remote HTTP remains an explicit choice.
- Browser: volume/mute and optional audio boost up to 200% without retuning. Actual listening validation remains device-dependent.

Docker: ghcr.io/minerport/vynodeplextv:latest (also tagged 0.9.2).
APK: Afterglow-TV-0.9.2-arm64.apk, standalone Android TV, Android 8+ ARM64.

The signed APK uses the same release certificate as 0.9.1. Debug-signed development installations cannot upgrade to it in place; do not uninstall or clear their data without a recovery plan. This release does not claim validation on all TV devices, Safari, WAN/load or physical Unraid.

Verify the downloaded APK against SHA256SUMS.txt. Community Applications catalog submission/review remains separate.
