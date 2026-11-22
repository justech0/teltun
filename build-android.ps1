# Builds the Android / ChromeOS app: build\bin\TelTun.apk
# Needs Go 1.25+, gomobile (go install golang.org/x/mobile/cmd/gomobile@latest),
# JDK 17+, and the Android SDK with an NDK (Android Studio installs the SDK;
# add the NDK from its SDK Manager).
$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot

if (-not $env:ANDROID_HOME) { $env:ANDROID_HOME = Join-Path $env:LOCALAPPDATA 'Android\Sdk' }
if (-not $env:ANDROID_NDK_HOME) {
  $ndk = Get-ChildItem (Join-Path $env:ANDROID_HOME 'ndk') -Directory | Sort-Object Name | Select-Object -Last 1
  if (-not $ndk) { throw "No NDK in $env:ANDROID_HOME\ndk" }
  $env:ANDROID_NDK_HOME = $ndk.FullName
}

# Phone helpers, embedded into the Go library (same as the desktop builds).
$env:CGO_ENABLED = '0'
$env:GOOS = 'linux'
foreach ($a in @('arm64', 'arm', 'amd64')) {
  $env:GOARCH = $a
  $env:GOARM = '7'
  go build -trimpath -ldflags '-s -w' -o "internal/droid/phone/teltun-$a" ./cmd/teltun-phone
  if ($LASTEXITCODE) { exit 1 }
}
Remove-Item Env:GOOS, Env:GOARCH, Env:GOARM, Env:CGO_ENABLED

# Go engine -> Java library. x86_64 for Intel/AMD Chromebooks, arm64 for the rest.
New-Item -ItemType Directory -Force android/app/libs | Out-Null
gomobile bind '-target=android/arm64,android/amd64' -androidapi 26 -tags with_gvisor -trimpath `
  -ldflags '-s -w' -javapkg app.teltun -o android/app/libs/teltun.aar ./mobile
if ($LASTEXITCODE) { exit 1 }

Push-Location android
try {
  ./gradlew.bat assembleRelease --console=plain -q
  if ($LASTEXITCODE) { exit 1 }
} finally { Pop-Location }

New-Item -ItemType Directory -Force build/bin | Out-Null
Copy-Item android/app/build/outputs/apk/release/app-release.apk build/bin/TelTun.apk -Force
Write-Host "Done: build\bin\TelTun.apk"
