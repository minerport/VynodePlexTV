# Release maintenance

Tag `vX.Y.Z` after updating Dockerfile, web package/version lock, Android versionName/versionCode, embedded-engine version, release compose tag and release notes. The tag workflow tests Go/web, builds a linux/amd64 image and an ARM64 standalone APK, signs the APK, publishes checksums and creates a GitHub beta release. The container receives both the version and latest tags.

Android signing requires Actions secrets ANDROID_KEYSTORE_BASE64 and ANDROID_KEY_PASSWORD. The JKS alias is afterglow; store and key passwords use the same secret. Signing keys must never be checked in or shared in logs. Maintain an encrypted offline backup of the original key and password: losing them prevents normal APK updates. Local builds use AFTERGLOW_KEYSTORE, AFTERGLOW_STORE_PASSWORD and AFTERGLOW_KEY_PASSWORD environment variables. Missing signing material produces an unsigned local release, not a distributable APK.

Ensure the GHCR package is publicly readable and verify an anonymous image pull. Publish source for each binary tag under the repository GPL-3.0 license. Dependency licenses remain with their respective owners. Android/Go dependencies are obtained from declared manifests; Alpine/FFmpeg packages retain their upstream license notices and source repositories.

CA submission uses the public repository URL at ca.unraid.net/submit; run its hosted Validate and Scan. Local XML checks do not substitute for review or acceptance. Do not advertise catalog inclusion before acceptance.

Validated main-branch pushes automatically publish `ghcr.io/minerport/vynodeplextv:latest`. Unraid and Compose installation defaults track latest. Versioned release tags remain available for recovery, not the default installation path. Unraid update scheduling is controlled by the operator.

Confirmed fixes must be published, not merely committed or installed on a development device. Docker fixes publish through validated main builds to latest. APK changes require a version-code increment, signed release APK/checksums, and a tagged GitHub release. Verify artifact availability before reporting publication; identify any device-only build explicitly.
