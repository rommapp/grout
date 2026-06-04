#!/bin/bash
CUR_DIR="$(realpath "$(dirname "$0")")"
cd "$CUR_DIR/Grout" || exit 1

export CFW=RETRODECK
export LD_LIBRARY_PATH="$CUR_DIR/Grout/lib:$LD_LIBRARY_PATH"
export GROUT_DATA_DIR="$XDG_CONFIG_HOME/Grout"
export GROUT_CACHE_DIR="$XDG_CONFIG_HOME/Grout/.cache"
export GROUT_TMP_DIR="/tmp/Grout"
#export GROUT_BACKUP_DIR="$GROUT_TMP_DIR/backup"
export GROUT_UPDATE_DIR="$GROUT_TMP_DIR/update"

mkdir -p "$GROUT_DATA_DIR" "$GROUT_CACHE_DIR" "$GROUT_TMP_DIR"

# Apply pending update
if [ -d "$GROUT_UPDATE_DIR" ]; then
    cp -rf "$GROUT_UPDATE_DIR/"* "$CUR_DIR/"
    rm -rf "$GROUT_UPDATE_DIR"
fi

./grout

exit 0
