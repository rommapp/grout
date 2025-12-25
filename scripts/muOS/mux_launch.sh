#!/bin/bash
# HELP: Grout
# ICON: retroarch
# GRID: RetroArch

. /opt/muos/script/var/func.sh
echo app >/tmp/act_go

GOV_GO="/tmp/gov_go"
[ -e "$GOV_GO" ] && cat "$GOV_GO" >"$(GET_VAR "device" "cpu/governor")"

APP_DIR="$(dirname "$0")"
cd "$APP_DIR" || exit 1

export CFW=MUOS
export LD_LIBRARY_PATH=$APP_DIR/lib

./grout
