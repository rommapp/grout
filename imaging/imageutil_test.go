package imaging

import (
	"image"
	_ "image/png" // register PNG decoder for image.Decode
	"os"
	"strings"
	"testing"

	goqr "github.com/piglig/go-qr"
)

func TestCreateTempQRCode(t *testing.T) {
	const content = "https://romm.example/pair/device?user_code=ABCD-1234"
	path, err := CreateTempQRCode(content, 300)
	if err != nil {
		t.Fatalf("CreateTempQRCode: %v", err)
	}
	defer os.Remove(path)

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open output: %v", err)
	}
	defer f.Close()

	img, format, err := image.Decode(f)
	if err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if format != "png" {
		t.Errorf("format = %q, want png", format)
	}

	// The corner must fall inside the quiet zone (white), proving the light
	// margin that scanners rely on is present.
	b := img.Bounds()
	r, g, bl, _ := img.At(b.Min.X, b.Min.Y).RGBA()
	if r>>8 != 0xff || g>>8 != 0xff || bl>>8 != 0xff {
		t.Errorf("corner pixel = (%d,%d,%d), want white quiet zone", r>>8, g>>8, bl>>8)
	}

	// The generated QR must still decode back to the exact content.
	decoded, err := goqr.Decode(img)
	if err != nil {
		t.Fatalf("QR did not decode: %v", err)
	}
	if decoded != content {
		t.Errorf("decoded QR = %q, want %q", decoded, content)
	}
}

// The caller is handed a path to clean up, so a failure that returns no path
// must not leave a file nobody knows about.
func TestCreateTempQRCode_LeavesNothingBehindOnFailure(t *testing.T) {
	before := qrFilesInTemp(t)

	// A zero size is rejected while writing the image, which is after the
	// temp file has already been made. Content too large to encode fails
	// earlier than that and never reaches this path.
	if _, err := CreateTempQRCode("https://example.com", 0); err == nil {
		t.Fatal("a zero size was accepted, so this no longer reaches the failure being guarded")
	}

	if after := qrFilesInTemp(t); after > before {
		t.Errorf("temp qr files went from %d to %d, so a failed encode left one behind", before, after)
	}
}

// qrFilesInTemp counts the qr code files sitting in the temp directory.
func qrFilesInTemp(t *testing.T) int {
	t.Helper()

	entries, err := os.ReadDir(os.TempDir())
	if err != nil {
		t.Skipf("cannot read the temp directory: %v", err)
	}

	count := 0
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "qrcode-") {
			count++
		}
	}
	return count
}
