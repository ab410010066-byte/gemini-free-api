#!/bin/bash
set -e
SRC="$HOME/Library/Application Support/Firefox/Profiles/73m0y9v6.default-release"
DST="$(cd "$(dirname "$0")" && pwd)/data"
mkdir -p "$DST"
if [ -f "$SRC/cookies.sqlite" ]; then
    cp "$SRC/cookies.sqlite" "$DST/firefox_cookies.sqlite"
    [ -f "$SRC/cookies.sqlite-wal" ] && cp "$SRC/cookies.sqlite-wal" "$DST/firefox_cookies.sqlite-wal" || true
    [ -f "$SRC/cookies.sqlite-shm" ] && cp "$SRC/cookies.sqlite-shm" "$DST/firefox_cookies.sqlite-shm" || true
    [ -f "$SRC/containers.json" ] && cp "$SRC/containers.json" "$DST/containers.json" || true
    echo "✓ Firefox cookies successfully synced to $DST/firefox_cookies.sqlite"
else
    echo "x Firefox profile not found at $SRC"
fi
