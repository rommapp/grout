//go:build e2e

package e2e

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// device runs grout on a handheld on the end of a cable.
//
// The three things a handheld does not have are a keyboard, a way to take a
// screenshot, and a filesystem this machine can reach. A virtual keyboard on
// /dev/uinput covers the first, the framebuffer covers the second, and the
// transport covers the third.
type device struct {
	t        *testing.T
	over     transport
	card     string
	keysFifo string
	// screen is what the framebuffer said it was, read once because it does
	// not change while grout is up.
	screen framebuffer
}

// how long a command on a device is given. A handheld's CPU is slow and its
// storage is an SD card.
const deviceTimeout = 60 * time.Second

// newDevice opens the cable and gets the device ready to be driven.
//
// A run is skipped rather than failed when nothing is attached, so `go test`
// with a device configured but unplugged behaves like the container suite does
// without a RomM.
func newDevice(t *testing.T, spec string) *device {
	t.Helper()

	over, err := openTransport(spec)
	if err != nil {
		t.Fatalf("reading GROUT_E2E_DEVICE: %v", err)
	}
	if err := over.available(); err != nil {
		t.Skipf("no device to test against: %v", err)
	}

	root := envOr("GROUT_E2E_DEVICE_CARD", "/tmp/grout-e2e")
	d := &device{
		t:    t,
		over: over,
		// Per test, so one test cannot see what another left behind, the way
		// t.TempDir does on this machine.
		card:     path.Join(root, strings.ReplaceAll(t.Name(), "/", "_")),
		keysFifo: path.Join(root, "keys.fifo"),
	}

	d.shell("rm -rf " + shellQuote(d.card))
	d.shell("mkdir -p " + shellQuote(d.card))
	t.Cleanup(func() { d.tidy("rm -rf " + shellQuote(d.card)) })

	// The firmware's own menu owns the screen and the buttons, so it has to be
	// out of the way before grout can have either.
	if prepare := os.Getenv("GROUT_E2E_DEVICE_PREPARE"); prepare != "" {
		d.shell(prepare)
		if restore := os.Getenv("GROUT_E2E_DEVICE_RESTORE"); restore != "" {
			t.Cleanup(func() { d.tidy(restore) })
		}
	}

	return d
}

func (d *device) label() string    { return d.over.describe() }
func (d *device) cardRoot() string { return d.card }

// readFile reads over the transport rather than pulling to a file, because
// the log is read again every tenth of a second while a step waits.
//
// Both transports hand back stdout untouched, which a plain `adb shell` would
// not: it rewrites newlines, and art and roms are read through here too.
func (d *device) readFile(p string) ([]byte, error) {
	return d.over.run(deviceTimeout, "cat "+shellQuote(p))
}

// writeFile stages the content on this machine and pushes it, which is the
// only way to put bytes on the device without a shell mangling them.
func (d *device) makeDir(p string) error {
	_, err := d.over.run(deviceTimeout, "mkdir -p "+shellQuote(p))
	return err
}

func (d *device) writeFile(p string, content []byte) error {
	if err := d.makeDir(path.Dir(p)); err != nil {
		return err
	}

	staged := filepath.Join(d.t.TempDir(), "staged")
	if err := os.WriteFile(staged, content, 0o644); err != nil {
		return err
	}
	return d.over.push(staged, p)
}

func (d *device) listFiles(root string) []string {
	out, err := d.over.run(deviceTimeout, "find "+shellQuote(root)+" -type f")
	if err != nil {
		return nil
	}

	var found []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		found = append(found, strings.TrimPrefix(strings.TrimPrefix(line, root), "/"))
	}

	sort.Strings(found)
	return found
}

