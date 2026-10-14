#!/bin/sh
# Grout launcher for Leaf on the Miniloong Pocket 1.
# Leaf lists every pak in Apps/mlp1 and runs its launch.sh with the platform
# environment set (PLATFORM, SDCARD_PATH, LOGS_PATH, UMRK_RUNTIME_PATH...).

PAK_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
# The release archive unpacks into Apps/mlp1, so updates are staged there.
APPS_DIR="$(dirname "$PAK_DIR")"
PLATFORM="${PLATFORM:-mlp1}"

# Started outside the launcher (over SSH, say), pick up the environment Leaf
# would have handed us, the way Leaf's own apps do.
if [ -z "$SDCARD_PATH" ]; then
    if [ -n "$UMRK_ENV_FILE" ] && [ -f "$UMRK_ENV_FILE" ]; then
        . "$UMRK_ENV_FILE"
    else
        for root in /mnt/sdcard /media/sdcard1; do
            env_sh="$root/.system/leaf/platforms/$PLATFORM/launcher/env.sh"
            if [ -f "$env_sh" ]; then
                . "$env_sh"
                break
            fi
        done
    fi
fi
SD="${SDCARD_PATH:-$(cd "$PAK_DIR/../../.."; pwd)}"

cd "$PAK_DIR" || exit 1

# Leaf collects app logs in one place; keep a copy of ours there too, so a
# loader error or a panic is visible from Central Scrutinizer's log viewer.
LOG_DIR="${LOGS_PATH:-$SD/.userdata/$PLATFORM/logs}"
mkdir -p "$LOG_DIR" 2>/dev/null || LOG_DIR="$PAK_DIR/logs"
mkdir -p "$LOG_DIR"
exec >>"$LOG_DIR/grout.txt" 2>&1
echo "=== Launching Grout at $(date) ==="

# Apply pending update. It sits in a hidden folder, which Leaf does not list.
if [ -d "$APPS_DIR/.update" ]; then
    cp -rf "$APPS_DIR"/.update/* "$APPS_DIR" || exit 1
    rm -rf "$APPS_DIR/.update"
fi

# Stock LoongOS has no default CA path, so HTTPS to a RomM server needs Leaf's
# bundle (the same one Leaf's own apps use).
CA="$SD/.system/leaf/platforms/$PLATFORM/launcher/res/certs/cacert.pem"
if [ -z "$SSL_CERT_FILE" ] && [ -f "$CA" ]; then
    export SSL_CERT_FILE="$CA"
fi

export CFW=LEAF
export BASE_PATH="$SD"
export LD_LIBRARY_PATH="$PAK_DIR/lib:$LD_LIBRARY_PATH"
chmod +x ./grout 2>/dev/null

./grout

exit 0
