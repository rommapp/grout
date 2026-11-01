package spruce

import (
	"embed"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

const DeviceType = "SPRUCE_DEVICE"

//go:embed input_mappings/*.json
var embeddedInputMappings embed.FS

type Device string

const (
	DeviceA30       Device = "A30"
	DeviceMiyooMini Device = "MIYOOMINI"
	DeviceMiyooFlip Device = "MIYOOFLIP"
	DeviceTrimui    Device = "TRIMUI"
	DevicePixel     Device = "PIXEL"
	DeviceH700      Device = "H700"
	DeviceRGB30     Device = "RGB30"
	DeviceGeneric   Device = "GENERIC"
)

// DetectDevice detects the device type when running on Spruce by checking environment variables.
func DetectDevice() Device {
	logger := slog.Default()
	logger.Debug("Detecting Spruce device type", "env", DeviceType)

	switch os.Getenv(DeviceType) {
	case "A30":
		return DeviceA30
	case "MIYOOMINI":
		return DeviceMiyooMini
	case "MIYOOFLIP":
		return DeviceMiyooFlip
	case "TRIMUI":
		return DeviceTrimui
	case "PIXEL":
		return DevicePixel
	case "H700":
		return DeviceH700
	case "RGB30":
		return DeviceRGB30
	default:
		logger.Warn("Unrecognized Spruce device, using generic SDL input", "device", os.Getenv(DeviceType), "platform", os.Getenv("PLATFORM"))
		return DeviceGeneric
	}
}

// GetInputMappingBytes returns the embedded input mapping JSON for the detected Spruce device.
func GetInputMappingBytes() ([]byte, error) {
	device := DetectDevice()
	return GetInputMappingBytesForDevice(device)
}

// GetInputMappingBytesForDevice returns the embedded input mapping JSON for a specific device.
func GetInputMappingBytesForDevice(device Device) ([]byte, error) {
	var filename string
	switch device {
	case DeviceMiyooMini:
		filename = "input_mappings/miyoo.json"
	case DeviceA30:
		filename = "input_mappings/a30.json"
	case DevicePixel:
		filename = "input_mappings/gkd_pixel_2.json"
	case DeviceH700:
		filename = "input_mappings/h700.json"
	case DeviceRGB30:
		filename = "input_mappings/rgb30.json"
	default:
		// TrimUI, Miyoo Flip, and generic devices use standard SDL controller input
		return nil, nil
	}

	overridePath := filepath.Join("overrides", "cfw", "spruce", filename)
	data, err := os.ReadFile(overridePath)
	if err != nil {
		data, err = embeddedInputMappings.ReadFile(filename)
		if err != nil {
			return nil, fmt.Errorf("failed to read embedded input mapping %s: %w", filename, err)
		}
	}

	return data, nil
}
