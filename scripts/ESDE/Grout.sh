#!/bin/bash
CUR_DIR="$(cd "$(dirname "$0")" && pwd)"
APP_DIR="$CUR_DIR/Grout"
LOCK_DIR="${XDG_RUNTIME_DIR:-/tmp}/grout-esde.lock"

if ! mkdir "$LOCK_DIR" 2>/dev/null; then
    if [ -f "$LOCK_DIR/pid" ] && kill -0 "$(cat "$LOCK_DIR/pid")" 2>/dev/null; then
        exit 0
    fi
    rm -rf "$LOCK_DIR"
    mkdir "$LOCK_DIR" 2>/dev/null || exit 0
fi
printf '%s\n' "$$" > "$LOCK_DIR/pid"
trap 'rm -rf "$LOCK_DIR"' EXIT

# Apply pending update
if [ -d "$CUR_DIR/.update" ]; then
    cp -rf "$CUR_DIR/.update/"* "$CUR_DIR/"
    rm -rf "$CUR_DIR/.update"
fi

cd "$APP_DIR" || exit 1

# EmuDeck's ES-DE ports folder is normally <Emulation>/roms/ports.
export CFW=ESDE
export BASE_PATH="$(cd "$CUR_DIR/../.." && pwd)"
export LD_LIBRARY_PATH="$APP_DIR/lib:$LD_LIBRARY_PATH"

# Steam Deck uses Xbox-style face buttons, so use direct A=A/B=B mappings.
export FLIP_FACE_BUTTONS=1

# Prefer SDL's game controller API and ignore duplicate keyboard/raw joystick events.
export DISABLE_KEYBOARD_INPUT=1
export DISABLE_JOYSTICK_INPUT=1

# Grout uses its own gamepad-driven keyboard. On Steam Deck, SDL/Steam can
# also surface the Steam keyboard, which can interfere with Grout's keyboard.
export SDL_ENABLE_SCREEN_KEYBOARD=0
export SDL_IME_SHOW_UI=0

./grout

exit 0
