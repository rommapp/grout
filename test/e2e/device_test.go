//go:build e2e

package e2e

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A device run needs a handful of things from the firmware, and every one of
// them fails in a way that looks like something else: a missing /dev/uinput
// looks like grout ignoring buttons, a missing base64 looks like a corrupt
// file, an unreadable framebuffer looks like a blank screenshot.
//
// This asks for all of them up front and reads nothing but what is already
// there, so it is safe to point at a device that is in the middle of a game.
//
//	GROUT_E2E_DEVICE=adb:<serial> go test -tags e2e -run TestDeviceIsDrivable ./test/e2e/
func TestDeviceIsDrivable(t *testing.T) {
	spec := os.Getenv("GROUT_E2E_DEVICE")
	if spec == "" {
		t.Skip("set GROUT_E2E_DEVICE to adb, adb:<serial> or ssh:<user@host>")
	}

	over, err := openTransport(spec)
	if err != nil {
		t.Fatalf("reading GROUT_E2E_DEVICE: %v", err)
	}
	if err := over.available(); err != nil {
		t.Skipf("no device to check: %v", err)
	}
	t.Logf("checking %s", over.describe())

	t.Run("the shell answers exactly", func(t *testing.T) {
		// A device's shell rewrites newlines, so everything read off it comes
		// back base64 encoded. This is that surviving a round trip.
		out, err := over.run(30*time.Second, `printf 'a\r\n\000b'`)
		if err != nil {
			t.Fatalf("running a command: %v", err)
		}
		if string(out) != "a\r\n\x00b" {
			t.Errorf("the device garbled its answer: %q", out)
		}
	})

	t.Run("a failure is reported as one", func(t *testing.T) {
		// adb throws the exit status away, so a command that carries its own
		// is the only way a failing step is ever noticed.
		if _, err := over.run(30*time.Second, "exit 7"); err == nil {
			t.Error("a command that failed looked like it succeeded")
		} else if !strings.Contains(err.Error(), "7") {
			t.Errorf("the failure does not say the status: %v", err)
		}
	})

	t.Run("the tools the runner uses are there", func(t *testing.T) {
		// A cut down busybox is the normal case on these devices.
		for _, tool := range []string{"base64", "md5sum", "mkfifo", "find", "dd", "chmod", "kill"} {
			if _, err := over.run(30*time.Second, "command -v "+tool); err != nil {
				t.Errorf("the device has no %s, which the runner needs", tool)
			}
		}
	})

	t.Run("there is a uinput to make a keyboard on", func(t *testing.T) {
		// Without it there is no way to press a button, and a run would get as
		// far as the first screen and stop.
		if _, err := over.run(30*time.Second, "test -c /dev/uinput"); err != nil {
			t.Errorf("no /dev/uinput, so nothing can be typed on this device: %v", err)
		}
	})

	t.Run("the screen can be read", func(t *testing.T) {
		described, err := over.run(30*time.Second,
			"cat /sys/class/graphics/fb0/virtual_size /sys/class/graphics/fb0/bits_per_pixel; cat /sys/class/graphics/fb0/modes 2>/dev/null")
		if err != nil {
			t.Skipf("this device has no fb0, so a run would have no screenshots: %v", err)
		}

		screen, err := readFramebufferShape(string(described))
		if err != nil {
			t.Fatalf("the device described its screen as %q: %v", described, err)
		}
		t.Logf("%dx%d at %d bits, in a buffer %d tall", screen.width, screen.visibleHeight, screen.bitsPerPixel, screen.height)

		raw, err := over.run(60*time.Second, "dd if=/dev/fb0 2>/dev/null")
		if err != nil {
			t.Fatalf("reading the framebuffer: %v", err)
		}

		picture, err := screen.decode(raw)
		if err != nil {
			t.Fatalf("decoding the framebuffer: %v", err)
		}

		// Written out so a person can see whether the colours came back in the
		// right order, which nothing here can tell on its own.
		path := filepath.Join(screenshotDir(t), "device-screen.png")
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()

		if err := png.Encode(file, picture); err != nil {
			t.Fatalf("writing the screenshot: %v", err)
		}
		t.Logf("what is on the device's screen right now: %s", path)
	})
}
