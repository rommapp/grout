#!/bin/bash
CUR_DIR="$(dirname "$0")"
FLAG_FILE="./rocknix_restart_request"
cd "$CUR_DIR/Grout" || exit 1

export CFW=ROCKNIX
export LD_LIBRARY_PATH="$CUR_DIR/Grout/lib:$LD_LIBRARY_PATH"
export FLIP_FACE_BUTTONS=1

./grout

if [ -f "$FLAG_FILE" ]; then
    rm -f "$FLAG_FILE"
    batocera-es-swissknife --restart
fi

exit 0