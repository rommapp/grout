//go:build e2e

// Package e2e drives grout's real UI on a virtual display.
//
// It runs the shipped binary, presses buttons, and waits for grout to say what
// it did. Assertions are on grout's own structured log rather than on pixels:
// the log says what happened, a screenshot only says what it looked like, and
// fonts and anti-aliasing make the second answer differ between machines.
//
// Screenshots are still taken, as artifacts for a person to look at when
// something fails.
package e2e

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	display = ":99"
	// waitTimeout bounds how long a step waits for grout to report something.
	// A slow CI runner emulating another architecture needs the room.
	waitTimeout = 30 * time.Second
	// settle is how long to let a screen redraw before a screenshot.
	settle = 500 * time.Millisecond
)

// session is one run of grout against a synthetic card.
type session struct {
	t        *testing.T
	cardPath string
	logPath  string
	shotDir  string
	// read is how far through the log this session has already looked, so a
	// later wait cannot match a line an earlier one already consumed.
	read int
}

// options say which device the run should look like.
type options struct {
	// CFW is the firmware to present as, which decides every path grout uses.
	CFW string
	// Width and Height are the virtual screen, ideally the real device's.
	Width, Height int
}

// start lays out a card, brings up a virtual display, and runs grout on it.
//
// Everything is torn down when the test ends, including on failure, so a
// panicking test does not leave an X server behind.
func start(t *testing.T, opts options) *session {
	t.Helper()

	if opts.Width == 0 {
		opts.Width, opts.Height = 1024, 768
	}

	card := t.TempDir()
	for _, dir := range []string{"ROMS", "BIOS", "logs"} {
		if err := os.MkdirAll(filepath.Join(card, dir), 0o755); err != nil {
			t.Fatalf("laying out the card: %v", err)
		}
	}

	s := &session{
		t:        t,
		cardPath: card,
		logPath:  filepath.Join(card, "logs", "app.log"),
		shotDir:  screenshotDir(t),
	}

	s.startBackground("Xvfb", display, "-screen", "0",
		fmt.Sprintf("%dx%dx24", opts.Width, opts.Height))
	s.waitForX()

	// SDL only sees keyboard input when its window holds focus, and nothing
	// hands out focus without a window manager.
	s.startBackground("openbox")

	grout := exec.Command(os.Getenv("GROUT_BINARY"))
	grout.Dir = card
	grout.Env = append(os.Environ(),
		"CFW="+opts.CFW,
		"BASE_PATH="+card,
		"DISPLAY="+display,
	)
	if err := grout.Start(); err != nil {
		t.Fatalf("starting grout: %v", err)
	}
	t.Cleanup(func() {
		_ = grout.Process.Kill()
		_, _ = grout.Process.Wait()
	})

	s.focusWindow()
	return s
}

// startBackground runs a helper for the lifetime of the test.
func (s *session) startBackground(name string, args ...string) {
	s.t.Helper()

	cmd := exec.Command(name, args...)
	cmd.Env = append(os.Environ(), "DISPLAY="+display)
	if err := cmd.Start(); err != nil {
		s.t.Fatalf("starting %s: %v", name, err)
	}
	s.t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})
}

// waitForX blocks until the virtual display is accepting connections, rather
// than guessing with a sleep.
func (s *session) waitForX() {
	s.t.Helper()

	deadline := time.Now().Add(waitTimeout)
	for time.Now().Before(deadline) {
		if exec.Command("xdpyinfo", "-display", display).Run() == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	s.t.Fatal("the virtual display never came up")
}

// focusWindow gives grout's window the input focus. Keys go nowhere without
// it, silently.
func (s *session) focusWindow() {
	s.t.Helper()

	deadline := time.Now().Add(waitTimeout)
	for time.Now().Before(deadline) {
		out, err := s.x("xdotool", "search", "--name", "^Grout$")
		if id := strings.TrimSpace(firstLine(out)); err == nil && id != "" {
			_, _ = s.x("xdotool", "windowactivate", id)
			time.Sleep(settle)
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	s.t.Fatal("grout never opened a window")
}

// press sends button presses, named as the toolkit maps them: arrow keys for
// the d-pad, a b x y for the face buttons, Return for start, space for select.
//
// The keys go through XTEST rather than being posted at the window, because
// SDL ignores events it can tell were synthesised.
func (s *session) press(keys ...string) {
	s.t.Helper()

	for _, key := range keys {
		if _, err := s.x("xdotool", "key", "--clearmodifiers", key); err != nil {
			s.t.Fatalf("pressing %s: %v", key, err)
		}
		time.Sleep(settle)
	}
}

// logEntry is one line of grout's structured log.
type logEntry struct {
	Level  string `json:"level"`
	Msg    string `json:"msg"`
	fields map[string]any
}

// Field reads one of the values grout logged alongside the message.
func (e logEntry) Field(name string) any { return e.fields[name] }

// awaitLog waits for grout to log a message, and returns it.
//
// Only lines this session has not already matched are considered, so waiting
// twice for the same message means it really happened twice.
func (s *session) awaitLog(msg string) logEntry {
	s.t.Helper()

	deadline := time.Now().Add(waitTimeout)
	for time.Now().Before(deadline) {
		entries := s.readLog()
		for i := s.read; i < len(entries); i++ {
			if entries[i].Msg == msg {
				s.read = i + 1
				return entries[i]
			}
		}
		time.Sleep(100 * time.Millisecond)
	}

	s.screenshot("timeout")
	s.t.Fatalf("grout never logged %q within %s\nlog so far:\n%s",
		msg, waitTimeout, s.logTail())
	return logEntry{}
}

func (s *session) readLog() []logEntry {
	file, err := os.Open(s.logPath)
	if err != nil {
		return nil
	}
	defer file.Close()

	var entries []logEntry
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		var entry logEntry
		line := scanner.Bytes()
		if json.Unmarshal(line, &entry) != nil {
			continue
		}
		_ = json.Unmarshal(line, &entry.fields)
		entries = append(entries, entry)
	}
	return entries
}

func (s *session) logTail() string {
	entries := s.readLog()
	if len(entries) > 12 {
		entries = entries[len(entries)-12:]
	}

	var b strings.Builder
	for _, entry := range entries {
		fmt.Fprintf(&b, "  %-5s %s\n", entry.Level, entry.Msg)
	}
	return b.String()
}

// screenshot saves what is on screen, for a person to look at when a test
// fails. CI keeps them as artifacts.
func (s *session) screenshot(name string) {
	s.t.Helper()

	time.Sleep(settle)
	path := filepath.Join(s.shotDir, name+".png")
	if _, err := s.x("import", "-window", "root", path); err != nil {
		s.t.Logf("could not capture %s: %v", name, err)
		return
	}
	s.t.Logf("screenshot: %s", path)
}

// cardHas reports whether grout wrote a file to the synthetic card.
func (s *session) cardHas(relative string) bool {
	_, err := os.Stat(filepath.Join(s.cardPath, relative))
	return err == nil
}

func (s *session) x(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Env = append(os.Environ(), "DISPLAY="+display)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// screenshotDir is where a test's screenshots go, named after the test so a
// matrix run does not overwrite itself.
func screenshotDir(t *testing.T) string {
	t.Helper()

	root := os.Getenv("GROUT_E2E_SCREENSHOTS")
	if root == "" {
		root = filepath.Join(os.TempDir(), "grout-e2e")
	}

	dir := filepath.Join(root, strings.ReplaceAll(t.Name(), "/", "_"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("making a place for screenshots: %v", err)
	}
	return dir
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
