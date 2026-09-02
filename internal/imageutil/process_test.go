package imageutil

import (
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestFitDimensions(t *testing.T) {
	tests := []struct {
		name                   string
		imgW, imgH, maxW, maxH int
		wantW, wantH           int
	}{
		{"square into square", 100, 100, 50, 50, 50, 50},
		{"wide image is limited by width", 200, 100, 100, 100, 100, 50},
		{"tall image is limited by height", 100, 200, 100, 100, 50, 100},
		{"already the right size is unchanged", 64, 64, 64, 64, 64, 64},
		{"box aspect is respected, not just the larger side", 300, 100, 90, 60, 90, 30},

		// Scaling goes both ways: a small image is enlarged to fill the box.
		{"small image is upscaled", 10, 10, 100, 100, 100, 100},
		{"small wide image is upscaled", 20, 10, 100, 100, 100, 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotW, gotH := FitDimensions(tt.imgW, tt.imgH, tt.maxW, tt.maxH)
			if gotW != tt.wantW || gotH != tt.wantH {
				t.Errorf("FitDimensions(%d, %d, %d, %d) = (%d, %d), want (%d, %d)",
					tt.imgW, tt.imgH, tt.maxW, tt.maxH, gotW, gotH, tt.wantW, tt.wantH)
			}
		})
	}
}

// A zero dimension must not divide by zero; the original size comes back.
func TestFitDimensions_DegenerateInputs(t *testing.T) {
	for _, tt := range []struct{ imgW, imgH, maxW, maxH int }{
		{0, 0, 100, 100},
		{100, 0, 100, 100},
		{0, 100, 100, 100},
		{100, 100, 0, 100},
		{100, 100, 100, 0},
		{-10, 50, 100, 100},
	} {
		gotW, gotH := FitDimensions(tt.imgW, tt.imgH, tt.maxW, tt.maxH)
		if gotW != tt.imgW || gotH != tt.imgH {
			t.Errorf("FitDimensions(%d, %d, %d, %d) = (%d, %d), want the original size",
				tt.imgW, tt.imgH, tt.maxW, tt.maxH, gotW, gotH)
		}
	}
}

// FitDimensions must never return a zero side for a real image, which would
// make image.NewRGBA produce an empty picture.
func TestFitDimensions_NeverReturnsAZeroSideForRealImages(t *testing.T) {
	for _, dims := range [][2]int{{1, 1000}, {1000, 1}, {3, 700}, {700, 3}} {
		w, h := FitDimensions(dims[0], dims[1], 200, 200)
		if w <= 0 || h <= 0 {
			t.Errorf("FitDimensions(%d, %d, 200, 200) = (%d, %d), want both sides positive",
				dims[0], dims[1], w, h)
		}
	}
}

func writePNG(t *testing.T, path string, w, h int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := range w {
		for y := range h {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 128, A: 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
}

func decodeSize(t *testing.T, path string) (int, int, string) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	cfg, format, err := image.DecodeConfig(f)
	if err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return cfg.Width, cfg.Height, format
}

func TestProcessArtImageTo_ResizesInPlace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cover.png")
	writePNG(t, path, 400, 200)

	if err := ProcessArtImageTo(path, 100, 100); err != nil {
		t.Fatalf("ProcessArtImageTo: %v", err)
	}

	w, h, format := decodeSize(t, path)
	if w != 100 || h != 50 {
		t.Errorf("resized to %dx%d, want 100x50", w, h)
	}
	if format != "png" {
		t.Errorf("format = %q, want png", format)
	}
}

// A JPEG must be re-encoded as PNG even when no resize is needed, because the
// firmwares expect .png files.
func TestProcessArtImageTo_ConvertsJPEGToPNG(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cover.png")

	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := jpeg.Encode(f, img, nil); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	f.Close()

	if err := ProcessArtImageTo(path, 64, 64); err != nil {
		t.Fatalf("ProcessArtImageTo: %v", err)
	}

	if _, _, format := decodeSize(t, path); format != "png" {
		t.Errorf("format = %q, want png", format)
	}
}

// Anything that is not an image must be reported as an error, which is what
// stops romm.ArtFetcher from leaving an error page on disk as artwork.
func TestProcessArtImageTo_RejectsNonImages(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cover.png")
	if err := os.WriteFile(path, []byte("<html>not an image</html>"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := ProcessArtImageTo(path, 100, 100); err == nil {
		t.Fatal("expected an error for a non-image payload")
	}
}

func TestProcessArtImageTo_MissingFile(t *testing.T) {
	if err := ProcessArtImageTo(filepath.Join(t.TempDir(), "nope.png"), 100, 100); err == nil {
		t.Fatal("expected an error for a missing file")
	}
}
