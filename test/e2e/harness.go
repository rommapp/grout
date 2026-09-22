//go:build e2e

// Package e2e drives grout's real UI, either on this machine or on a handheld.
//
// Assertions are on grout's own structured log and on what it writes, rather
// than on pixels: the log says what happened, a screenshot only says what it
// looked like, and fonts and anti-aliasing make the second answer differ
// between machines.
//
// Screenshots are still taken, as artifacts for a person to look at when
// something fails.
package e2e

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	// waitTimeout bounds how long a step waits for grout to report something.
	// A slow runner, or a handheld, needs the room.
	waitTimeout = 30 * time.Second
	// settle is how long to leave between frames when watching for the screen
	// to stop changing.
	settle = 500 * time.Millisecond
)

// session is one run of grout against a synthetic card.
type session struct {
	t       *testing.T
	on      target
	shotDir string
	// read is how far through the log this session has already looked, so a
	// later wait cannot match a line an earlier one already consumed.
	read int
}

// options say which device the run should look like.
type options struct {
	// CFW is the firmware to present as, which decides every path grout uses.
	CFW string
	// Width and Height are the virtual screen, ideally the real device's.
	// Ignored on a handheld, which has the screen it has.
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

// start lays out a card and runs grout against it.
//
// Everything is torn down when the test ends, including on failure.
func start(t *testing.T, opts options) *session {
	t.Helper()

	if opts.Width == 0 {
		opts.Width, opts.Height = 1024, 768
	}

	s := &session{t: t, on: chooseTarget(t, opts), shotDir: screenshotDir(t)}
	t.Logf("running on %s", s.on.label())

	s.layOutCard(opts)
	s.on.launch(t, map[string]string{
		"CFW":       opts.CFW,
		"BASE_PATH": s.on.cardRoot(),
	})

	return s
}

// layOutCard puts a card on the target in the state the test asked for.
func (s *session) layOutCard(opts options) {
	s.t.Helper()

	folders := append([]string{"ROMS", "BIOS", "logs"}, opts.Existing...)
	for _, slug := range opts.Platforms {
		folders = append(folders, path.Join("ROMS", slug))
	}
	for _, folder := range folders {
		if err := s.on.makeDir(path.Join(s.on.cardRoot(), folder)); err != nil {
			s.t.Fatalf("making %s on the card: %v", folder, err)
		}
	}

	if opts.Server != nil {
		s.writeConfig(opts)
	}
}

// writeConfig puts grout on the card already signed in.
//
// The login flow is worth testing on its own, but every screen behind it is
// unreachable while each test has to walk through it first, and pairing is
// meant to be answered by a person.
func (s *session) writeConfig(opts options) {
	s.t.Helper()

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
	}

	encoded, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		s.t.Fatalf("encoding the config: %v", err)
	}
	s.write("config.json", encoded)
}

// press sends button presses, named as the toolkit maps them: arrow keys for
// the d-pad, a b x y for the face buttons, Return for start, space for select.
//
// Each key waits for the screen to stop moving first. A key sent while a
// screen is still being drawn lands on whatever was there before, which shows
// up as a test that passes alone and fails in a run.
func (s *session) press(keys ...string) {
	s.t.Helper()

	for _, key := range keys {
		s.on.awaitStill()
		if err := s.on.press(key); err != nil {
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
	s.t.Fatalf("grout never logged %q within %s\n\nlog so far:\n%s", msg, waitTimeout, s.logTail())
	return logEntry{}
}

func (s *session) readLog() []logEntry {
	content, err := s.on.readFile(path.Join(s.on.cardRoot(), "logs", "app.log"))
	if err != nil {
		return nil
	}

	var entries []logEntry
	scanner := bufio.NewScanner(bytes.NewReader(content))
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

// screenshot saves what is on screen, for a person to look at when a test
// fails. CI keeps them as artifacts.
func (s *session) screenshot(name string) {
	s.t.Helper()

	s.on.awaitStill()
	local := filepath.Join(s.shotDir, name+".png")
	if err := s.on.capture(local); err != nil {
		s.t.Logf("could not capture %s: %v", name, err)
		return
	}
	s.t.Logf("screenshot: %s", local)
}

// cardHas reports whether grout wrote a file to the card.
func (s *session) cardHas(relative string) bool {
	_, err := s.on.readFile(path.Join(s.on.cardRoot(), relative))
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
		relative, waitTimeout, indent(strings.Join(s.on.listFiles(s.on.cardRoot()), "\n")), s.logTail())
}

// cardFile reads a file grout wrote to the card.
func (s *session) cardFile(relative string) []byte {
	s.t.Helper()

	content, err := s.on.readFile(path.Join(s.on.cardRoot(), relative))
	if err != nil {
		s.t.Fatalf("reading %s from the card: %v", relative, err)
	}
	return content
}

// putSave writes a save onto the card where the firmware keeps them, as
// though a game had been played.
func (s *session) putSave(relative, content string) {
	s.t.Helper()
	s.write(relative, []byte(content))
}

func (s *session) write(relative string, content []byte) {
	s.t.Helper()

	if err := s.on.writeFile(path.Join(s.on.cardRoot(), relative), content); err != nil {
		s.t.Fatalf("writing %s to the card: %v", relative, err)
	}
}

// awaitStill waits for the screen to stop changing.
func (s *session) awaitStill() { s.on.awaitStill() }

// indent sets a block apart from the failure around it.
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

// screenshotDir is where a test's screenshots go, named after the test so a
// matrix run does not overwrite itself.
func screenshotDir(t *testing.T) string {
	t.Helper()

	root := envOr("GROUT_E2E_SCREENSHOTS", filepath.Join(os.TempDir(), "grout-e2e"))
	dir := filepath.Join(root, strings.ReplaceAll(t.Name(), "/", "_"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("making a place for screenshots: %v", err)
	}
	return dir
}
