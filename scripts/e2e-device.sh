#!/usr/bin/env bash
#
# Run grout's UI end to end on a handheld, over ADB or SSH.
#
# This is the same suite scripts/e2e.sh runs in a container, pointed at real
# hardware instead: a real screen, real buttons, the real firmware's paths.
# What a container cannot answer is whether grout can talk to the device it was
# built for, and that is the whole of what this is for.
#
#   GROUT_DEVICE_BINARY=/mnt/mmc/MUOS/application/Grout/grout \
#   GROUT_E2E_DEVICE_CFW=MUOS \
#   ROMM_URL=http://10.0.0.2:8080 ROMM_TOKEN=... ROMM_DEVICE_ID=... \
#     scripts/e2e-device.sh
#
# Settings, all through the environment:
#
#   GROUT_E2E_DEVICE          adb, adb:<serial> or ssh:<user@host>. Default adb.
#   GROUT_E2E_DEVICE_CFW      the firmware the device runs, so the matrix cases
#                             for the other eleven skip instead of lying.
#   GROUT_DEVICE_BINARY       where grout is on the device. Required.
#   GROUT_E2E_DEVICE_CARD     scratch folder on the device. Default /tmp/grout-e2e.
#   GROUT_E2E_DEVICE_ARCH     linux arch for the keyboard helper, if uname is wrong.
#   GROUT_E2E_DEVICE_PREPARE  command to run first, to get the firmware's own
#                             menu off the screen and off the buttons.
#   GROUT_E2E_DEVICE_RESTORE  command to put it back afterwards.
#   ROMM_URL                  a RomM the device can reach.
#   ROMM_TOKEN, ROMM_DEVICE_ID
#                             a token and device from that RomM. Without them
#                             the tests that need a server skip.
#
# Screenshots are pulled off the framebuffer into test/e2e/screenshots/.
set -euo pipefail

cd "$(dirname "$0")/.."

export GROUT_E2E_DEVICE="${GROUT_E2E_DEVICE:-adb}"
export GROUT_E2E_SCREENSHOTS="${GROUT_E2E_SCREENSHOTS:-$PWD/test/e2e/screenshots}"

if [[ -z "${GROUT_DEVICE_BINARY:-}" ]]; then
  echo "set GROUT_DEVICE_BINARY to where grout is on the device" >&2
  echo "deploy it first with: task deploy:muos-sd1   (or next, muos-sd2, knulli)" >&2
  exit 2
fi

mkdir -p "$GROUT_E2E_SCREENSHOTS"

echo "==> ${GROUT_E2E_DEVICE}, running ${GROUT_E2E_DEVICE_CFW:-every firmware case}"
go test -tags=e2e -count=1 -v -timeout 30m ./test/e2e/ "$@"

echo "==> screenshots in test/e2e/screenshots/"
