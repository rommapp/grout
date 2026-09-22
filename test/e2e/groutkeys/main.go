//go:build linux

// groutkeys gives a handheld a keyboard that a test can type on.
//
// It exists because a handheld has no keyboard, and grout's toolkit maps one
// when ENVIRONMENT=DEV keeps the firmware's controller mapping from loading.
// A virtual keyboard on /dev/uinput is therefore the whole of the input side
// of a device run.
//
// It is a daemon rather than a command per keypress because a uinput device
// disappears the moment its file descriptor closes, and SDL reads the input
// devices that exist when it starts. So this has to be up before grout is, and
// stay up for as long as grout does.
//
// It reads key names on stdin, one per line, and presses each:
//
//	echo KEY_RIGHT | groutkeys      # useless on its own, the device goes away
//	groutkeys < /tmp/grout-e2e.keys # what a test does, through a fifo
//
// It prints "ready" once the keyboard exists, so a caller knows when it is
// safe to start grout.
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	evdev "github.com/holoplot/go-evdev"
)

// keys are the ones the toolkit's keyboard mapping uses. A uinput device
// declares its capabilities when it is created and cannot grow new ones, so
// this list is the whole vocabulary.
var keys = []evdev.EvCode{
	evdev.KEY_UP, evdev.KEY_DOWN, evdev.KEY_LEFT, evdev.KEY_RIGHT,
	evdev.KEY_A, evdev.KEY_B, evdev.KEY_X, evdev.KEY_Y,
	evdev.KEY_L, evdev.KEY_R, evdev.KEY_T, evdev.KEY_H,
	evdev.KEY_SEMICOLON, evdev.KEY_ENTER, evdev.KEY_SPACE,
}

func main() {
	keyboard, err := evdev.CreateDevice("grout-e2e-keyboard", evdev.InputID{
		BusType: 0x03, // BUS_USB, so it is taken for a real keyboard
		Vendor:  0x6772,
		Product: 0x6b62,
		Version: 1,
	}, map[evdev.EvType][]evdev.EvCode{evdev.EV_KEY: keys})
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not create the keyboard: %v\n", err)
		os.Exit(1)
	}
	defer keyboard.Close()

	// Give the kernel a moment to publish the device before anything is told
	// it is there, so SDL does not enumerate a half-made one.
	time.Sleep(200 * time.Millisecond)
	fmt.Println("ready")
	os.Stdout.Sync()

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		name := strings.TrimSpace(scanner.Text())
		if name == "" {
			continue
		}

		code, ok := evdev.KEYFromString[name]
		if !ok {
			fmt.Fprintf(os.Stderr, "no such key: %s\n", name)
			continue
		}

		if err := press(keyboard, code); err != nil {
			fmt.Fprintf(os.Stderr, "pressing %s: %v\n", name, err)
			continue
		}
		fmt.Println("pressed " + name)
		os.Stdout.Sync()
	}
}

// press taps a key: down, up, and a sync after each so the kernel delivers
// them as two separate events rather than one.
func press(keyboard *evdev.InputDevice, code evdev.EvCode) error {
	if err := emit(keyboard, code, 1); err != nil {
		return err
	}
	// Long enough to look like a person, short enough not to repeat.
	time.Sleep(40 * time.Millisecond)
	return emit(keyboard, code, 0)
}

func emit(keyboard *evdev.InputDevice, code evdev.EvCode, value int32) error {
	if err := keyboard.WriteOne(&evdev.InputEvent{
		Time:  now(),
		Type:  evdev.EV_KEY,
		Code:  code,
		Value: value,
	}); err != nil {
		return err
	}
	return keyboard.WriteOne(&evdev.InputEvent{
		Time: now(),
		Type: evdev.EV_SYN,
		Code: evdev.SYN_REPORT,
	})
}

// now is built through NsecToTimeval because a timeval is 32 bit on the older
// handhelds and 64 bit on the newer ones.
func now() syscall.Timeval {
	return syscall.NsecToTimeval(time.Now().UnixNano())
}
