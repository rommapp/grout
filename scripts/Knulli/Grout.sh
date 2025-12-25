#!/bin/bash
APP_DIR="$(dirname "$0")"
cd "$APP_DIR" || exit 1

export CFW=KNULLI
export LD_LIBRARY_PATH=$APP_DIR/lib

./grout
