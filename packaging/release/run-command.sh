#!/bin/sh
set -eu
export PATH=/usr/bin:/bin:/usr/sbin:/sbin LC_ALL=C
[ "$#" = 1 ] || exit 2
MODE=$1
[ "$(id -u)" != 0 ] || { echo 'Start this command as your ordinary local account.' >&2; exit 1; }
[ "$(sysctl -n hw.optional.arm64)" = 1 ] || { echo 'Apple Silicon is required.' >&2; exit 1; }
ROOT=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
TOOL="$ROOT/bin/nero-setup"
ACCOUNT_UID=$(id -u)
case "$MODE" in
 check|update-gui) exec /usr/bin/arch -arm64 "$TOOL" --mode "$MODE" --release "$ROOT" --uid "$ACCOUNT_UID";;
 install|uninstall) ;;
 *) exit 2;;
esac
# The ordinary caller can access its selected download/documents directory.
# Privileged children must only read this explicit payload in unprotected /tmp;
# they receive no additional access to personal folders or TCC permissions.
/usr/bin/arch -arm64 "$TOOL" --mode verify --release "$ROOT"
RELEASE_STAGE=$(/usr/bin/mktemp -d /private/tmp/nero-release.XXXXXXXX)
trap '/bin/rm -rf "$RELEASE_STAGE"' EXIT
trap 'exit 1' HUP INT TERM
/usr/bin/ditto "$ROOT" "$RELEASE_STAGE/release"
ROOT="$RELEASE_STAGE/release"
TOOL="$ROOT/bin/nero-setup"
# AppleScript argv and quoted form prevent shell expansion of paths/account data.
/usr/bin/osascript - "$TOOL" "$ROOT" "$MODE" "$ACCOUNT_UID" <<'APPLESCRIPT'
on run argv
 set toolPath to item 1 of argv
 set releasePath to item 2 of argv
 set setupMode to item 3 of argv
 set accountUID to item 4 of argv
 set commandText to "/usr/bin/arch -arm64 " & quoted form of toolPath & " --mode " & quoted form of setupMode & " --release " & quoted form of releasePath & " --uid " & quoted form of accountUID
 return do shell script commandText with administrator privileges
end run
APPLESCRIPT
