#!/bin/bash
# HELP: Grout
# ICON: grout
# GRID: grout

. /opt/muos/script/var/func.sh
echo app >/tmp/act_go

GOV_GO="/tmp/gov_go"
[ -e "$GOV_GO" ] && cat "$GOV_GO" >"$(GET_VAR "device" "cpu/governor")"

CUR_DIR="$(dirname "$0")"
ICON_DIR=/opt/muos/share/theme/active/glyph/muxapp/
cp "${CUR_DIR}/resources/grout.png" "${ICON_DIR}/grout.png"

cd "$CUR_DIR" || exit 1

export CFW=MUOS
export LD_LIBRARY_PATH=$CUR_DIR/lib:$LD_LIBRARY_PATH

./grout
