//go:build e2e

package e2e

import (
	"image/color"
	"strings"
	"testing"
)

// The keys a test presses are named the way the toolkit maps them, and the
// device target has to be able to say all of them to a kernel. A name the
// device cannot translate would work on this machine and do nothing on a
// handheld.
func TestEveryToolkitKeyHasAnEvdevName(t *testing.T) {
	// The toolkit's default keyboard mapping, which is what is active when
	// ENVIRONMENT=DEV keeps the firmware's own mapping from loading.
	toolkit := []string{
		"Up", "Down", "Left", "Right",
		"a", "b", "x", "y",
		"l", "r", "semicolon", "t",
		"Return", "space", "h",
	}

	for _, key := range toolkit {
		if _, ok := evdevKeys[key]; !ok {
			t.Errorf("a test can press %q on this machine but not on a device", key)
		}
	}

	if len(evdevKeys) != len(toolkit) {
		t.Errorf("the device knows %d keys, the toolkit maps %d", len(evdevKeys), len(toolkit))
	}
}

// Every name has to be one the kernel actually defines, or the keyboard helper
// rejects it at run time on a device nobody is watching.
func TestEvdevNamesAreSpelledTheWayTheKernelSpellsThem(t *testing.T) {
	for key, name := range evdevKeys {
		if !strings.HasPrefix(name, "KEY_") {
			t.Errorf("%s maps to %q, which is not a kernel key name", key, name)
		}
	}
}

// A handheld's framebuffer is sixteen bit far more often than not, and a
// channel widened by padding with zeroes turns white into grey.
func TestDecodesSixteenBitFramebuffer(t *testing.T) {
	screen := framebuffer{width: 2, height: 1, visibleHeight: 1, bitsPerPixel: 16}

	// White, then pure red, little endian.
	picture, err := screen.decode([]byte{0xff, 0xff, 0x00, 0xf8})
	if err != nil {
		t.Fatalf("decoding: %v", err)
	}

	if got := picture.At(0, 0); got != (color.RGBA{255, 255, 255, 255}) {
		t.Errorf("white came out as %v", got)
	}
	if got := picture.At(1, 0); got != (color.RGBA{255, 0, 0, 255}) {
		t.Errorf("red came out as %v", got)
	}
}

// The thirty two bit devices put blue first, so a decode that assumes the
// usual order swaps red and blue in every screenshot.
func TestDecodesThirtyTwoBitFramebuffer(t *testing.T) {
	screen := framebuffer{width: 1, height: 1, visibleHeight: 1, bitsPerPixel: 32}

	picture, err := screen.decode([]byte{0x00, 0x00, 0xff, 0x00})
	if err != nil {
		t.Fatalf("decoding: %v", err)
	}

	if got := picture.At(0, 0); got != (color.RGBA{255, 0, 0, 255}) {
		t.Errorf("a blue-first red came out as %v", got)
	}
}

// A device that double buffers reports twice the height it shows, and a
// screenshot of both halves is a screenshot of nothing anyone can read.
func TestDecodesOnlyTheVisiblePartOfADoubleBuffer(t *testing.T) {
	screen := framebuffer{width: 1, height: 2, visibleHeight: 1, bitsPerPixel: 32}

	picture, err := screen.decode([]byte{0, 0, 0xff, 0, 0, 0, 0xff, 0})
	if err != nil {
		t.Fatalf("decoding: %v", err)
	}

	if got := picture.Bounds().Dy(); got != 1 {
		t.Errorf("the picture is %d rows tall, want just the visible one", got)
	}
}

// A short read means the framebuffer was still being written, and a picture
// built from it would be half garbage rather than an obvious failure.
func TestRefusesAShortFramebuffer(t *testing.T) {
	screen := framebuffer{width: 4, height: 4, visibleHeight: 4, bitsPerPixel: 32}

	if _, err := screen.decode([]byte{1, 2, 3, 4}); err == nil {
		t.Error("a four byte read passed for a 4x4 screen")
	}
}

