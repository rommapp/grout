package imageutil

import (
	"image"
	_ "image/png" // register PNG decoder for image.Decode
	"os"
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
