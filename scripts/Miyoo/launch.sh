#!/bin/bash
# Grout launcher for Miyoo Mini Plus
# Requires steward-fu's patched SDL2 libraries in libs/ directory

APP_DIR="$(dirname "$0")"
cd "$APP_DIR" || exit 1

# Stop the main UI to prevent conflicts
kill -STOP $(pidof MainUI) 2>/dev/null

# Required environment variables for steward-fu's SDL2 port
# Driver names from steward-fu's SDL2 source: "Mini" for video, "Miyoo Mini" for audio
export SDL_VIDEODRIVER=Mini
export SDL_AUDIODRIVER="Miyoo Mini"
export EGL_VIDEODRIVER=Mini

# Tell grout which CFW we're running on
export CFW=MIYOO

# Set library path to include bundled SDL2 libraries
export LD_LIBRARY_PATH=$APP_DIR/libs:/config/lib:/customer/lib:$LD_LIBRARY_PATH

# Run grout
./grout

# Resume the main UI
kill -CONT $(pidof MainUI) 2>/dev/null
