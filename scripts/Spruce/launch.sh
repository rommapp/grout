#!/bin/sh
CUR_DIR="$(dirname "$0")"
cd "$CUR_DIR"/grout || exit 1

export CFW=SPRUCE
export INPUT_CAPTURE=true
#export LD_LIBRARY_PATH=$CUR_DIR/grout/lib:$LD_LIBRARY_PATH

case "$PLATFORM" in

############################################################
# A30
############################################################
    "A30" )
        echo "A30 detected, setting up environment variables for SDL2"
        export LD_LIBRARY_PATH="/mnt/SDCARD/spruce/a30/sdl2:$LD_LIBRARY_PATH"
        export PYSDL2_DLL_PATH="/mnt/SDCARD/spruce/a30/sdl2"

        ./grout32
    ;;

############################################################
# Brick / SmartPro / SmartProS
############################################################
    "Brick" | "SmartPro" | "SmartProS" )
        echo "Brick/SmartPro/SmartProS detected, setting up environment variables for SDL2"
#        export LD_LIBRARY_PATH="/mnt/SDCARD/spruce/brick/sdl2:$LD_LIBRARY_PATH"
#        export PYSDL2_DLL_PATH="/mnt/SDCARD/spruce/brick/sdl2"
        export LD_LIBRARY_PATH="$CUR_DIR/grout/lib:$LD_LIBRARY_PATH"
        ./grout64
    ;;

############################################################
# Miyoo Flip
############################################################
    "Flip" )
#        mkdir "$CUR_DIR"/grout/lib32/
#        ln -s /mnt/SDCARD/App/PyUI/dll/libSDL2_gfx-1.0.so "$CUR_DIR"/grout/lib32/libSDL2_gfx-1.0.so.0
        echo "Miyoo Flip detected, setting up environment variables for SDL2"
        # shellcheck disable=SC2086
#        export LD_LIBRARY_PATH="$CUR_DIR/grout/lib:/mnt/SDCARD/App/PyUI/dll:$LD_LIBRARY_PATH"
#        export LD_LIBRARY_PATH="$CUR_DIR/grout/lib32:$LD_LIBRARY_PATH"
        export LD_LIBRARY_PATH="$CUR_DIR/grout/lib:$LD_LIBRARY_PATH"
#        export PYSDL2_DLL_PATH="/mnt/SDCARD/App/PyUI/dll"
        ./grout64
    ;;
############################################################
# Miyoo Mini Flip
############################################################
    "MiyooMini" )
        echo "Miyoo Mini detected, setting up environment variables for SDL2"
        export PYSDL2_DLL_PATH="/mnt/SDCARD/spruce/miyoomini/lib"
        export LD_LIBRARY_PATH="/mnt/SDCARD/spruce/bin/python/lib:$LD_LIBRARY_PATH"
        export LD_LIBRARY_PATH="/mnt/SDCARD/spruce/miyoomini/lib:$LD_LIBRARY_PATH"

        export SDL_VIDEODRIVER=mmiyoo
        export SDL_AUDIODRIVER=mmiyoo
        export EGL_VIDEODRIVER=mmiyoo
        export SDL_MMIYOO_DOUBLE_BUFFER=1

        ./grout32
    ;;
esac
