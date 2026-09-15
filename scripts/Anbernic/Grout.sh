#!/bin/bash
# Grout launcher for the Anbernic stock OS.
# The stock launcher lists every *.sh in Roms/APPS and runs it with $0 set to its path,
# so the payload lives in the sibling "grout" folder (directories are not listed as apps).

progdir="$(cd "$(dirname "$0")"; pwd)"
# Roms/APPS -> the root of the card this copy of Grout was installed on (/mnt/mmc or /mnt/sdcard)
basedir="$(cd "$progdir/../.."; pwd)"

cd "$progdir/grout" || exit 1

# The stock launcher runs apps with stdout and stderr sent to /dev/null, so keep our own copy:
# without it a panic or a loader error leaves no trace at all. Stock apps do the same.
mkdir -p logs
exec >logs/stdout.log 2>&1

# Apply pending update
if [ -d "$basedir/.update" ]; then
    cp -rf "$basedir"/.update/* "$basedir"
    rm -rf "$basedir/.update"
fi

export CFW=ANBERNIC
export BASE_PATH="$basedir"
export LD_LIBRARY_PATH="$progdir/grout/lib:$LD_LIBRARY_PATH"
chmod +x ./grout 2>/dev/null

./grout

exit 0
