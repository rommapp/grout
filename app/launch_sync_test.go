package main

import "testing"

func TestParseLaunchSync(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want launchSync
		ok   bool
	}{
		{"nothing", nil, launchSync{}, false},
		{"no rom", []string{"--sync-rom"}, launchSync{}, false},
		{"empty rom", []string{"--sync-rom", ""}, launchSync{}, false},
		{"before", []string{"--sync-rom", "/r/Tetris.gb"}, launchSync{romPath: "/r/Tetris.gb"}, true},
		{"after", []string{"--sync-rom", "/r/Tetris.gb", "--upload-only"}, launchSync{romPath: "/r/Tetris.gb", uploadOnly: true}, true},
		{"other flag", []string{"--other", "x"}, launchSync{}, false},
		{"unknown extra", []string{"--sync-rom", "a", "b"}, launchSync{}, false},
		{"too many", []string{"--sync-rom", "a", "--upload-only", "b"}, launchSync{}, false},
	}
	for _, c := range cases {
		got, ok := parseLaunchSync(c.args)
		if got != c.want || ok != c.ok {
			t.Errorf("%s: parseLaunchSync(%q) = (%+v, %v), want (%+v, %v)", c.name, c.args, got, ok, c.want, c.ok)
		}
	}
}