// launch starts the keyboard, then grout, in that order: SDL reads the input
// devices that exist when it starts.
func (d *device) launch(t *testing.T, env map[string]string) {
	t.Helper()

	d.startKeyboard()

	binary := os.Getenv("GROUT_DEVICE_BINARY")
	if binary == "" {
		t.Fatal("set GROUT_DEVICE_BINARY to where grout is on the device, for example /mnt/mmc/MUOS/application/Grout/grout")
	}

	// The firmware's controller mapping would otherwise be loaded and the
	// keyboard ignored, which is the one thing that has to stay true.
	assignments := []string{"ENVIRONMENT=DEV"}
	for name, value := range env {
		assignments = append(assignments, name+"="+shellQuote(value))
	}
	sort.Strings(assignments)

	stdout := path.Join(d.card, "stdout.log")
	pidFile := path.Join(d.card, "grout.pid")
	// Started from the folder it was deployed into rather than from the card,
	// because that is where the firmware would launch it from and grout finds
	// what ships alongside it that way. BASE_PATH is what points it at the
	// card.
	//
	// Backgrounded with its output redirected, so the transport's own command
	// returns instead of waiting on grout's stdout for the whole run.
	d.shell(fmt.Sprintf("cd %s && %s %s > %s 2>&1 & echo $! > %s",
		shellQuote(path.Dir(binary)), strings.Join(assignments, " "), shellQuote(binary),
		shellQuote(stdout), shellQuote(pidFile)))

	t.Cleanup(func() {
		d.tidy(fmt.Sprintf("kill $(cat %s) 2>/dev/null; sleep 1; kill -9 $(cat %s) 2>/dev/null; true",
			shellQuote(pidFile), shellQuote(pidFile)))
		if out, err := d.over.run(deviceTimeout, "cat "+shellQuote(stdout)); err == nil && len(out) > 0 {
			t.Logf("grout printed:\n%s", out)
		}
	})

	d.awaitScreen()
}

// startKeyboard puts the virtual keyboard on the device and leaves it running.
func (d *device) startKeyboard() {
	d.t.Helper()

	binary := d.buildKeyboard()
	remote := path.Join(path.Dir(d.keysFifo), "groutkeys")

	if err := d.over.push(binary, remote); err != nil {
		d.t.Fatalf("putting the keyboard on the device: %v", err)
	}
	d.shell("chmod +x " + shellQuote(remote))

	// A fifo rather than a pipe through the transport, so a keypress is one
	// short command and does not need the connection held open.
	d.shell(fmt.Sprintf("rm -f %s && mkfifo %s", shellQuote(d.keysFifo), shellQuote(d.keysFifo)))

	// Held open for writing by a sleeper as well, or the daemon sees end of
	// file the moment the first keypress finishes writing and exits.
	log := path.Join(d.card, "keys.log")
	cmd, err := d.over.start(fmt.Sprintf("sleep 86400 > %s & %s < %s > %s 2>&1",
		shellQuote(d.keysFifo), shellQuote(remote), shellQuote(d.keysFifo), shellQuote(log)))
	if err != nil {
		d.t.Fatalf("starting the keyboard: %v", err)
	}
	d.t.Cleanup(func() {
		// pidof as well as pkill, because a minimal busybox has only one of
		// them and which one differs by firmware.
		d.tidy("pkill -f groutkeys 2>/dev/null; kill $(pidof groutkeys) 2>/dev/null; rm -f " + shellQuote(d.keysFifo) + "; true")
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})

	d.awaitKeyboard(log)
}

// buildKeyboard cross-compiles the helper for the device's architecture.
//
// It is pure Go, so this needs no toolchain beyond the one already running the
// tests.
func (d *device) buildKeyboard() string {
	d.t.Helper()

	arch := envOr("GROUT_E2E_DEVICE_ARCH", d.detectArch())
	out := filepath.Join(d.t.TempDir(), "groutkeys")

	build := exec.Command("go", "build", "-o", out, "./test/e2e/groutkeys")
	build.Dir = repoRoot(d.t)
	build.Env = append(os.Environ(), "GOOS=linux", "GOARCH="+arch, "CGO_ENABLED=0")
	if message, err := build.CombinedOutput(); err != nil {
		d.t.Fatalf("building the keyboard for linux/%s: %v\n%s", arch, err, message)
	}
	return out
}

// detectArch asks the device what it is, because the handhelds are a mix of
// 32 and 64 bit ARM.
func (d *device) detectArch() string {
	out, err := d.over.run(deviceTimeout, "uname -m")
	if err != nil {
		d.t.Fatalf("asking the device its architecture: %v", err)
	}

	switch machine := strings.TrimSpace(string(out)); machine {
	case "aarch64", "arm64":
		return "arm64"
	case "x86_64":
		return "amd64"
	default:
		if strings.HasPrefix(machine, "arm") {
			return "arm"
		}
		d.t.Fatalf("unknown device architecture %q, set GROUT_E2E_DEVICE_ARCH", machine)
		return ""
	}
}

