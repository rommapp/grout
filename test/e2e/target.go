//go:build e2e

package e2e

import "testing"

// target is where a run happens.
//
// The tests do not know which they are talking to. What differs between them
// is only how a file is read, a button is pressed and the screen is captured,
// so that is all this covers.
type target interface {
	// label says what this target is, for a test's output.
	label() string

	// cardRoot is where the synthetic card lives on the target. Grout is
	// pointed at it with BASE_PATH.
	cardRoot() string

	// readFile reads from the target's filesystem.
	readFile(path string) ([]byte, error)

	// writeFile puts a file on the target's filesystem, making the folders
	// above it.
	writeFile(path string, content []byte) error

	// makeDir makes a folder, and the folders above it.
	//
	// It is its own operation rather than a file written into the folder,
	// because grout reads the rom folders and would count a placeholder as a
	// game that had already been downloaded.
	makeDir(path string) error

	// listFiles names every file under a folder on the target, for a failure
	// to show what landed when the expected thing did not.
	listFiles(root string) []string

	// launch starts grout and stops it when the test ends.
	launch(t *testing.T, env map[string]string)

	// press sends button presses, named the way the toolkit maps them.
	press(keys ...string) error

	// capture saves what is on screen to a file on this machine.
	capture(localPath string) error

	// awaitStill waits for the screen to stop changing, so a screenshot is
	// not taken of a transition and a button is not pressed into one.
	awaitStill()
}

// chooseTarget decides where a run happens. For now that is always this
// machine.
func chooseTarget(t *testing.T, opts options) target {
	t.Helper()
	return newLocal(t, opts.Width, opts.Height)
}
