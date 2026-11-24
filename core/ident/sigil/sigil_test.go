package sigil

import (
	"archive/zip"
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	binding "github.com/rommforge/argosy-sigil/bindings/go"

	"github.com/rommapp/grout/core/ident"
)

var _ ident.Extractor = Sigil{}

func TestVersionIsThePinnedRelease(t *testing.T) {
	if got := (Sigil{}).Version(); got != "1.0.0" {
		t.Fatalf("Version() = %q, want 1.0.0", got)
	}
	if got := Version(); got != "1.0.0" {
		t.Fatalf("Version() = %q, want 1.0.0", got)
	}
}

func TestEveryMappedPlatformIsKnownToSigil(t *testing.T) {
	for slug, v := range ident.Platforms() {
		if binding.PlatformFromSlug(v) == binding.PlatformAuto {
			t.Errorf("%s maps to %q which sigil does not know", slug, v)
		}
	}
}

func writeFile(t *testing.T, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestFilenameFallbackReadsTheSerial(t *testing.T) {
	p := writeFile(t, "Tactics Ogre [ULUS10565].iso", make([]byte, 64*1024))
	r := Sigil{}.Extract(p, "psp")
	if r.Status != ident.StatusOK {
		t.Fatalf("Status = %q", r.Status)
	}
	if r.TitleID != "ULUS10565" {
		t.Errorf("TitleID = %q", r.TitleID)
	}
	if r.Source != ident.SourceFilename {
		t.Errorf("Source = %q", r.Source)
	}
	if r.Platform != "psp" {
		t.Errorf("Platform = %q", r.Platform)
	}
}

func mario() []byte {
	rom := make([]byte, 1<<20)
	copy(rom, []byte{0x80, 0x37, 0x12, 0x40})
	copy(rom[0x20:], "SUPER MARIO 64      ")
	copy(rom[0x3B:], "NSME")
	return rom
}

func upperMD5(b []byte) string {
	s := md5.Sum(b)
	return strings.ToUpper(hex.EncodeToString(s[:]))
}

func wordReversed(b []byte) []byte {
	out := make([]byte, len(b))
	for i := 0; i+3 < len(b); i += 4 {
		out[i], out[i+1], out[i+2], out[i+3] = b[i+3], b[i+2], b[i+1], b[i]
	}
	return out
}

func TestN64HeaderAndHashes(t *testing.T) {
	rom := mario()
	p := writeFile(t, "mario.z64", rom)
	r := Sigil{}.Extract(p, "n64")
	if r.Status != ident.StatusOK {
		t.Fatalf("Status = %q", r.Status)
	}
	if r.TitleID != "NSME" {
		t.Errorf("TitleID = %q", r.TitleID)
	}
	if r.N64Header != "SUPER MARIO 64" {
		t.Errorf("N64Header = %q", r.N64Header)
	}
	if want := upperMD5(rom); r.N64MD5 != want {
		t.Errorf("N64MD5 = %q, want %q", r.N64MD5, want)
	}
	if want := upperMD5(wordReversed(rom)); r.N64MD5N64 != want {
		t.Errorf("N64MD5N64 = %q, want %q", r.N64MD5N64, want)
	}
}

func TestZippedN64MatchesTheRawRom(t *testing.T) {
	rom := mario()
	raw := Sigil{}.Extract(writeFile(t, "mario.z64", rom), "n64")

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.CreateHeader(&zip.FileHeader{Name: "mario.z64", Method: zip.Deflate})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(rom); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	z := Sigil{}.Extract(writeFile(t, "mario.zip", buf.Bytes()), "n64")

	if z.Status != ident.StatusOK {
		t.Fatalf("Status = %q", z.Status)
	}
	if z.TitleID != raw.TitleID || z.N64Header != raw.N64Header || z.N64MD5 != raw.N64MD5 || z.N64MD5N64 != raw.N64MD5N64 {
		t.Fatalf("zip %+v differs from raw %+v", z, raw)
	}
}

func TestUnreadableBinaryWithNoSerialIsNotFound(t *testing.T) {
	p := writeFile(t, "game.iso", make([]byte, 64*1024))
	r := Sigil{}.Extract(p, "psp")
	if r.Status != ident.StatusNotFound {
		t.Fatalf("Status = %q", r.Status)
	}
	if (r != ident.Result{Status: ident.StatusNotFound}) {
		t.Errorf("result has fields set: %+v", r)
	}
}

func TestMissingFileIsIO(t *testing.T) {
	r := Sigil{}.Extract(filepath.Join(t.TempDir(), "absent.iso"), "psp")
	if r.Status != ident.StatusIO {
		t.Fatalf("Status = %q", r.Status)
	}
}
