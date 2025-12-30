package utils

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"github.com/yeqown/go-qrcode/v2"
	"github.com/yeqown/go-qrcode/writer/standard"
	"golang.org/x/image/draw"
)

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

	windowWidth := int(gaba.GetWindow().GetWidth()) / 2
	windowHeight := int(gaba.GetWindow().GetHeight()) / 2

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

func CreateTempQRCode(content string, size int) (string, error) {
	qr, err := qrcode.New(content)
	if err != nil {
		return "", err
	}

	tempFile, err := os.CreateTemp("", "qrcode-*.png")
	if err != nil {
		return "", err
	}
	defer tempFile.Close()

	w := standard.NewWithWriter(tempFile,
		standard.WithQRWidth(uint8(size/10)),
		standard.WithBgColor(color.Black),
		standard.WithFgColor(color.White),
		standard.WithBorderWidth(0),
	)

	if err := qr.Save(w); err != nil {
		return "", err
	}

	return tempFile.Name(), nil
}
