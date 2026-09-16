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
	"bytes"
	"encoding/json"
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

const (
	// waitTimeout bounds how long a step waits for grout to report something.
	// A slow CI runner emulating another architecture needs the room.
	waitTimeout = 30 * time.Second
	// settle is how long to let a screen redraw before a screenshot.
	settle = 500 * time.Millisecond
)

// displays hands out a display number per session.
//
// Sharing one would make each test depend on the previous one's X server
// having fully gone, lock file and all, which it has not always done by the
// time the next starts.
var displays atomic.Int32

// session is one run of grout against a synthetic card.
type session struct {
	t *testing.T
	// display is this session's own X server.
	display  string
	cardPath string
	logPath  string
	shotDir  string
	// read is how far through the log this session has already looked, so a
	// later wait cannot match a line an earlier one already consumed.
	read int
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

// options say which device the run should look like.
type options struct {
	// CFW is the firmware to present as, which decides every path grout uses.
	CFW string
	// Width and Height are the virtual screen, ideally the real device's.
	Width, Height int
	// Server, when set, is a RomM the card is already paired with, so the run
	// starts at the library rather than at the login screen.
	Server *server
	// Platforms are the rom folders to map, by their RomM slug. Without a
	// mapping a platform has nowhere to download to and is not offered.
	Platforms []string
	// Existing are folders a real card of this firmware would already have.
	// Grout writes into some of them and does not create them itself, so a
	// card without them behaves in ways a device never would.
	Existing []string
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
	for _, dir := range append([]string{"ROMS", "BIOS", "logs"}, opts.Existing...) {
		if err := os.MkdirAll(filepath.Join(card, dir), 0o755); err != nil {
			t.Fatalf("laying out the card: %v", err)
		}
	}

	s := &session{
		t:        t,
		display:  fmt.Sprintf(":%d", 90+displays.Add(1)),
		cardPath: card,
		logPath:  filepath.Join(card, "logs", "app.log"),
		shotDir:  screenshotDir(t),
	}

	if opts.Server != nil {
		writeConfig(t, card, opts)
	}

	s.startBackground("Xvfb", s.display, "-screen", "0",
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
		"DISPLAY="+s.display,
	)
	// Kept so a failure can show it. Grout writes its own log to the card, but
	// anything that stops it before that starts only appears here.
	grout.Stdout = &s.output
	grout.Stderr = &s.output
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
	cmd.Env = append(os.Environ(), "DISPLAY="+s.display)
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
		if exec.Command("xdpyinfo", "-display", s.display).Run() == nil {
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
// Each key waits for the screen to stop moving first. A key sent while a
// screen is still being drawn lands on whatever was there before, which shows
// up as a test that passes alone and fails in a run.
//
// The keys go through XTEST rather than being posted at the window, because
// SDL ignores events it can tell were synthesised.
func (s *session) press(keys ...string) {
	s.t.Helper()

	for _, key := range keys {
		s.awaitStill()
		if _, err := s.x("xdotool", "key", "--clearmodifiers", key); err != nil {
			s.t.Fatalf("pressing %s: %v", key, err)
		}
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
	s.t.Fatalf("grout never logged %q within %s\n\nlog so far:\n%s\noutput:\n%s",
		msg, waitTimeout, s.logTail(), indent(s.output.String()))
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
	if len(entries) > 25 {
		entries = entries[len(entries)-25:]
	}

	var b strings.Builder
	for _, entry := range entries {
		fmt.Fprintf(&b, "  %-5s %s", entry.Level, entry.Msg)
		// Anything that went wrong says why in its fields, and a line without
		// them is no use when a test is failing.
		for _, name := range []string{"error", "path", "file"} {
			if value, ok := entry.fields[name]; ok {
				fmt.Fprintf(&b, "  %s=%v", name, value)
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

// awaitStill waits for the screen to stop changing.
//
// Progress bars, transitions and the cache build all mean a screenshot taken
// at a fixed moment catches whatever happened to be mid flight. Waiting for
// two identical frames catches the screen a person would have waited for,
// without having to name a log line for every one of them.
func (s *session) awaitStill() {
	s.t.Helper()

	deadline := time.Now().Add(waitTimeout)
	previous := ""

	for time.Now().Before(deadline) {
		current := s.frame()
		if current != "" && current == previous {
			return
		}
		previous = current
		time.Sleep(settle)
	}

	s.t.Logf("the screen was still changing after %s", waitTimeout)
}

// frame is a fingerprint of what is on screen, used only to tell one frame
// from the next.
func (s *session) frame() string {
	out, err := s.x("sh", "-c", "import -window root png:- | md5sum")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// screenshot saves what is on screen, for a person to look at when a test
// fails. CI keeps them as artifacts.
func (s *session) screenshot(name string) {
	s.t.Helper()

	s.awaitStill()
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

// awaitCardFile waits for grout to put a file on the card.
//
// A download finishes when the file is there, which is a better thing to wait
// on than a log line: it is what the user is actually after.
func (s *session) awaitCardFile(relative string) {
	s.t.Helper()

	deadline := time.Now().Add(waitTimeout)
	for time.Now().Before(deadline) {
		if s.cardHas(relative) {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}

	s.screenshot("timeout")
	s.t.Fatalf("grout never wrote %s to the card within %s\n\ncard holds:\n%s\nlog so far:\n%s",
		relative, waitTimeout, indent(s.cardTree()), s.logTail())
}

// putSave writes a save onto the card where the firmware keeps them, as
// though a game had been played.
func (s *session) putSave(relative, content string) {
	s.t.Helper()

	path := filepath.Join(s.cardPath, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		s.t.Fatalf("making a save folder: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		s.t.Fatalf("writing a save: %v", err)
	}
}

// cardFile reads a file grout wrote to the card.
func (s *session) cardFile(relative string) []byte {
	s.t.Helper()

	content, err := os.ReadFile(filepath.Join(s.cardPath, relative))
	if err != nil {
		s.t.Fatalf("reading %s from the card: %v", relative, err)
	}
	return content
}

// cardTree lists what is on the card, for a failure to show what did land
// when the expected thing did not.
func (s *session) cardTree() string {
	var found []string
	_ = filepath.WalkDir(s.cardPath, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		relative, _ := filepath.Rel(s.cardPath, path)
		found = append(found, relative)
		return nil
	})

	sort.Strings(found)
	return strings.Join(found, "\n")
}

func (s *session) x(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Env = append(os.Environ(), "DISPLAY="+s.display)
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

// indent sets a subprocess's output apart from the failure around it.
func indent(s string) string {
	if s == "" {
		return "  (nothing)\n"
	}
	var b strings.Builder
	for _, line := range strings.Split(strings.TrimRight(s, "\n"), "\n") {
		b.WriteString("  " + line + "\n")
	}
	return b.String()
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// writeConfig puts grout on the card already signed in.
//
// The login flow is worth testing on its own, but every screen behind it is
// unreachable while each test has to walk through it first, and pairing is
// meant to be answered by a person.
func writeConfig(t *testing.T, card string, opts options) {
	t.Helper()

	type host struct {
		RootURI    string `json:"root_uri"`
		Username   string `json:"username"`
		Token      string `json:"token"`
		DeviceID   string `json:"device_id,omitempty"`
		DeviceName string `json:"device_name,omitempty"`
	}
	type mapping struct {
		RomMSlug     string `json:"romm_slug"`
		RelativePath string `json:"relative_path"`
	}

	config := struct {
		Hosts             []host             `json:"hosts"`
		DirectoryMappings map[string]mapping `json:"directory_mappings,omitempty"`
		LogLevel          string             `json:"log_level"`
	}{
		Hosts: []host{{
			RootURI:  opts.Server.URL,
			Username: opts.Server.Username,
			Token:    opts.Server.Token,
			// Save sync needs a device the server knows. Grout registers its
			// own when a person walks the pairing flow; a seeded card is
			// handed one instead.
			DeviceID:   opts.Server.DeviceID,
			DeviceName: "grout-e2e-device",
		}},
		DirectoryMappings: map[string]mapping{},
		// Grout defaults to logging errors only, and everything worth waiting
		// for here is logged at debug.
		LogLevel: "debug",
	}

	for _, slug := range opts.Platforms {
		config.DirectoryMappings[slug] = mapping{RomMSlug: slug, RelativePath: slug}
		if err := os.MkdirAll(filepath.Join(card, "ROMS", slug), 0o755); err != nil {
			t.Fatalf("making a rom folder for %s: %v", slug, err)
		}
	}

	encoded, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		t.Fatalf("encoding the config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(card, "config.json"), encoded, 0o644); err != nil {
		t.Fatalf("writing the config: %v", err)
	}
}
