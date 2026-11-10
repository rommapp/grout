#!/bin/sh
# Syncs the save for a game if a network the device knows is in range.
#
# Called by the lines Grout adds to each emulator's launch.sh when Sync on Launch
# is turned on, and by the "Sync save" entry in the game list's Y menu, with the
# ROM path as its argument:
#
#   sync_on_launch.sh <rom>            just before the game starts
#   sync_on_launch.sh --after <rom>    once the game has been quit
#   sync_on_launch.sh --now <rom>      because the player asked, with messages
#
# Before and after a game this must never get in the way of the game or the
# menu, so every way out is exit 0 and nothing is shown. On request it says what
# happened.
#
# Wi-Fi is never switched on or off from here. With it off, there is nothing to
# sync over.

MODE=before
case "$1" in
    --after) MODE=after; shift ;;
    --now) MODE=now; shift ;;
esac

ROM="$1"
[ -n "$ROM" ] || exit 0

APP_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$APP_DIR/grout" || exit 0

export PATH="/mnt/SDCARD/.tmp_update/bin:$PATH"

LOG_DIR="$APP_DIR/grout/logs"
LOG="$LOG_DIR/sync_on_launch.log"

uptime_seconds() {
    cut -d. -f1 "${GROUT_UPTIME_FILE:-/proc/uptime}" 2> /dev/null || echo 0
}

# A line in the log, for working out afterwards why a sync did not happen. The
# device may have no clock, so the time since boot goes with it.
log() {
    mkdir -p "$LOG_DIR" 2> /dev/null
    if [ -f "$LOG" ] && [ "$(wc -c < "$LOG")" -gt 100000 ]; then
        tail -n 200 "$LOG" > "$LOG.tmp" && mv "$LOG.tmp" "$LOG"
    fi
    echo "up=$(uptime_seconds)s [$MODE] $*" >> "$LOG"
}

RESULT="$APP_DIR/grout/launch_sync_result.txt"
PANEL=0

# While the player waits on a sync they asked for, a message that stays up.
panel_on() {
    [ "$MODE" = now ] || return 0
    infoPanel --title "SYNC SAVE" --message "$1" --persistent &
    PANEL=1
}

panel_off() {
    [ "$PANEL" -eq 1 ] || return 0
    touch /tmp/dismiss_info_panel
    # Let the panel go before the next one is drawn over it.
    sleep 1
    PANEL=0
}

# Shows a message, but only when the player asked for the sync.
say() {
    [ "$MODE" = now ] || return 0
    panel_off
    infoPanel --title "SYNC SAVE" --message "$1" --auto
}

# Leaves quietly: before or after a game, say nothing; on request, say why.
stop() {
    log "stopped: $2"
    say "$1"
    exit 0
}

case "$MODE" in
    now) ;;
    after)
        # Cheapest check first: the setting is off for most people.
        grep -q '"sync_on_launch": *"before_and_after"' config.json 2>/dev/null || exit 0
        # Holding power quits the game on the way to switching off. Nobody
        # wants the shutdown held up, and this session's save goes up the next
        # time the game is launched.
        [ ! -f /tmp/.offOrder ] || exit 0
        ;;
    *)
        grep -Eq '"sync_on_launch": *"(before|before_and_after)"' config.json 2>/dev/null || exit 0
        ;;
esac

WPA_CONF="${GROUT_WPA_CONF:-/appconfigs/wpa_supplicant.conf}"

# No saved networks means there is nothing to join, however long we wait.
grep -q '^[[:space:]]*ssid=' "$WPA_CONF" 2>/dev/null || stop "No saved Wi-Fi networks.\nAdd one in Onion's settings." "no saved networks"

# Wi-Fi off: wpa_supplicant is not running.
ps | grep -v grep | grep -q wpa_supplicant || stop "Wi-Fi is off." "wpa_supplicant is not running"

has_ip() {
    ifconfig wlan0 2>/dev/null | grep -q 'inet addr:'
}

# wpa_supplicant joins a known network on its own once it sees one, but a device
# that has just started, or woken, takes a while to get there. A sync the player
# asked for waits longest; before or after a game the wait is kept short unless
# the device only just started, so that being away from every saved network
# costs a few seconds, not half a minute.
WAIT_SECONDS=8
if [ "$MODE" = now ] || [ "$(uptime_seconds)" -lt 180 ]; then
    WAIT_SECONDS=30
fi

waited=0
if ! has_ip; then
    panel_on "Connecting to Wi-Fi..."
    while ! has_ip; do
        if [ "$waited" -ge "$WAIT_SECONDS" ]; then
            stop "Wi-Fi did not connect\nin ${WAIT_SECONDS} seconds.\nTry again in a moment." "no IP on wlan0 after ${WAIT_SECONDS}s ($(ifconfig wlan0 2>&1 | head -3 | tr '\n' ' '))"
        fi
        sleep 1
        waited=$((waited + 1))
    done
    panel_off
fi
log "connected after ${waited}s"

export CFW=ONION
export LD_LIBRARY_PATH="$APP_DIR/grout/lib:$LD_LIBRARY_PATH"

started=$(uptime_seconds)
case "$MODE" in
    now)
        panel_on "Syncing..."
        rm -f "$RESULT"
        ./grout --sync-rom "$ROM" --now > /dev/null 2>&1
        code=$?
        panel_off
        log "grout exited $code after $(($(uptime_seconds) - started))s"
        # Grout leaves a message saying what the sync did, when it got far enough
        # to know. Otherwise all there is to go on is how it ended.
        if [ -s "$RESULT" ]; then
            say "$(sed ':a;N;$!ba;s/\n/\\n/g' "$RESULT")"
        else
            case "$code" in
                12) say "Could not reach RomM." ;;
                13) say "This game is not matched\nto a game in RomM." ;;
                15) say "Register this device in\nGrout (Settings > Save Sync)." ;;
                17) say "This game has saves in two\ncore folders. In Grout, open\nits Game Options and pick\nResolve Saves." ;;
                0)  say "Grout did not report\nwhat happened.\nSee the Grout logs." ;;
                *)  say "Sync failed.\nSee the Grout logs." ;;
            esac
        fi
        ;;
    after)
        # After a game the save on the device is the newest there is, so only
        # send it.
        ./grout --sync-rom "$ROM" --upload-only > /dev/null 2>&1
        log "grout exited $? after $(($(uptime_seconds) - started))s"
        ;;
    *)
        ./grout --sync-rom "$ROM" > /dev/null 2>&1
        log "grout exited $? after $(($(uptime_seconds) - started))s"
        ;;
esac
exit 0
