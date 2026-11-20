#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

OUT=build/libgrout
mkdir -p "$OUT"
CGO_ENABLED=1 go build -buildmode=c-shared -o "$OUT/libgrout.so" ./capi
cc -std=c99 -Wall -Werror -I "$OUT" -o "$OUT/smoke" capi/smoke/smoke.c -L "$OUT" -lgrout

SCRATCH="$(mktemp -d)"
trap 'rm -rf "$SCRATCH"' EXIT

LD_LIBRARY_PATH="$OUT" DYLD_LIBRARY_PATH="$OUT" "$OUT/smoke" "$SCRATCH"