func TestReadsTheDeviceSpec(t *testing.T) {
	for _, tc := range []struct {
		spec    string
		want    string
		refused bool
	}{
		{spec: "adb", want: "the attached adb device"},
		{spec: "adb:ABC123", want: "adb device ABC123"},
		{spec: "ssh:root@10.0.0.5", want: "ssh to root@10.0.0.5"},
		{spec: "ssh", refused: true},
		{spec: "telnet:box", refused: true},
	} {
		t.Run(tc.spec, func(t *testing.T) {
			over, err := openTransport(tc.spec)
			if tc.refused {
				if err == nil {
					t.Fatalf("%q was accepted", tc.spec)
				}
				return
			}
			if err != nil {
				t.Fatalf("%q was refused: %v", tc.spec, err)
			}
			if got := over.describe(); got != tc.want {
				t.Errorf("describe() = %q, want %q", got, tc.want)
			}
		})
	}
}

// Card paths and rom names both have spaces in them, and a device's shell
// would otherwise read one argument as two.
func TestQuotesForADeviceShell(t *testing.T) {
	if got := shellQuote("/mnt/SDCARD/Roms/Test Game (USA).sfc"); got != `'/mnt/SDCARD/Roms/Test Game (USA).sfc'` {
		t.Errorf("quoting a space gave %s", got)
	}
	if got := shellQuote("it's"); got != `'it'\''s'` {
		t.Errorf("quoting a quote gave %s", got)
	}
}

// The shape sysfs describes is what every screenshot is cut to, and the one
// device this was checked against reports a buffer twice the height of its
// screen.
func TestReadsTheFramebufferShape(t *testing.T) {
	for _, tc := range []struct {
		name     string
		sysfs    string
		want     framebuffer
		unusable bool
	}{
		{
			// Verbatim from a handheld over adb.
			name:  "a double buffered handheld",
			sysfs: "720,960\n32\nU:720x480p-59\n",
			want:  framebuffer{width: 720, height: 960, visibleHeight: 480, bitsPerPixel: 32},
		},
		{
			name:  "no mode line, so the whole buffer is the screen",
			sysfs: "640,480\n16\n",
			want:  framebuffer{width: 640, height: 480, visibleHeight: 480, bitsPerPixel: 16},
		},
		{
			// A mode taller than the buffer is the sysfs lying, and cutting to
			// it would read past the end of the frame.
			name:  "a mode line bigger than the buffer is ignored",
			sysfs: "640,480\n16\nU:640x960p-60\n",
			want:  framebuffer{width: 640, height: 480, visibleHeight: 480, bitsPerPixel: 16},
		},
		{name: "nothing at all", sysfs: "", unusable: true},
		{name: "a size that is not a size", sysfs: "unknown\n32\n", unusable: true},
		{name: "no depth", sysfs: "640,480\n0\n", unusable: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := readFramebufferShape(tc.sysfs)
			if tc.unusable {
				if err == nil {
					t.Fatalf("%q was read as %+v", tc.sysfs, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("reading %q: %v", tc.sysfs, err)
			}
			if got != tc.want {
				t.Errorf("read %q as %+v, want %+v", tc.sysfs, got, tc.want)
			}
		})
	}
}

// A device's shell swallows the exit status and rewrites newlines, so a
// command is wrapped to carry its own answer back. These are the shapes that
// wrapping produces, and a device is not needed to check them.
func TestUnwrapsADeviceAnswer(t *testing.T) {
	// Exactly what a device sends: markers in it too, and carriage returns
	// everywhere because the shell put them there.
	answer := "__rc=0\r\naGVsbG8=\r\n__stderr\r\n\r\n"
	out, err := unwrap([]byte(answer), "echo hello")
	if err != nil {
		t.Fatalf("unwrapping: %v", err)
	}
	if string(out) != "hello" {
		t.Errorf("read %q, want %q", out, "hello")
	}
}

func TestUnwrapReportsAFailureWithWhatTheDeviceSaid(t *testing.T) {
	// status 1, nothing on stdout, a complaint on stderr.
	answer := "__rc=1\r\n\r\n__stderr\r\nY2F0OiBubyBzdWNoIGZpbGU=\r\n"
	_, err := unwrap([]byte(answer), "cat /nope")
	if err == nil {
		t.Fatal("a failure looked like a success")
	}
	if !strings.Contains(err.Error(), "cat: no such file") {
		t.Errorf("the error does not say what the device said: %v", err)
	}
}

// A device that answers with nothing at all, because it was unplugged mid
// command, must not read as an empty success.
func TestUnwrapRefusesAnAnswerThatIsNotOne(t *testing.T) {
	if _, err := unwrap([]byte("adb: device offline\r\n"), "uname -m"); err == nil {
		t.Error("an unplugged device looked like an empty success")
	}
}
