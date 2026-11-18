#!/bin/sh
# Optimized, ad-hoc signed Apple Silicon release. Never reads an installed App.
set -eu
[ "$#" = 1 ] || { echo 'Usage: build.sh /absolute/new-stage' >&2; exit 2; }
STAGE=$1
case "$STAGE" in /*) ;; *) exit 2;; esac
[ ! -e "$STAGE" ] || { echo 'Use a new staging directory.' >&2; exit 1; }
[ "$(uname -m)" = arm64 ] || { echo 'Build natively on Apple Silicon.' >&2; exit 1; }
ROOT=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
cd "$ROOT"
GO=${GO:-go}
export GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 GOTOOLCHAIN=local
export GOMAXPROCS=${GOMAXPROCS:-2} MACOSX_DEPLOYMENT_TARGET=14.0
export CGO_CFLAGS='-O2 -g0 -mmacosx-version-min=14.0'
export CGO_CXXFLAGS='-O2 -g0 -mmacosx-version-min=14.0'
export CGO_LDFLAGS='-O2 -g0 -mmacosx-version-min=14.0'
export GOFLAGS=-mod=vendor
BUILD="$STAGE/build"
RELEASE_VERSION=$(cat VERSION)
RELEASE="$STAGE/NERO Battery $RELEASE_VERSION"
APP="$RELEASE/app/NERO Battery.app"
mkdir -p "$BUILD" "$APP/Contents/MacOS" "$APP/Contents/Resources" "$RELEASE/bin" "$RELEASE/launchd" "$RELEASE/docs" "$RELEASE/licenses"
"$GO" build -p 2 -trimpath -buildvcs=false -ldflags='-s -w -buildid=' -o "$RELEASE/bin/nero-setup" ./cmd/nero-setup
# A deterministic digest of the clean exported source, excluding the digest itself.
"$RELEASE/bin/nero-setup" --mode source-info --release "$ROOT"
for pair in 'nero-battery-helper' 'nero-control-client';do
 "$GO" build -p 2 -trimpath -buildvcs=false -ldflags='-s -w -buildid=' -o "$RELEASE/bin/$pair" "./cmd/$pair"
done
"$GO" build -p 2 -trimpath -buildvcs=false -ldflags='-s -w -buildid=' -o "$APP/Contents/MacOS/nero-battery" ./cmd/nero-battery
cp packaging/local/Info.plist "$APP/Contents/Info.plist"
cp LICENSE "$APP/Contents/Resources/LICENSE-batt.txt"
/usr/bin/clang -O2 -mmacosx-version-min=14.0 -fobjc-arc -framework Cocoa packaging/local/artwork.m -o "$BUILD/render-artwork"
"$BUILD/render-artwork" "$BUILD/artwork" "$BUILD/artwork-preview.png"
/usr/bin/iconutil --convert icns --output "$APP/Contents/Resources/AppIcon.icns" "$BUILD/artwork/Nero.iconset"
cp "$BUILD/artwork/MenuMarkTemplate.png" "$BUILD/artwork/MenuMarkTemplate@2x.png" "$APP/Contents/Resources/"
cp packaging/local/io.nero.battery.helper.plist "$RELEASE/launchd/"
cp VERSION SOURCE-INFO.json "$RELEASE/"
cp README.md "$RELEASE/README.md"
cp docs/PORTABLE.md docs/PRIVACY.md docs/SECURITY.md docs/TRIAL.md docs/THIRD-PARTY.md docs/CHANGES.md "$RELEASE/docs/"
cp docs/USB-POWER.md "$RELEASE/docs/"
cp LICENSE "$RELEASE/licenses/LICENSE-batt.txt"
cp "$("$GO" env GOROOT)/LICENSE" "$RELEASE/licenses/LICENSE-Go.txt"
cp -R vendor/. "$BUILD/vendor-licenses-source"
# Only licenses, not runtime source trees, belong in the binary archive.
find "$BUILD/vendor-licenses-source" -type f ! -iname '*license*' ! -iname '*copying*' ! -iname '*notice*' -delete
cp -R "$BUILD/vendor-licenses-source/." "$RELEASE/licenses/"
for command in Check Install Uninstall Update-GUI;do
 cp "packaging/release/$command.command" "$RELEASE/$command.command"
 chmod 755 "$RELEASE/$command.command"
done
cp packaging/release/run-command.sh "$RELEASE/bin/run-command.sh"
chmod 755 "$RELEASE/bin/run-command.sh"
for component in helper client setup;do
 case "$component" in helper) exe=nero-battery-helper; id=io.nero.battery.helper;; client) exe=nero-control-client; id=io.nero.battery.controlclient;; setup) exe=nero-setup; id=io.nero.battery.setup;; esac
 /usr/bin/codesign --force --sign - --options runtime --timestamp=none --identifier "$id" "$RELEASE/bin/$exe"
done
/usr/bin/codesign --force --sign - --options runtime --timestamp=none --identifier io.nero.battery.app "$APP"
"$RELEASE/bin/nero-setup" --mode manifest --release "$RELEASE"
"$RELEASE/bin/nero-setup" --mode verify --release "$RELEASE"
printf '%s\n' "$RELEASE"
