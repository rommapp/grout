#!/bin/bash
CUR_DIR="$(dirname "$0")"
FLAG_FILE="./es_restart_request"
cd "$CUR_DIR/Grout" || exit 1

# Apply pending update
if [ -d "../.update" ]; then
    cp -rf ../.update/* ..
    rm -rf ../.update
fi

export CFW=ARKOS
export LD_LIBRARY_PATH="$CUR_DIR/Grout/lib:$LD_LIBRARY_PATH"
chmod +x ./grout

./grout

if [ -f "$FLAG_FILE" ]; then
    rm -f "$FLAG_FILE"
    killall emulationstation
    nohup bash -c "sleep 3 && batocera-es-swissknife --restart" >/dev/null 2>&1 &
fi

exit 0
