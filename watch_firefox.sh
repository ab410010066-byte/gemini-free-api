#!/bin/bash
# Real-time Firefox Cookie Sync Watcher
DIR="$(cd "$(dirname "$0")" && pwd)"
SYNC_SCRIPT="$DIR/sync_firefox.sh"
FF_DIR="$HOME/Library/Application Support/Firefox"
ADMIN_TOKEN="${ADMIN_TOKEN:-$1}"
if [ -z "$ADMIN_TOKEN" ] && [ -f "$DIR/.env" ]; then
    ADMIN_TOKEN=$(grep -E '^ADMIN_TOKEN=' "$DIR/.env" | cut -d= -f2- | tr -d '"' | tr -d "'" || true)
fi
LAST_MTIME=0

# Locate profile
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

FF_DB="$PROFILE_DIR/cookies.sqlite"
echo "[+] Watcher monitoring: $FF_DB"

while true; do
    if [ -f "$FF_DB" ]; then
        CURRENT_MTIME=$(stat -f "%m" "$FF_DB" 2>/dev/null || echo 0)
        if [ "$CURRENT_MTIME" -ne "$LAST_MTIME" ]; then
            "$SYNC_SCRIPT" >/dev/null 2>&1 || true
            if [ -n "$ADMIN_TOKEN" ]; then
                curl -s -X POST -H "Authorization: Bearer $ADMIN_TOKEN" http://localhost:8083/admin/api/cookies/import-firefox >/dev/null 2>&1 || true
            else
                curl -s -X POST http://localhost:8083/admin/api/cookies/import-firefox >/dev/null 2>&1 || true
            fi
            LAST_MTIME=$CURRENT_MTIME
        fi
    fi
    sleep 10
done
