#!/bin/sh
CUR_DIR="$(dirname "$0")"
cd "$CUR_DIR" || exit 1

# Apply pending update
if [ -d ".update" ]; then
    cp -rf .update/* .
    rm -rf .update
fi

export CFW=NEXTUI
# This $PLATFORM is automatically set by NextUI, we map it another variable just to remember where it comes from.
export NEXTUI_DEVICE="$PLATFORM"
export LD_LIBRARY_PATH=$CUR_DIR/lib:$LD_LIBRARY_PATH

./grout
