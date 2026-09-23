# End to end

Runs the shipped binary against a real RomM, either on a virtual display with
one synthetic card per firmware, or on a handheld on the end of a cable.

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

## Walking a flow

Buttons are the ones a device has: arrow keys for the d-pad, `a` `b` `x` `y`
for the face buttons, Return for start, space for select, `h` for menu.

```go
s.press("a")  // the platform
s.press("a")  // the game
s.press("a")  // download it
s.awaitCardFile("ROMS/snes/Test Game (USA).sfc")
s.press("a")  // acknowledge the download
```

A press waits for the screen to stop moving first, so a key is never sent into
a half drawn screen.

Downloading only continues on its own when artwork is being fetched too.
Otherwise it waits on "Download Completed!", and the metadata is written after
that is acknowledged, which is what a person does on a device.

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

A card needs the folders a real one of that firmware would already have.
EmulationStation cards have `roms/tools`, and grout adds itself to the
gamelist in it at startup rather than creating it.

## What it asserts

Grout's own structured log, and the contents of the card. Not pixels: the log
says what happened, a screenshot only says what it looked like, and fonts and
anti-aliasing make that answer differ between machines.

Screenshots are taken once the screen stops changing, so a progress bar or a
transition is not what gets captured.

## On a real device

The same tests run against a handheld over ADB or SSH. Nothing in a test
changes: the target underneath it does.

```sh
GROUT_E2E_DEVICE=adb:<serial> \
  go test -tags e2e -run TestDeviceIsDrivable -v ./test/e2e/   # read only, safe

GROUT_DEVICE_BINARY=/mnt/mmc/MUOS/application/Grout/grout \
GROUT_E2E_DEVICE_CFW=MUOS \
ROMM_URL=http://10.0.0.2:8080 ROMM_TOKEN=... ROMM_DEVICE_ID=... \
  scripts/e2e-device.sh
```

Deploy grout first, with `task deploy:muos-sd1` or one of its neighbours, and
point `GROUT_DEVICE_BINARY` at where that put it.

`TestDeviceIsDrivable` reads nothing but what is already there and is worth
running first: every one of the things a device run needs fails in a way that
looks like something else. A missing `/dev/uinput` looks like grout ignoring
buttons, a missing `base64` looks like a corrupt file, an unreadable `fb0`
looks like a blank screenshot.

### What the device target stands in for

| | On this machine | On a handheld |
| --- | --- | --- |
| Buttons | `xdotool` through XTEST | a uinput keyboard, fed through a fifo |
| Screen | `import` off the X root | `/dev/fb0`, decoded here |
| Card | a temp directory | a scratch folder, `adb push` and `cat` |
| Waiting for a still screen | hash of the X root | hash of the framebuffer |

The keyboard is `test/e2e/groutkeys`, cross compiled and pushed at the start of
a run. It has to be a daemon rather than a command per keypress: a uinput
device disappears when its descriptor closes, and SDL reads the input devices
that exist when it starts. So it comes up before grout and stays up.

`ENVIRONMENT=DEV` matters here for the same reason it does in a container.
Without it the firmware's controller mapping replaces the keyboard mapping and
the virtual keyboard is ignored.

### The firmware is in the way

A handheld's own menu owns the screen and the buttons. `GROUT_E2E_DEVICE_PREPARE`
runs before a test and `GROUT_E2E_DEVICE_RESTORE` after it, and what goes in
them is firmware specific: killing MainUI on NextUI, stopping
`emulationstation` on Knulli. There is no sensible default, so there is not
one.

### One firmware at a time

A handheld is one firmware, and running the twelve case matrix at it would be
eleven runs of the wrong device pretending to be this one. Set
`GROUT_E2E_DEVICE_CFW` and the other eleven skip.

### What ADB actually gives you

These handhelds run a cut down adbd. `adb exec-out` is not implemented, and
`adb shell` both throws the exit status away and rewrites every newline. So
every command is wrapped to carry its own status and its output comes back
base64 encoded. This was found by pointing the runner at hardware, not by
reading a manual.

The framebuffer is usually double buffered: a device reporting `720,960` in
`virtual_size` with `U:720x480p-59` in `modes` has a 720x480 screen in a buffer
twice that tall. Keeping all of it would put two copies of the screen in every
screenshot.

### What is verified and what is assumed

Verified against hardware: the transport, exit statuses, exact binary reads,
the framebuffer shape, and a screenshot that comes out the right way round and
the right colours.

Verified in a privileged container: `groutkeys` creates a keyboard the kernel
publishes at `/dev/input/event0` with the fifteen keys the toolkit maps, and a
name written to its fifo comes back out as a press and a release.

Not verified: that SDL on a given firmware picks that keyboard up, that grout
starts and stops cleanly under this, and the prepare and restore commands.
Those need a device with grout deployed to it and its frontend stopped.

## What it does not cover

Whether the platform tables are right. This tests what grout does given a
belief about a firmware; it cannot tell you the belief is correct. See
`docs/platforms/unverified.md`, which needs someone holding the device.
