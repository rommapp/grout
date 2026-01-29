#!/bin/bash
CUR_DIR="$(dirname "$0")"
FLAG_FILE="./es_restart_request"
cd "$CUR_DIR" || exit 1

export CFW=KNULLI
export LD_LIBRARY_PATH=$CUR_DIR/lib:$LD_LIBRARY_PATH

./grout

if [ -f "$FLAG_FILE" ]; then
    rm -f "$FLAG_FILE"
    # batocera-es-swissknife --update-gamelists
    batocera-es-swissknife --restart
fi

exit 0