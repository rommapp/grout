#!/bin/sh
# Syncs the save for a game if a network the device knows is in range.
# Called by the lines Grout adds to each emulator's launch.sh when Sync on Launch
# is turned on, with the ROM path as its argument:
#
#   sync_on_launch.sh <rom>            just before the game starts
#   sync_on_launch.sh --after <rom>    once the game has been quit
#
# This must never get in the way of the game or the menu, so every way out is
# exit 0.
#
# Wi-Fi is never switched on or off from here. With it off, there is nothing to
# sync over and the game starts, or the menu returns, straight away.

AFTER=0
if [ "$1" = "--after" ]; then
    AFTER=1
    shift
fi

ROM="$1"
[ -n "$ROM" ] || exit 0

APP_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$APP_DIR/grout" || exit 0

# Cheapest check first: the setting is off for most people.
if [ "$AFTER" -eq 1 ]; then
    grep -q '"sync_on_launch": *"before_and_after"' config.json 2>/dev/null || exit 0
else
    grep -Eq '"sync_on_launch": *"(before|before_and_after)"' config.json 2>/dev/null || exit 0
fi

# Holding power quits the game on the way to switching off. Nobody wants the
# shutdown held up, and this session's save goes up the next time the game is
# launched.
[ ! -f /tmp/.offOrder ] || exit 0

WPA_CONF="${GROUT_WPA_CONF:-/appconfigs/wpa_supplicant.conf}"
WAIT_SECONDS=8

# No saved networks means there is nothing to join, however long we wait.
grep -q '^[[:space:]]*ssid=' "$WPA_CONF" 2>/dev/null || exit 0

# Wi-Fi off: wpa_supplicant is not running.
ps | grep -v grep | grep -q wpa_supplicant || exit 0

has_ip() {
    ifconfig wlan0 2>/dev/null | grep -q 'inet addr:'
}

# wpa_supplicant joins a known network on its own once it sees one. Give it a
# few seconds, which is also what a device that has just woken needs; if
# nothing is joined by then, none of the known networks is here.
waited=0
while ! has_ip; do
    [ "$waited" -ge "$WAIT_SECONDS" ] && exit 0
    sleep 1
    waited=$((waited + 1))
done

export CFW=ONION
export LD_LIBRARY_PATH="$APP_DIR/grout/lib:$LD_LIBRARY_PATH"

# After a game the save on the device is the newest there is, so only send it.
if [ "$AFTER" -eq 1 ]; then
    ./grout --sync-rom "$ROM" --upload-only > /dev/null 2>&1
else
    ./grout --sync-rom "$ROM" > /dev/null 2>&1
fi
exit 0
