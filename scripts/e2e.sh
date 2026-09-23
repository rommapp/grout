#!/usr/bin/env bash
#
# Run grout's UI end to end, against a real RomM.
#
# Everything happens in containers, so this behaves the same on macOS and on a
# CI runner and needs nothing installed but Docker.
#
#   scripts/e2e.sh                             everything
#   scripts/e2e.sh -run TestPairedDevice       one test
#   scripts/e2e.sh -run 'TestFirstLaunch/MUOS' one firmware
#
# Screenshots land in test/e2e/screenshots/ and are worth looking at when
# something fails.
set -euo pipefail

cd "$(dirname "$0")/.."

COMPOSE=(docker compose -f test/e2e/compose.yml)
SHOTS="$PWD/test/e2e/screenshots"

cleanup() {
  echo "==> tearing down"
  "${COMPOSE[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT

mkdir -p "$SHOTS"
rm -rf "${SHOTS:?}/"*

echo "==> starting RomM"
"${COMPOSE[@]}" up -d --wait db romm

echo "==> running"
"${COMPOSE[@]}" run --rm --build tests \
  go test -tags=e2e -count=1 -v -timeout 20m ./... "$@"

echo "==> screenshots in test/e2e/screenshots/"
