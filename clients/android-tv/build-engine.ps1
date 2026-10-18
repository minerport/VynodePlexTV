# Requires Docker only on the BUILD machine, not on the Android device.
$ErrorActionPreference = 'Stop'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$target = Join-Path $PSScriptRoot 'app/src/main/jniLibs/arm64-v8a'
New-Item -ItemType Directory -Force $target | Out-Null
$goVersion = ((Get-Content (Join-Path $repo 'go.mod') | Where-Object { $_ -match '^go ' }) -split ' ')[1]
docker run --rm --mount "type=bind,source=$repo,target=/src" -w /src "golang:$goVersion" sh -c 'GOOS=android GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -o clients/android-tv/app/src/main/jniLibs/arm64-v8a/libafterglow.so ./cmd/android-engine'
if ($LASTEXITCODE -ne 0) { throw 'Android engine build failed' }
