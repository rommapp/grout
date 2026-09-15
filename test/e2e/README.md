# End to end

Runs the shipped binary against a real RomM, on a virtual display, with one
synthetic card per firmware.

```sh
scripts/e2e.sh                                 # everything
scripts/e2e.sh -run TestPairedDevice           # one test
scripts/e2e.sh -run 'TestFirstLaunch/MUOS'     # one firmware
```

Everything runs in containers, so the only thing needed is Docker and the same
command works on macOS and on a CI runner.

Screenshots land in `test/e2e/screenshots/` and are uploaded as artifacts by
the workflow, on failure and success alike.

## What is real and what is not

Real: the binary, SDL, the file I/O, the RomM API, the database, the platform
and rom records, the token.

Not real: the device. There is no framebuffer, no physical input, and no
firmware. Grout ships its own SDL and reads its layout from `BASE_PATH`, so
what a firmware supplies is a kernel, a screen, buttons and a directory
layout. The first three are stood in for; the fourth is exactly what is being
tested.

Also not real: the roms. They are a few bytes of nothing. Grout is being
tested on what it does with a library, not on what is in one.

## How a run goes

1. Compose starts MariaDB and RomM, with `library/` mounted as RomM's library.
2. `romm/provision.py` creates the first user, asks RomM to scan, and mints a
   client token.
3. Each test lays out a card, brings up Xvfb and openbox, and starts grout
   pointed at that card. A test that wants to start past the login screen has
   its card written with the token already in it.
4. Buttons are pressed with xdotool, and the test waits for grout to say what
   it did.

Nothing is persisted between runs: the database is on tmpfs and the cards are
temporary directories, so a test cannot pass because of something an earlier
one left behind.

## Four things that will waste an afternoon

`ENVIRONMENT=DEV` is required. Without it grout loads the firmware's own
controller mapping, which replaces the toolkit's keyboard mapping, and every
keystroke is dropped in silence.

Keys must go through XTEST, which is what `xdotool key` does on its own.
Adding `--window` switches it to `XSendEvent`, and SDL ignores events it can
tell were synthesised.

A window manager is not optional. Without one nothing ever takes input focus,
so the keys land nowhere. openbox is enough.

Grout logs errors and nothing else by default, so a seeded card asks for debug
logging. Waiting for a debug line on a card that did not would hang until the
timeout.

## What it asserts

Grout's own structured log, and the contents of the card. Not pixels: the log
says what happened, a screenshot only says what it looked like, and fonts and
anti-aliasing make that answer differ between machines.

Screenshots are taken once the screen stops changing, so a progress bar or a
transition is not what gets captured.

## What it does not cover

Whether the platform tables are right. This tests what grout does given a
belief about a firmware; it cannot tell you the belief is correct. See
`docs/platforms/unverified.md`, which needs someone holding the device.
