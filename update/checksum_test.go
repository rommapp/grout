package update

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/atomic"

	"grout/cfw"
)

// Nothing else checks what was downloaded: the expected size only drives the
// progress bar. Installing without a checksum would unpack a zip over the
// install on trust alone.
func TestPerformUpdate_RefusesWithoutAChecksum(t *testing.T) {
	t.Setenv(cfw.EnvVar, string(cfw.MuOS))
	t.Setenv("BASE_PATH", t.TempDir())

	err := PerformUpdate(cfw.MuOS, "http://example.invalid/grout.zip", 1024, "", &atomic.Float64{})

	if err == nil {
		t.Fatal("an update with no checksum was accepted")
	}
	if !strings.Contains(err.Error(), "checksum") {
		t.Errorf("error = %q, want it to say why", err)
	}
}

func TestVerifySHA256(t *testing.T) {
	path := filepath.Join(t.TempDir(), "asset.zip")
	content := []byte("a grout release")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}

	sum := sha256.Sum256(content)
	correct := hex.EncodeToString(sum[:])

	if err := verifySHA256(path, correct); err != nil {
		t.Errorf("a matching checksum was rejected: %v", err)
	}

	// A zip that is not what the manifest described is the case this exists
	// for: a corrupt download, or one that was swapped out.
	if err := verifySHA256(path, strings.Repeat("0", 64)); err == nil {
		t.Error("a mismatched checksum was accepted")
	}
}

// An asset the manifest publishes no checksum for is not offered at all, so
// the user is not made to download something that will then be refused.
func TestCheckForUpdate_RejectsAnAssetWithNoChecksum(t *testing.T) {
	release := &ChannelRelease{
		Version: "99.0.0",
		Assets: map[string]*ChannelAsset{
			assetNameFor(t): {URL: "https://example.invalid/grout.zip", Size: 1024},
		},
	}

	_, err := checkRelease(cfw.MuOS, release, "1.0.0")

	if err == nil {
		t.Fatal("an asset with no checksum was offered as an update")
	}
	if !strings.Contains(err.Error(), "checksum") {
		t.Errorf("error = %q, want it to say why", err)
	}
}

// The same release with a checksum is offered, so the refusal above is about
// the missing checksum and not about the release being unusable.
func TestCheckForUpdate_OffersAnAssetWithAChecksum(t *testing.T) {
	release := &ChannelRelease{
		Version: "99.0.0",
		Assets: map[string]*ChannelAsset{
			assetNameFor(t): {URL: "https://example.invalid/grout.zip", Size: 1024, SHA256: strings.Repeat("a", 64)},
		},
	}

	info, err := checkRelease(cfw.MuOS, release, "1.0.0")
	if err != nil {
		t.Fatalf("checkRelease: %v", err)
	}
	if !info.UpdateAvailable {
		t.Error("a newer version with a checksum must be offered")
	}
}

func assetNameFor(t *testing.T) string {
	t.Helper()
	name := GetDistributionAssetName(cfw.MuOS)
	if name == "" {
		t.Skip("no distribution asset name for muOS")
	}
	return name
}
