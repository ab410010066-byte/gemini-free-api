#!/bin/bash
set -e

FF_DIR="$HOME/Library/Application Support/Firefox"
PROFILE_DIR=""

if [ -f "$FF_DIR/profiles.ini" ]; then
    REL_PATH=$(grep -E 'Path=Profiles/.*\.default-release' "$FF_DIR/profiles.ini" | head -n 1 | cut -d= -f2 || true)
    if [ -n "$REL_PATH" ] && [ -d "$FF_DIR/$REL_PATH" ]; then
        PROFILE_DIR="$FF_DIR/$REL_PATH"
    fi
fi

if [ -z "$PROFILE_DIR" ]; then
    for d in "$FF_DIR/Profiles/"*.default-release "$FF_DIR/Profiles/"*.default* "$FF_DIR/Profiles/"*; do
        if [ -f "$d/cookies.sqlite" ]; then
            PROFILE_DIR="$d"
            break
        fi
    done
fi

if [ -z "$PROFILE_DIR" ] || [ ! -f "$PROFILE_DIR/cookies.sqlite" ]; then
    echo "[!] Could not find active Firefox profile under $FF_DIR"
    exit 1
fi

DST="$(cd "$(dirname "$0")" && pwd)/data"
mkdir -p "$DST"

cp "$PROFILE_DIR/cookies.sqlite" "$DST/firefox_cookies.sqlite"
[ -f "$PROFILE_DIR/cookies.sqlite-wal" ] && cp "$PROFILE_DIR/cookies.sqlite-wal" "$DST/firefox_cookies.sqlite-wal" || true
[ -f "$PROFILE_DIR/cookies.sqlite-shm" ] && cp "$PROFILE_DIR/cookies.sqlite-shm" "$DST/firefox_cookies.sqlite-shm" || true
[ -f "$PROFILE_DIR/containers.json" ] && cp "$PROFILE_DIR/containers.json" "$DST/containers.json" || true

echo "[+] Firefox cookies successfully synced from $PROFILE_DIR to $DST/firefox_cookies.sqlite"
