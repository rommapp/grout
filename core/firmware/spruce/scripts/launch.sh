#!/bin/sh
CUR_DIR="$(dirname "$0")"
cd "$CUR_DIR"/grout || exit 1

# Apply pending update
if [ -d "../../.update" ]; then
    cp -rf ../../.update/* ../..
    rm -rf ../../.update
fi

export CFW=SPRUCE

# Sprig is an alternative OS developed by the Spruce team for the Miyoo Mini.
# Both Spruce and Sprig are compatible with our grout package, so we treat Sprig
# as MiyooMini to reuse the same platform configuration.
# Unlike Spruce (which always exports PLATFORM), Sprig may leave it unset.
if [ -d "/mnt/SDCARD/sprig" ] || [ -z "$PLATFORM" ]; then
    PLATFORM="MiyooMini"
fi

case "$PLATFORM" in

############################################################
# A30
############################################################
    "A30" )
        export LD_LIBRARY_PATH="/mnt/SDCARD/spruce/a30/sdl2:$LD_LIBRARY_PATH"
        export LD_LIBRARY_PATH="$CUR_DIR/grout/lib32/a30:$LD_LIBRARY_PATH"
        export SPRUCE_DEVICE="A30"
        ./grout32
    ;;

############################################################
# Brick / BrickPro / SmartPro / SmartProS
############################################################
    "Brick" | "SmartPro" | "SmartProS" | "BrickPro" )
        export LD_LIBRARY_PATH="$CUR_DIR/grout/lib64:$LD_LIBRARY_PATH"
        export SPRUCE_DEVICE="TRIMUI"
        ./grout64
    ;;

############################################################
# GKD Pixel 2
############################################################
    "Pixel2")
        export LD_LIBRARY_PATH="$CUR_DIR/grout/lib64:$LD_LIBRARY_PATH"
        export SPRUCE_DEVICE="PIXEL"
        ./grout64
    ;;


############################################################
# Miyoo Flip
############################################################
    "Flip" )
        export LD_LIBRARY_PATH="$CUR_DIR/grout/lib64:$LD_LIBRARY_PATH"
        export SPRUCE_DEVICE="MIYOOFLIP"
        ./grout64
    ;;

############################################################
# Miyoo Mini Flip
############################################################
    "MiyooMini" )
        export SDL_VIDEODRIVER=mmiyoo
        export SDL_AUDIODRIVER=mmiyoo
        export EGL_VIDEODRIVER=mmiyoo
        export SDL_MMIYOO_DOUBLE_BUFFER=1
        export IS_MIYOO=1
        export SPRUCE_DEVICE="MIYOOMINI"
        export LD_LIBRARY_PATH="$CUR_DIR/grout/lib32/miyoo:$LD_LIBRARY_PATH"
        ./grout32
    ;;

############################################################
# Anbernic H700
############################################################
    "Anbernic"* )
        export LD_LIBRARY_PATH="$CUR_DIR/grout/lib64:$LD_LIBRARY_PATH"
        export SPRUCE_DEVICE="H700"
        ./grout64
    ;;


############################################################
# Unknown
############################################################
    * )
        case "$(uname -m)" in
            arm*)
                echo "Unknown 32-bit Spruce platform: '${PLATFORM:-<empty>}' ($(uname -m))" >> grout.log
                exit 0
            ;;
        esac
        echo "Unrecognized Spruce platform: '${PLATFORM:-<empty>}', trying generic 64-bit launch" >> grout.log
        export LD_LIBRARY_PATH="$CUR_DIR/grout/lib64:$LD_LIBRARY_PATH"
        ./grout64
        status=$?
        if [ "$status" -ne 0 ]; then
            echo "grout64 exited with status $status on '${PLATFORM:-<empty>}'" >> grout.log
        fi
    ;;
esac
