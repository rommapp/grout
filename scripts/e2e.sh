#!/usr/bin/env bash
#
# Run grout's UI end to end on a virtual display.
#
# Everything happens inside a container, so this behaves the same on macOS and
# on a CI runner and needs nothing installed but Docker.
#
#   scripts/e2e.sh                      every firmware
#   scripts/e2e.sh -run TestFirstLaunch/MUOS   one of them
#
# Screenshots land in test/e2e/screenshots/ either way, and are worth looking
# at when something fails.
set -euo pipefail

cd "$(dirname "$0")/.."

IMAGE=grout-e2e
SHOTS="$PWD/test/e2e/screenshots"

echo "==> building the harness"
docker build -q -f test/e2e/Dockerfile -t "$IMAGE" . >/dev/null

mkdir -p "$SHOTS"
rm -rf "${SHOTS:?}/"*

echo "==> running"
docker run --rm \
  -v "$SHOTS:/screenshots" \
  -e GROUT_E2E_SCREENSHOTS=/screenshots \
  "$IMAGE" \
  go test -tags=e2e -count=1 -v -timeout 20m ./... "$@"

echo "==> screenshots in test/e2e/screenshots/"
