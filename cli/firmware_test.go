package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rommapp/grout/api"
)

const validHostDefinition = `
id = "hostfw"
name = "Host firmware"
maintained = "community"

[paths]
card = { value = "/mnt/SDCARD" }
roms = { value = "{card}/Roms" }
bios = { value = "{card}/Bios" }
saves = { value = "{card}/Saves" }
`

func writeDefinition(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "fw.toml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestFirmwareValidateHost(t *testing.T) {
	code, e, _ := runCLI(t, t.TempDir(), "firmware", "validate", writeDefinition(t, validHostDefinition), "--host")
	if code != ExitOK || !e.OK {
		t.Fatalf("valid: code=%d env=%+v", code, e)
	}

	missing := strings.Replace(validHostDefinition, `card = { value = "/mnt/SDCARD" }`+"\n", "", 1)
	missing = strings.ReplaceAll(missing, "{card}", "/mnt/SDCARD")
	code, e, _ = runCLI(t, t.TempDir(), "firmware", "validate", writeDefinition(t, missing), "--host")
	if code != ExitNotConfigured || e.OK || e.Error == nil || e.Error.Code != api.CodeBadDefinition || !strings.Contains(e.Error.Message, "paths.card") {
		t.Fatalf("missing card: code=%d env=%+v", code, e)
	}

	var out, errb strings.Builder
	got := Run(t.Context(), Env{Dir: t.TempDir(), Stdout: &out, Stderr: &errb}, []string{"firmware", "validate", writeDefinition(t, validHostDefinition)})
	if got != ExitUsage {
		t.Fatalf("without --host: code=%d", got)
	}
}
