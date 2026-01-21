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

func CreateTempQRCode(content string, size int) (string, error) {
	qr, err := goqr.EncodeText(content, goqr.Low)
	if err != nil {
		return "", err
	}

	tempFile, err := os.CreateTemp("", "qrcode-*.png")
	if err != nil {
		return "", err
	}
	tempFile.Close()

	config := goqr.NewQrCodeImgConfig(size/10, 0)
	if err := qr.PNG(config, tempFile.Name()); err != nil {
		return "", err
	}

	return tempFile.Name(), nil
}

func ProcessArtImage(inputPath string) error {
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

	windowWidth := int(gabagool.GetWindow().GetWidth()) / 2
	windowHeight := int(gabagool.GetWindow().GetHeight()) / 2

	bounds := img.Bounds()
	imgWidth := bounds.Dx()
	imgHeight := bounds.Dy()

	var newWidth, newHeight int
	imgAspect := float64(imgWidth) / float64(imgHeight)
	windowAspect := float64(windowWidth) / float64(windowHeight)

	if imgAspect > windowAspect {
		newWidth = windowWidth
		newHeight = int(float64(windowWidth) / imgAspect)
	} else {
		newHeight = windowHeight
		newWidth = int(float64(windowHeight) * imgAspect)
	}

	var processedImg = img
	if newWidth != imgWidth || newHeight != imgHeight {
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
