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

# Shows a message, but only when the player asked for the sync.
say() {
    [ "$MODE" = now ] || return 0
    infoPanel --title "SYNC SAVE" --message "$1" --auto
}

# Leaves quietly: before or after a game, say nothing; on request, say why.
stop() {
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
WAIT_SECONDS=8

# No saved networks means there is nothing to join, however long we wait.
grep -q '^[[:space:]]*ssid=' "$WPA_CONF" 2>/dev/null || stop "No saved Wi-Fi networks.\nAdd one in Onion's settings."

# Wi-Fi off: wpa_supplicant is not running.
ps | grep -v grep | grep -q wpa_supplicant || stop "Wi-Fi is off."

has_ip() {
    ifconfig wlan0 2>/dev/null | grep -q 'inet addr:'
}

# wpa_supplicant joins a known network on its own once it sees one. Give it a
# few seconds, which is also what a device that has just woken needs; if
# nothing is joined by then, none of the known networks is here.
waited=0
while ! has_ip; do
    [ "$waited" -ge "$WAIT_SECONDS" ] && stop "No saved Wi-Fi network\nis in range."
    sleep 1
    waited=$((waited + 1))
done

export CFW=ONION
export LD_LIBRARY_PATH="$APP_DIR/grout/lib:$LD_LIBRARY_PATH"

case "$MODE" in
    now)
        infoPanel --title "SYNC SAVE" --message "Syncing..." --persistent &
        ./grout --sync-rom "$ROM" --now > /dev/null 2>&1
        code=$?
        touch /tmp/dismiss_info_panel
        # Let the panel go before the next one is drawn over it.
        sleep 1
        case "$code" in
            0)  say "Save is up to date." ;;
            10) say "Newer save downloaded." ;;
            16) say "Newer save downloaded.\nThe game will start from it,\nnot from its old resume state." ;;
            11) say "Save uploaded." ;;
            12) say "Could not reach RomM." ;;
            13) say "This game is not matched\nto a game in RomM." ;;
            14) say "A conflict needs you.\nResolve it in Grout > Sync." ;;
            15) say "Register this device in\nGrout (Settings > Save Sync)." ;;
            *)  say "Sync failed.\nSee the Grout logs." ;;
        esac
        ;;
    after)
        # After a game the save on the device is the newest there is, so only
        # send it.
        ./grout --sync-rom "$ROM" --upload-only > /dev/null 2>&1
        ;;
    *)
        ./grout --sync-rom "$ROM" > /dev/null 2>&1
        ;;
esac
exit 0
