package cli

import (
	"encoding/json"
	"testing"
)

func TestGamelistRefreshCommand(t *testing.T) {
	dir := fetchDirFW(t, fetchServer(t), "knulli")
	root := t.TempDir()
	if code, _, raw := runFWAt(t, dir, "knulli", root, "fetch", "game", "1"); code != ExitOK {
		t.Fatalf("code=%d env=%s", code, raw)
	}
	code, e, raw := runFWAt(t, dir, "knulli", root, "gamelist", "refresh")
	if code != ExitOK || !e.OK {
		t.Fatalf("code=%d env=%s", code, raw)
	}
	var res map[string]json.RawMessage
	if err := json.Unmarshal(e.Result, &res); err != nil {
		t.Fatal(err)
	}
	if _, ok := res["written"]; !ok {
		t.Fatalf("result=%s", e.Result)
	}
}

func TestGamelistRefreshPlatformArg(t *testing.T) {
	dir := fetchDirFW(t, fetchServer(t), "knulli")
	root := t.TempDir()
	code, e, raw := runFWAt(t, dir, "knulli", root, "gamelist", "refresh", "1")
	if code != ExitOK || !e.OK {
		t.Fatalf("code=%d env=%s", code, raw)
	}
	for _, args := range [][]string{
		{"gamelist", "refresh", "abc"},
		{"gamelist", "refresh", "0"},
		{"gamelist", "refresh", "1", "2"},
	} {
		if code := runPlain(t, t.TempDir(), args...); code != ExitUsage {
			t.Errorf("%v exit = %d, want %d", args, code, ExitUsage)
		}
	}
}
