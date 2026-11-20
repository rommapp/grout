package firmware

import (
	"strings"
	"testing"

	"github.com/rommapp/grout/core/firmware/host"
)

func TestLoadHostJoinsErrors(t *testing.T) {
	_, err := LoadHost([]byte(`
id = "x"
maintained = "community"
[paths]
roms = { value = "/r" }
bios = { value = "/b" }
saves = { value = "/s" }
`), Registry{})
	if err == nil || !strings.Contains(err.Error(), "paths.card: required") || !strings.Contains(err.Error(), "name: required") {
		t.Fatalf("err=%v", err)
	}
}

func TestMergeHooksExtraWins(t *testing.T) {
	base := Registry{Path: map[string]PathHook{
		"x": func(host.Host, map[string]string) (string, error) { return "base", nil },
		"y": func(host.Host, map[string]string) (string, error) { return "y", nil },
	}}
	extra := Registry{Path: map[string]PathHook{
		"x": func(host.Host, map[string]string) (string, error) { return "extra", nil },
	}}
	m := MergeHooks(base, extra)
	if v, _ := m.Path["x"](nil, nil); v != "extra" {
		t.Fatal(v)
	}
	if v, _ := m.Path["y"](nil, nil); v != "y" {
		t.Fatal(v)
	}
}
