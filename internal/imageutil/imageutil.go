package imageutil

import (
	"fmt"
	"image"
	_ "image/gif"  // Register GIF decoder
	_ "image/jpeg" // Register JPEG decoder
	"image/png"
	"os"

	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	goqr "github.com/piglig/go-qr"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // Register WebP decoder
)

// qrQuietZone is the QR spec's minimum light-margin, in modules, around the
// symbol. Without it scanners struggle to lock onto the code.
const qrQuietZone = 4

func CreateTempQRCode(content string, size int) (string, error) {
	qr, err := goqr.EncodeText(content, goqr.Medium)
	if err != nil {
		return "", err
	}

	tempFile, err := os.CreateTemp("", "qrcode-*.png")
	if err != nil {
		return "", err
	}
	tempFile.Close()

	config := goqr.NewQrCodeImgConfig(size/10, qrQuietZone)
	if err := qr.PNG(config, tempFile.Name()); err != nil {
		return "", err
	}

	return tempFile.Name(), nil
}

// FitDimensions returns the size to draw an imgW x imgH image at to fill a
// maxW x maxH box, keeping its aspect ratio.
//
// It scales both ways: an image smaller than the box is enlarged, so a small
// cover is written to disk larger than it arrived. Degenerate inputs return the
// original size.
func FitDimensions(imgW, imgH, maxW, maxH int) (int, int) {
	if imgW <= 0 || imgH <= 0 || maxW <= 0 || maxH <= 0 {
		return imgW, imgH
	}

	imgAspect := float64(imgW) / float64(imgH)
	boxAspect := float64(maxW) / float64(maxH)

	if imgAspect > boxAspect {
		return maxW, atLeastOne(int(float64(maxW) / imgAspect))
	}
	return atLeastOne(int(float64(maxH) * imgAspect)), maxH
}

// atLeastOne stops an extreme aspect ratio rounding a side to zero, which
// encodes as a corrupt PNG.
func atLeastOne(n int) int {
	if n < 1 {
		return 1
	}
	return n
}

// ProcessArtImage normalises the image at inputPath to a PNG sized for the
// display.
func ProcessArtImage(inputPath string) error {
	window := gabagool.GetWindow()
	return ProcessArtImageTo(inputPath, int(window.GetWidth())/2, int(window.GetHeight())/2)
}

// ProcessArtImageTo is ProcessArtImage with the box supplied, so it can run
// without a display.
func ProcessArtImageTo(inputPath string, maxW, maxH int) error {
	inputFile, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open image: %w", err)
	}
	defer inputFile.Close()

	img, format, err := image.Decode(inputFile)
	if err != nil {
		return fmt.Errorf("failed to decode image: %w", err)
	}
	inputFile.Close()

	bounds := img.Bounds()
	newWidth, newHeight := FitDimensions(bounds.Dx(), bounds.Dy(), maxW, maxH)

	var processedImg = img
	if newWidth != bounds.Dx() || newHeight != bounds.Dy() {
		dst := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))

		draw.BiLinear.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Over, nil)
		processedImg = dst
	}

	if format != "png" || processedImg != img {
		outputFile, err := os.Create(inputPath)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer outputFile.Close()

		if err := png.Encode(outputFile, processedImg); err != nil {
			return fmt.Errorf("failed to encode PNG: %w", err)
		}
	}

	return nil
}
