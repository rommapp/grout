#!/bin/sh
# Syncs the save for the game about to start, if a network the device knows is
# in range. Called by the line Grout adds to each emulator's launch.sh when
# Sync on Launch is turned on, with the ROM path as its only argument.
#
# This must never get in the way of the game, so every way out is exit 0.
#
# Wi-Fi is never switched on or off from here. With it off, there is nothing to
# sync over and the game starts straight away.

ROM="$1"
[ -n "$ROM" ] || exit 0

APP_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$APP_DIR/grout" || exit 0

# Cheapest check first: the setting is off for most people.
grep -q '"sync_on_launch": *true' config.json 2>/dev/null || exit 0

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

./grout --sync-rom "$ROM" > /dev/null 2>&1
exit 0
