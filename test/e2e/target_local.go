//go:build e2e

package e2e

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// displays hands out a display number per run.
//
// Sharing one would make each test depend on the previous one's X server
// having fully gone, lock file and all, which it has not always done by the
// time the next starts.
var displays atomic.Int32

// local runs grout on this machine, on a virtual display.
type local struct {
	t       *testing.T
	display string
	card    string
	// output is everything grout printed, which is the only clue when it dies
	// before it opens its own log.
	output syncBuffer
}

// syncBuffer collects a subprocess's output safely while a test reads it.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (w *syncBuffer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.Write(p)
}

func (w *syncBuffer) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.String()
}

// newLocal brings up a virtual display with a window manager on it.
//
// SDL only sees keyboard input when its window holds focus, and nothing hands
// out focus without a window manager.
func newLocal(t *testing.T, width, height int) *local {
	t.Helper()

	l := &local{
		t:       t,
		display: fmt.Sprintf(":%d", 90+displays.Add(1)),
		card:    t.TempDir(),
	}

	l.background("Xvfb", l.display, "-screen", "0", fmt.Sprintf("%dx%dx24", width, height))
	l.awaitDisplay()
	l.background("openbox")

	return l
}

func (l *local) label() string    { return "this machine" }
func (l *local) cardRoot() string { return l.card }

func (l *local) readFile(path string) ([]byte, error) { return os.ReadFile(path) }

func (l *local) writeFile(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o644)
}

func (l *local) makeDir(path string) error { return os.MkdirAll(path, 0o755) }

func (l *local) listFiles(root string) []string {
	var found []string
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		relative, _ := filepath.Rel(root, path)
		found = append(found, relative)
		return nil
	})

	sort.Strings(found)
	return found
}

func (l *local) launch(t *testing.T, env map[string]string) {
	t.Helper()

	grout := exec.Command(os.Getenv("GROUT_BINARY"))
	grout.Dir = l.card
	grout.Env = append(os.Environ(), "DISPLAY="+l.display)
	for name, value := range env {
		grout.Env = append(grout.Env, name+"="+value)
	}
	// Kept so a failure can show it. Grout writes its own log to the card, but
	// anything that stops it before that starts only appears here.
	grout.Stdout = &l.output
	grout.Stderr = &l.output

	if err := grout.Start(); err != nil {
		t.Fatalf("starting grout: %v", err)
	}
	t.Cleanup(func() {
		_ = grout.Process.Kill()
		_, _ = grout.Process.Wait()
	})

	l.focusWindow()
}

// press sends keys through XTEST rather than posting them at the window,
// because SDL ignores events it can tell were synthesised.
func (l *local) press(keys ...string) error {
	for _, key := range keys {
		if _, err := l.x("xdotool", "key", "--clearmodifiers", key); err != nil {
			return err
		}
	}
	return nil
}

func (l *local) capture(localPath string) error {
	_, err := l.x("import", "-window", "root", localPath)
	return err
}

func (l *local) awaitStill() {
	deadline := time.Now().Add(waitTimeout)
	previous := ""

	for time.Now().Before(deadline) {
		current, err := l.x("sh", "-c", "import -window root png:- | md5sum")
		current = strings.TrimSpace(current)
		if err == nil && current != "" && current == previous {
			return
		}
		previous = current
		time.Sleep(settle)
	}

	l.t.Logf("the screen was still changing after %s", waitTimeout)
}

// background runs a helper for the lifetime of the test.
func (l *local) background(name string, args ...string) {
	l.t.Helper()

	cmd := exec.Command(name, args...)
	cmd.Env = append(os.Environ(), "DISPLAY="+l.display)
	if err := cmd.Start(); err != nil {
		l.t.Fatalf("starting %s: %v", name, err)
	}
	l.t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})
}

// awaitDisplay blocks until the virtual display accepts connections, rather
// than guessing with a sleep.
func (l *local) awaitDisplay() {
	l.t.Helper()

	deadline := time.Now().Add(waitTimeout)
	for time.Now().Before(deadline) {
		if exec.Command("xdpyinfo", "-display", l.display).Run() == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	l.t.Fatal("the virtual display never came up")
}

// focusWindow gives grout's window the input focus. Keys go nowhere without
// it, silently.
func (l *local) focusWindow() {
	l.t.Helper()

	deadline := time.Now().Add(waitTimeout)
	for time.Now().Before(deadline) {
		out, err := l.x("xdotool", "search", "--name", "^Grout$")
		if id := strings.TrimSpace(firstLine(out)); err == nil && id != "" {
			_, _ = l.x("xdotool", "windowactivate", id)
			time.Sleep(settle)
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	l.t.Fatal("grout never opened a window")
}

func (l *local) x(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Env = append(os.Environ(), "DISPLAY="+l.display)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