// awaitKeyboard waits for the helper to say the keyboard exists, so grout is
// not started before there is anything to type on.
func (d *device) awaitKeyboard(log string) {
	d.t.Helper()

	deadline := time.Now().Add(waitTimeout)
	for time.Now().Before(deadline) {
		out, err := d.over.run(deviceTimeout, "cat "+shellQuote(log)+" 2>/dev/null")
		if err == nil && strings.Contains(string(out), "ready") {
			return
		}
		if err == nil && strings.Contains(string(out), "could not create the keyboard") {
			d.t.Fatalf("the device would not give up a keyboard, which usually means no /dev/uinput:\n%s", out)
		}
		time.Sleep(200 * time.Millisecond)
	}
	d.t.Fatalf("the keyboard never came up within %s", waitTimeout)
}

func (d *device) press(keys ...string) error {
	for _, key := range keys {
		name, ok := evdevKeys[key]
		if !ok {
			return fmt.Errorf("no evdev key for %q", key)
		}
		if _, err := d.over.run(deviceTimeout,
			fmt.Sprintf("printf '%%s\\n' %s > %s", shellQuote(name), shellQuote(d.keysFifo))); err != nil {
			return err
		}
	}
	return nil
}

// evdevKeys translates the names the tests press into the names the kernel
// knows, which is the same translation SDL does in reverse.
var evdevKeys = map[string]string{
	"Up": "KEY_UP", "Down": "KEY_DOWN", "Left": "KEY_LEFT", "Right": "KEY_RIGHT",
	"a": "KEY_A", "b": "KEY_B", "x": "KEY_X", "y": "KEY_Y",
	"l": "KEY_L", "r": "KEY_R", "t": "KEY_T", "h": "KEY_H",
	"semicolon": "KEY_SEMICOLON", "Return": "KEY_ENTER", "space": "KEY_SPACE",
}

func (d *device) capture(localPath string) error {
	raw, err := d.readFramebuffer()
	if err != nil {
		return err
	}

	picture, err := d.screen.decode(raw)
	if err != nil {
		return err
	}

	file, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer file.Close()

	return png.Encode(file, picture)
}

// awaitStill hashes the framebuffer on the device rather than pulling it,
// because a frame is megabytes and this happens before every keypress.
func (d *device) awaitStill() {
	deadline := time.Now().Add(waitTimeout)
	previous := ""

	for time.Now().Before(deadline) {
		out, err := d.over.run(deviceTimeout, "dd if=/dev/fb0 2>/dev/null | md5sum")
		current := strings.TrimSpace(string(out))
		if err != nil {
			// Without a hash there is nothing to compare, so fall back to
			// waiting out the longest a screen takes to draw.
			time.Sleep(settle)
			return
		}
		if current != "" && current == previous {
			return
		}
		previous = current
		time.Sleep(settle)
	}

	d.t.Logf("the screen was still changing after %s", waitTimeout)
}

// framebuffer is what the device said its screen is.
type framebuffer struct {
	width, height int
	// visibleHeight is the part of the buffer actually on screen, which is
	// half of it on a device that double buffers.
	visibleHeight int
	bitsPerPixel  int
}

// awaitScreen reads the framebuffer's shape, which does not change while
// grout is up.
func (d *device) awaitScreen() {
	d.t.Helper()

	out, err := d.over.run(deviceTimeout,
		"cat /sys/class/graphics/fb0/virtual_size /sys/class/graphics/fb0/bits_per_pixel; cat /sys/class/graphics/fb0/modes 2>/dev/null")
	if err != nil {
		d.t.Logf("the device would not describe its framebuffer, so screenshots will be skipped: %v", err)
		return
	}

	screen, err := readFramebufferShape(string(out))
	if err != nil {
		d.t.Logf("the device described its framebuffer as %q, which says nothing useful: %v", out, err)
		return
	}

	d.screen = screen
	d.t.Logf("the screen is %dx%d at %d bits, in a buffer %d tall",
		screen.width, screen.visibleHeight, screen.bitsPerPixel, screen.height)
}

