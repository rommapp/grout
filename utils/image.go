package utils

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"

	gaba "github.com/UncleJunVIP/gabagool/v2/pkg/gabagool"
	"github.com/skip2/go-qrcode"
	"golang.org/x/image/draw"
)

func ProcessArtImage(inputPath string) error {
	logger := gaba.GetLogger()

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

	logger.Debug("Detected image format", "format", format, "path", inputPath)

	windowWidth := int(gaba.GetWindow().GetWidth()) / 2
	windowHeight := int(gaba.GetWindow().GetHeight()) / 2

	bounds := img.Bounds()
	imgWidth := bounds.Dx()
	imgHeight := bounds.Dy()

	var newWidth, newHeight int
	imgAspect := float64(imgWidth) / float64(imgHeight)
	windowAspect := float64(windowWidth) / float64(windowHeight)

	if imgAspect > windowAspect {
		// Image is wider than window, fit to width
		newWidth = windowWidth
		newHeight = int(float64(windowWidth) / imgAspect)
	} else {
		// Image is taller than window, fit to height
		newHeight = windowHeight
		newWidth = int(float64(windowHeight) * imgAspect)
	}

	// Only resize if dimensions changed
	var processedImg image.Image = img
	if newWidth != imgWidth || newHeight != imgHeight {
		logger.Debug("Resizing image", "from", fmt.Sprintf("%dx%d", imgWidth, imgHeight), "to", fmt.Sprintf("%dx%d", newWidth, newHeight))

		// Create a new image with the target dimensions
		dst := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))

		// Use high-quality bilinear scaling
		draw.BiLinear.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Over, nil)
		processedImg = dst
	}

	// If the original format is not PNG, or if we resized, save as PNG
	if format != "png" || processedImg != img {
		logger.Debug("Converting/saving image as PNG", "original_format", format)

		// Create output file
		outputFile, err := os.Create(inputPath)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer outputFile.Close()

		// Encode as PNG
		if err := png.Encode(outputFile, processedImg); err != nil {
			return fmt.Errorf("failed to encode PNG: %w", err)
		}
	}

	return nil
}

func CreateTempQRCode(content string, size int) (string, error) {
	qr, err := qrcode.New(content, qrcode.Medium)

	if err != nil {
		return "", err
	}

	qr.BackgroundColor = color.Black
	qr.ForegroundColor = color.White
	qr.DisableBorder = true

	tempFile, err := os.CreateTemp("", "qrcode-*")

	err = qr.Write(size, tempFile)

	if err != nil {
		return "", err
	}
	defer tempFile.Close()

	return tempFile.Name(), err
}
