#!/bin/sh
CUR_DIR="$(dirname "$0")"
cd "$CUR_DIR" || exit 1

export CFW=NEXTUI
export LD_LIBRARY_PATH=$CUR_DIR/lib:$LD_LIBRARY_PATH

./grout
