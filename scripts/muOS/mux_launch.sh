#!/bin/bash
# HELP: Grout
# ICON: retroarch
# GRID: RetroArch

. /opt/muos/script/var/func.sh
echo app >/tmp/act_go

GOV_GO="/tmp/gov_go"
[ -e "$GOV_GO" ] && cat "$GOV_GO" >"$(GET_VAR "device" "cpu/governor")"

#!/bin/sh
APP_DIR="$(dirname "$0")"
cd "$APP_DIR" || exit 1

export LD_LIBRARY_PATH=$APP_DIR/resources/lib
export FALLBACK_FONT=$APP_DIR/resources/fonts/font.ttf
export INPUT_MAPPING_PATH=$APP_DIR/input_mapping.json
export ROM_DIRECTORY=/mnt/sdcard/ROMS

./grout
