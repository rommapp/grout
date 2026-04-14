#!/bin/bash
CUR_DIR="$(dirname "$0")"
cd "$CUR_DIR/Grout" || exit 1

# Apply pending update
if [ -d "../.update" ]; then
    cp -rf ../.update/* ..
    rm -rf ../.update
fi

export CFW=RETRODECK
export LD_LIBRARY_PATH="$CUR_DIR/Grout/lib:$LD_LIBRARY_PATH"

./grout

exit 0
