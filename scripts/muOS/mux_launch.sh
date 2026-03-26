#!/bin/bash
# HELP: Grout
# ICON: grout
# GRID: grout

. /opt/muos/script/var/func.sh
echo app >/tmp/act_go

GOV_GO="/tmp/gov_go"
[ -e "$GOV_GO" ] && cat "$GOV_GO" >"$(GET_VAR "device" "cpu/governor")"

CUR_DIR="$(dirname "$0")"
GROUT_ICON_PATH="${CUR_DIR}/resources/grout.png"

CURRENT_THEME_FILE="/opt/muos/config/theme/active"
# If the config file doesn't exist, copy the icon to the active theme directory
# MuOS version < 202601.0
ICON_DIR=/opt/muos/share/theme/active/glyph/muxapp/
cp "${GROUT_ICON_PATH}" "${ICON_DIR}/grout.png"

THEME=$(cat "$CURRENT_THEME_FILE")
if [ -n "$THEME" ]; then

    # Possible paths to copy the icon to, depends on one or two SD cards setup
    TARGETS=(
        "/opt/muos/browse/SD1 (mmc)/MUOS/theme/${THEME}/glyph/muxapp"
        "/opt/muos/browse/SD2 (sdcard)/MUOS/theme/${THEME}/glyph/muxapp"
    )

    for DIR in "${TARGETS[@]}"; do
        if [ -d "$DIR" ]; then
            cp "$GROUT_ICON_PATH" "$DIR/grout.png"
            if [ $? -eq 0 ]; then
                echo "Icon copied to $DIR"
            else
                echo "Failed to copy to $DIR"
            fi
        else
            echo "Dir $DIR not found, skipping."
        fi
    done
fi

cd "$CUR_DIR" || exit 1

# Apply pending update
if [ -d "../.update" ]; then
    cp -rf ../.update/* ..
    rm -rf ../.update
fi

export CFW=MUOS
export LD_LIBRARY_PATH=$CUR_DIR/lib:$LD_LIBRARY_PATH

./grout
