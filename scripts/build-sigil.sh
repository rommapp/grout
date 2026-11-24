#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

rm -rf third_party/argosy-sigil/build
cmake -B third_party/argosy-sigil/build -S third_party/argosy-sigil \
  -DSIGIL_BUILD_CLI=OFF -DSIGIL_BUILD_TESTS=OFF -DCMAKE_BUILD_TYPE=Release "$@"
cmake --build third_party/argosy-sigil/build --target sigil --parallel
