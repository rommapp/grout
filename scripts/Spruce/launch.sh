#!/bin/sh
CUR_DIR="$(dirname "$0")"
cd "$CUR_DIR"/grout || exit 1

# Apply pending update
if [ -d "../../.update" ]; then
    cp -rf ../../.update/* ../..
    rm -rf ../../.update
fi

export CFW=SPRUCE
export LD_LIBRARY_PATH=$CUR_DIR/grout/lib:$LD_LIBRARY_PATH

./grout
