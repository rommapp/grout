package main

import (
	"testing"

	"grout/settings"
)

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
		{"on request", []string{"--sync-rom", "/r/Tetris.gb", "--now"}, launchSync{romPath: "/r/Tetris.gb", now: true}, true},
		{"other flag", []string{"--other", "x"}, launchSync{}, false},
		{"unknown extra", []string{"--sync-rom", "a", "b"}, launchSync{}, false},
		{"after and on request", []string{"--sync-rom", "a", "--upload-only", "--now"}, launchSync{}, false},
	}
	for _, c := range cases {
		got, ok := parseLaunchSync(c.args)
		if got != c.want || ok != c.ok {
			t.Errorf("%s: parseLaunchSync(%q) = (%+v, %v), want (%+v, %v)", c.name, c.args, got, ok, c.want, c.ok)
		}
	}
}

func TestWhichSyncsTheSettingAsksFor(t *testing.T) {
	before := launchSync{romPath: "r"}
	after := launchSync{romPath: "r", uploadOnly: true}
	byHand := launchSync{romPath: "r", now: true}

	cases := []struct {
		mode                  settings.LaunchSyncMode
		before, after, byHand bool
	}{
		{settings.LaunchSyncOff, false, false, true},
		{settings.LaunchSyncBefore, true, false, true},
		{settings.LaunchSyncBeforeAndAfter, true, true, true},
	}
	for _, c := range cases {
		if got := before.wanted(c.mode); got != c.before {
			t.Errorf("mode %q: before = %v, want %v", c.mode, got, c.before)
		}
		if got := after.wanted(c.mode); got != c.after {
			t.Errorf("mode %q: after = %v, want %v", c.mode, got, c.after)
		}
		if got := byHand.wanted(c.mode); got != c.byHand {
			t.Errorf("mode %q: by hand = %v, want %v", c.mode, got, c.byHand)
		}
	}
}
