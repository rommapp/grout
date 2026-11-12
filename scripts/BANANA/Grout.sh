#!/bin/bash
CUR_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$CUR_DIR/Grout" || exit 1

# Apply pending update
if [ -d "../.update" ]; then
    cp -rf ../.update/* ..
    rm -rf ../.update
fi

export CFW=BANANA
export LD_LIBRARY_PATH="$CUR_DIR/Grout/lib:$LD_LIBRARY_PATH"
chmod +x ./grout

exec ./grout
