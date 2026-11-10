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
    label="$MODE"
    [ "$SHUTTING_DOWN" -eq 1 ] && label="$MODE, power off"
    echo "up=$(uptime_seconds)s [$label] $*" >> "$LOG"
}

RESULT="$APP_DIR/grout/launch_sync_result.txt"
PANEL=0
SHUTTING_DOWN=0

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
        # Holding power quits the game on the way to switching off. The save
        # still goes up, since there may be no other chance before the next
        # time the device is used, but it must not hold the shutdown up: Onion's
        # key monitor forces it 30 seconds after the button was held, so the
        # upload gets a strict limit well inside that, never waits for Wi-Fi,
        # and is skipped when the battery is nearly flat.
        if [ -f /tmp/.offOrder ]; then
            [ ! -f /tmp/.lowBat ] || exit 0
            SHUTTING_DOWN=1
        fi
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

# Runs a command, and stops it if it takes longer than the limit, in seconds. It
# polls rather than sleeping in a helper, so nothing is left running once the
# command is done. Giving up on a sync is safe: files are replaced atomically, so
# the worst is a partly finished upload, and the next sync settles whatever RomM
# did or did not receive.
run_within() {
    limit="$1"
    shift
    "$@" > /dev/null 2>&1 &
    pid=$!
    waited_for=0
    while kill -0 "$pid" 2> /dev/null; do
        if [ "$waited_for" -ge "$limit" ]; then
            kill "$pid" 2> /dev/null
            break
        fi
        sleep 1
        waited_for=$((waited_for + 1))
    done
    wait "$pid" 2> /dev/null
    return $?
}

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
# Switching off: Wi-Fi was in use a moment ago, so it is either up or not worth
# waiting for.
[ "$SHUTTING_DOWN" -eq 1 ] && WAIT_SECONDS=0

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
        if [ "$SHUTTING_DOWN" -eq 1 ]; then
            run_within "${GROUT_SHUTDOWN_LIMIT:-15}" ./grout --sync-rom "$ROM" --upload-only
        else
            run_within "${GROUT_AFTER_LIMIT:-60}" ./grout --sync-rom "$ROM" --upload-only
        fi
        log "grout exited $? after $(($(uptime_seconds) - started))s"
        ;;
    *)
        # Before a game: it must not keep the game waiting for long, whatever
        # RomM or the network are doing.
        run_within "${GROUT_LAUNCH_LIMIT:-45}" ./grout --sync-rom "$ROM"
        log "grout exited $? after $(($(uptime_seconds) - started))s"
        ;;
esac
exit 0
