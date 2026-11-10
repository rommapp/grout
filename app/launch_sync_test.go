package main

import "testing"

func TestSyncRomArg(t *testing.T) {
	cases := []struct {
		args []string
		rom  string
		ok   bool
	}{
		{nil, "", false},
		{[]string{"--sync-rom"}, "", false},
		{[]string{"--sync-rom", ""}, "", false},
		{[]string{"--sync-rom", "/mnt/SDCARD/Roms/GB/Tetris.gb"}, "/mnt/SDCARD/Roms/GB/Tetris.gb", true},
		{[]string{"--other", "x"}, "", false},
		{[]string{"--sync-rom", "a", "b"}, "", false},
	}
	for _, c := range cases {
		rom, ok := syncRomArg(c.args)
		if rom != c.rom || ok != c.ok {
			t.Errorf("syncRomArg(%q) = (%q, %v), want (%q, %v)", c.args, rom, ok, c.rom, c.ok)
		}
	}
}
