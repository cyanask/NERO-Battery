#!/bin/sh
# Export known source inputs, never local App, policy, account data or build caches.
set -eu
[ "$#" = 1 ] || { echo 'Usage: export-source.sh /absolute/new-source' >&2; exit 2; }
DEST=$1
case "$DEST" in /*) ;; *) exit 2;; esac
[ ! -e "$DEST" ] || exit 1
ROOT=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
cd "$ROOT"
mkdir -p "$DEST"
for path in cmd pkg packaging vendor docs go.mod go.sum LICENSE README.md VERSION;do
 [ -e "$path" ] || { echo "Missing source input: $path" >&2; exit 1; }
 cp -R "$path" "$DEST/"
done
[ -z "$(find "$DEST" -type l -print -quit)" ] || { echo 'Source symlinks are unsupported.' >&2; exit 1; }
find "$DEST" -name .DS_Store -delete
printf '%s\n' "$DEST"
