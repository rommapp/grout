package fetch

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"testing"
)

func solid(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 200, G: 40, B: 40, A: 255})
		}
	}
	return img
}

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, solid(w, h)); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func decodedSize(t *testing.T, data []byte) (int, int) {
	t.Helper()
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil || format != "png" {
		t.Fatalf("format=%q err=%v", format, err)
	}
	return img.Bounds().Dx(), img.Bounds().Dy()
}

func TestToPNGFits(t *testing.T) {
	out, err := ToPNG(pngBytes(t, 400, 200), Size{Width: 100, Height: 100})
	if err != nil {
		t.Fatal(err)
	}
	if w, h := decodedSize(t, out); w != 100 || h != 50 {
		t.Fatalf("got %dx%d", w, h)
	}
}

func TestToPNGUpscales(t *testing.T) {
	out, err := ToPNG(pngBytes(t, 20, 10), Size{Width: 100, Height: 100})
	if err != nil {
		t.Fatal(err)
	}
	if w, h := decodedSize(t, out); w != 100 || h != 50 {
		t.Fatalf("got %dx%d", w, h)
	}
}

func TestToPNGZeroKeepsSize(t *testing.T) {
	out, err := ToPNG(pngBytes(t, 37, 21), Size{})
	if err != nil {
		t.Fatal(err)
	}
	if w, h := decodedSize(t, out); w != 37 || h != 21 {
		t.Fatalf("got %dx%d", w, h)
	}
}

func TestToPNGOneZeroAxisIsUnbounded(t *testing.T) {
	out, err := ToPNG(pngBytes(t, 400, 200), Size{Width: 100})
	if err != nil {
		t.Fatal(err)
	}
	if w, h := decodedSize(t, out); w != 100 || h != 50 {
		t.Fatalf("width only: got %dx%d", w, h)
	}
	out, err = ToPNG(pngBytes(t, 400, 200), Size{Height: 20})
	if err != nil {
		t.Fatal(err)
	}
	if w, h := decodedSize(t, out); w != 40 || h != 20 {
		t.Fatalf("height only: got %dx%d", w, h)
	}
}

func TestToPNGExtremeAspectKeepsOnePixel(t *testing.T) {
	out, err := ToPNG(pngBytes(t, 1000, 2), Size{Width: 10, Height: 10})
	if err != nil {
		t.Fatal(err)
	}
	if w, h := decodedSize(t, out); w != 10 || h != 1 {
		t.Fatalf("got %dx%d", w, h)
	}
}

func TestToPNGFormats(t *testing.T) {
	var j, g bytes.Buffer
	if err := jpeg.Encode(&j, solid(8, 8), nil); err != nil {
		t.Fatal(err)
	}
	if err := gif.Encode(&g, solid(8, 8), nil); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{"jpeg": j.Bytes(), "gif": g.Bytes()} {
		out, err := ToPNG(data, Size{})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if w, h := decodedSize(t, out); w != 8 || h != 8 {
			t.Fatalf("%s: got %dx%d", name, w, h)
		}
	}
}

func TestToPNGWebP(t *testing.T) {
	data, err := base64.StdEncoding.DecodeString("UklGRhoAAABXRUJQVlA4TA0AAAAvAAAAEAcQERGIiP4HAA==")
	if err != nil {
		t.Fatal(err)
	}
	out, err := ToPNG(data, Size{})
	if err != nil {
		t.Fatal(err)
	}
	if w, h := decodedSize(t, out); w != 1 || h != 1 {
		t.Fatalf("got %dx%d", w, h)
	}
}

func TestToPNGRejectsGarbage(t *testing.T) {
	if _, err := ToPNG([]byte("not an image at all"), Size{}); err == nil {
		t.Fatal("expected error")
	}
	if _, err := ToPNG(nil, Size{}); err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestToPNGRejectsHugeDimensions(t *testing.T) {
	var b bytes.Buffer
	img := image.NewGray(image.Rect(0, 0, 5000, 4000))
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	if _, err := ToPNG(b.Bytes(), Size{}); err == nil {
		t.Fatal("expected refusal of oversized image")
	}
}