// readFramebufferShape reads what sysfs says about the screen.
//
// It is given virtual_size, then bits_per_pixel, then the mode line if there
// is one:
//
//	720,960
//	32
//	U:720x480p-59
//
// The buffer there is twice the height of the screen, because the device draws
// into one half while the other is on show. Taking the whole of it would put
// two copies of the screen in every screenshot, so the mode line is what says
// how much of it to keep.
func readFramebufferShape(described string) (framebuffer, error) {
	lines := strings.Split(strings.TrimSpace(described), "\n")
	if len(lines) < 2 {
		return framebuffer{}, fmt.Errorf("want at least a size and a depth")
	}

	width, height, ok := strings.Cut(strings.TrimSpace(lines[0]), ",")
	if !ok {
		return framebuffer{}, fmt.Errorf("%q is not a size", lines[0])
	}

	screen := framebuffer{
		width:         atoiOr(width, 0),
		height:        atoiOr(height, 0),
		visibleHeight: atoiOr(height, 0),
		bitsPerPixel:  atoiOr(strings.TrimSpace(lines[1]), 0),
	}
	if screen.width == 0 || screen.height == 0 || screen.bitsPerPixel == 0 {
		return framebuffer{}, fmt.Errorf("a %sx%s screen at %s bits is not a screen", width, height, lines[1])
	}

	// A mode line reads like "U:720x480p-59". Its height is what is on show,
	// which is the part of the buffer worth keeping.
	if len(lines) > 2 {
		if _, size, found := strings.Cut(lines[2], ":"); found {
			if _, tail, ok := strings.Cut(size, "x"); ok {
				if visible := leadingNumber(tail); visible > 0 && visible <= screen.height {
					screen.visibleHeight = visible
				}
			}
		}
	}

	return screen, nil
}

// leadingNumber reads the digits a string starts with, for a mode line whose
// height runs straight into its refresh rate.
func leadingNumber(s string) int {
	end := 0
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	return atoiOr(s[:end], 0)
}

func (d *device) readFramebuffer() ([]byte, error) {
	if d.screen.width == 0 || d.screen.bitsPerPixel == 0 {
		return nil, fmt.Errorf("the device never said what its framebuffer looks like")
	}
	return d.over.run(deviceTimeout, "dd if=/dev/fb0 2>/dev/null")
}

// decode turns a raw framebuffer into a picture.
//
// The handhelds are either 16 bit RGB565 or 32 bit with the blue byte first,
// and there is no third case worth carrying.
func (f framebuffer) decode(raw []byte) (image.Image, error) {
	bytesPerPixel := f.bitsPerPixel / 8
	if bytesPerPixel != 2 && bytesPerPixel != 4 {
		return nil, fmt.Errorf("a %d bit framebuffer is not something this can read", f.bitsPerPixel)
	}

	stride := f.width * bytesPerPixel
	height := f.visibleHeight
	if want := stride * height; len(raw) < want {
		return nil, fmt.Errorf("the framebuffer gave back %d bytes, short of the %d a %dx%d screen needs",
			len(raw), want, f.width, height)
	}

	picture := image.NewRGBA(image.Rect(0, 0, f.width, height))
	for y := range height {
		row := raw[y*stride:]
		for x := range f.width {
			pixel := row[x*bytesPerPixel:]

			var c color.RGBA
			if bytesPerPixel == 2 {
				value := uint16(pixel[0]) | uint16(pixel[1])<<8
				// RGB565, widened back to eight bits a channel by repeating
				// the high bits rather than padding with zeroes, so white
				// stays white.
				r, g, b := (value>>11)&0x1f, (value>>5)&0x3f, value&0x1f
				c = color.RGBA{uint8(r<<3 | r>>2), uint8(g<<2 | g>>4), uint8(b<<3 | b>>2), 255}
			} else {
				c = color.RGBA{pixel[2], pixel[1], pixel[0], 255}
			}
			picture.SetRGBA(x, y, c)
		}
	}
	return picture, nil
}

// shell runs a command on the device and fails the test if it cannot.
func (d *device) shell(command string) {
	d.t.Helper()

	if _, err := d.over.run(deviceTimeout, command); err != nil {
		d.t.Fatalf("running %q on the device: %v", command, err)
	}
}

// tidy runs a command that is putting things back, where a failure is worth
// mentioning but is not the test's fault.
func (d *device) tidy(command string) {
	if _, err := d.over.run(deviceTimeout, command); err != nil {
		d.t.Logf("could not run %q on the device: %v", command, err)
	}
}

func atoiOr(s string, fallback int) int {
	if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
		return n
	}
	return fallback
}

// repoRoot is where the module is, so the keyboard can be built from a test
// running in a package below it.
func repoRoot(t *testing.T) string {
	t.Helper()

	out, err := exec.Command("go", "env", "GOMOD").Output()
	if err != nil {
		t.Fatalf("finding the module root: %v", err)
	}
	return filepath.Dir(strings.TrimSpace(string(out)))
}
