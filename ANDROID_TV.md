# Android TV — standalone application (Phase 8 in progress)

Afterglow TV runs on the television itself. No Docker installation, Afterglow
server address, pairing page, Google Play Services, or WebView is required.
Plex remains the media source. The APK signs in to Plex directly using a QR
approval flow, discovers libraries, and synchronizes metadata into app-private
SQLite storage.

## Architecture

Kotlin / Compose for TV provides the native interface; Media3 provides playback.
The APK bundles the existing Go programming engine as an ARM64 native executable.
It reuses migrations 001–011, channel resolution, deterministic scheduling,
presentation, guide, viewing history and Plex orchestration. The UI talks to a
random authenticated loopback endpoint restricted to 127.0.0.1. This is private
in-process-application communication, not a separately installed LAN server.
No port is advertised or exposed on the network.

Every installation owns its inventory, channels, anchors, schedules and history.
Docker and Android do not automatically synchronize them. The Docker deployment
continues to run independently. Plex media is streamed, not copied to the TV.

The Plex encryption key is encrypted by Android Keystore and passed to the engine
through its private startup pipe, never command arguments or logs. The engine
stores encrypted Plex credentials in its private database. TLS verification stays
enabled. Android active-network DNS addresses are supplied to the Go resolver.
If protected storage cannot be decrypted, local data is retained and startup fails
rather than silently generating a replacement key.

## Build

Requirements: JDK 17, Android SDK 36, Go toolchain matching go.mod, and Gradle wrapper.
Current development package targets ARM64. Other ABIs need their own engine build
and physical validation before claiming support. Run from repository root:

```powershell
./clients/android-tv/build-engine.ps1
./clients/android-tv/gradlew.bat -p clients/android-tv :app:assembleDebug :app:testDebugUnitTest :app:lintDebug
```

On Windows installations that use the Windows trust store for corporate roots,
append `-Djavax.net.ssl.trustStoreType=Windows-ROOT -Djavax.net.ssl.trustStore=NONE`.
Do not disable TLS validation.

Debug APK: `clients/android-tv/app/build/outputs/apk/debug/app-debug.apk`.
Generated native binaries, APKs, SDK paths and signing material are ignored.
Release signing uses environment variables and encrypted Actions secrets; see docs/RELEASING.md. Signed APKs and SHA-256 checksums are on GitHub Releases. Release APKs cannot update debug-signed installs in place; preserve private data before changing installations.

## Shield / Android TV / Fire TV installation

Enable network debugging in device developer settings, then:

```text
adb connect DEVICE_IP:5555
adb install -r clients/android-tv/app/build/outputs/apk/debug/app-debug.apk
adb shell am start -n tv.afterglow.client/.MainActivity
```

Approve the debugging prompt on your TV. Fire TV sideloading also requires enabling
ADB debugging; this ARM64 package requires a 64-bit app runtime. Fire TV physical
compatibility is not yet validated. No Google account dependency is used.

Launch Afterglow, choose Sign in with Plex, scan the QR code with a phone, approve,
and select the Plex server connection reachable from the television. In Settings,
select libraries and synchronize. Channels offers Recent Movies/TV and explicit
Genre, Actor, Director, Show and Decade templates using local metadata.

Use `adb install -r` to upgrade while retaining app-private data. Uninstall with
`adb uninstall tv.afterglow.client`; uninstalling deletes local channels, inventory,
history and credentials. It does not modify Plex or Docker data.

## Current validation and limitations

Physical Shield (Android 11): standalone engine launch and direct Plex QR creation
verified. Full sign-in, library sync, native playback, track selection and lifecycle
validation remain in progress. Do not treat this development APK as Phase 8 complete.
The native administration surface is still narrower than Docker's web administration.
No background service guarantee: Android may stop local jobs when the app is killed;
reopening restarts the engine and existing job recovery. Schedules persist and are
extended/repaired from their saved state. No external Afterglow dependency is used.
Do not collect unrestricted logcat: limit diagnostics to the application and never
export credentials, database files, QR contents, or personal viewing history.

The embedded engine sets SQLITE_TMPDIR and TMPDIR to an app-private cache
subdirectory before launch. Android does not provide a writable conventional
/tmp; large SQLite transactions must be able to spill to this directory.
Sync retries use incremental refresh, retaining local identities and checking
complete inventory before missing-item reconciliation. Settings displays safe
job errors and processed/total counts.
