package fetch

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const maxImagePixels = 16 << 20

var ErrImageTooLarge = errors.New("image dimensions exceed the safe limit")

func fitDimensions(imgW, imgH, maxW, maxH int) (int, int) {
	if imgW <= 0 || imgH <= 0 || (maxW <= 0 && maxH <= 0) {
		return imgW, imgH
	}
	if maxW <= 0 {
		maxW = 1 << 30
	}
	if maxH <= 0 {
		maxH = 1 << 30
	}
	imgAspect := float64(imgW) / float64(imgH)
	boxAspect := float64(maxW) / float64(maxH)
	if imgAspect > boxAspect {
		return maxW, atLeastOne(int(float64(maxW) / imgAspect))
	}
	return atLeastOne(int(float64(maxH) * imgAspect)), maxH
}

func atLeastOne(n int) int {
	if n < 1 {
		return 1
	}
	return n
}

func ToPNG(src []byte, max Size) ([]byte, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(src))
	if err != nil {
		return nil, fmt.Errorf("decoding image: %w", err)
	}
	if int64(cfg.Width)*int64(cfg.Height) > maxImagePixels {
		return nil, fmt.Errorf("%dx%d: %w", cfg.Width, cfg.Height, ErrImageTooLarge)
	}
	img, _, err := image.Decode(bytes.NewReader(src))
	if err != nil {
		return nil, fmt.Errorf("decoding image: %w", err)
	}
	bounds := img.Bounds()
	w, h := fitDimensions(bounds.Dx(), bounds.Dy(), max.Width, max.Height)
	if w != bounds.Dx() || h != bounds.Dy() {
		dst := image.NewRGBA(image.Rect(0, 0, w, h))
		draw.BiLinear.Scale(dst, dst.Bounds(), img, bounds, draw.Over, nil)
		img = dst
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil, fmt.Errorf("encoding png: %w", err)
	}
	return out.Bytes(), nil
}
