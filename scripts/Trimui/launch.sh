#!/bin/sh
CUR_DIR="$(dirname "$0")"
cd "$CUR_DIR"/grout || exit 1

export CFW=TRIMUI
export LD_LIBRARY_PATH=$CUR_DIR/lib:$LD_LIBRARY_PATH

./grout
