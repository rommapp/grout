# End to end

Drives grout's real UI on a virtual display, one synthetic card per firmware.

```sh
scripts/e2e.sh                                    # every firmware
scripts/e2e.sh -run TestFirstLaunch/MUOS          # just one
```

Everything runs inside a container, so the only thing needed is Docker. The
same command works on macOS and on a CI runner.

Screenshots land in `test/e2e/screenshots/` and are uploaded as artifacts by
the workflow, on failure and success alike.

## How it works

`BASE_PATH` points grout at a temporary directory, so every path it computes
lands there and nothing touches the machine. `CFW` decides which firmware it
believes it is running on. Xvfb provides a display, openbox hands out focus,
and xdotool presses buttons through XTEST.

## Three things that will waste an afternoon

`ENVIRONMENT=DEV` is required. Without it grout loads the firmware's own
controller mapping, which replaces the toolkit's keyboard mapping, and every
keystroke is dropped in silence.

Keys must go through XTEST, which is what `xdotool key` does on its own.
Adding `--window` switches it to `XSendEvent`, and SDL ignores events it can
tell were synthesised.

A window manager is not optional. Without one nothing ever gains input focus,
so the keys land nowhere. openbox is enough.

## What it asserts

Grout's own structured log, not pixels. The log says what happened; a
screenshot only says what it looked like, and fonts and anti-aliasing make
that answer differ between machines. Screenshots are for a person to look at
when something fails.

## What it does not cover

Whether the platform tables are right. This tests what grout does given a
belief about a firmware; it cannot tell you the belief is correct. See
`docs/platforms/unverified.md`, which needs someone holding the device.
