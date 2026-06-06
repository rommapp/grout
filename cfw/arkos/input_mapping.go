package arkos

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed input_mappings/*.json
var embeddedInputMappings embed.FS

// Device represents the detected device type on muOS
type Device string

const (
	DeviceR40ProMax Device = "R40ProMax"
	DeviceGeneric   Device = "generic"
)

// DetectDevice detects the device type when running on muOS by checking input devices.
// Returns DeviceTrimui if "TRIMUI" is found in /proc/bus/input/devices, otherwise DeviceAnbernic.
func DetectDevice() Device {
	compatible, err := os.ReadFile("/sys/firmware/devicetree/base/compatible")
	if err == nil && strings.Contains(string(compatible), "rk3326-odroidgo3-linux") {
		return DeviceR40ProMax
	}

	return DeviceGeneric
}

// GetInputMappingBytes returns the embedded input mapping JSON for the detected muOS device
func GetInputMappingBytes() ([]byte, error) {
	device := DetectDevice()
	return GetInputMappingBytesForDevice(device)
}

// GetInputMappingBytesForDevice returns the embedded input mapping JSON for a specific device
func GetInputMappingBytesForDevice(device Device) ([]byte, error) {
	var filename string
	switch device {
	case DeviceR40ProMax:
		filename = "input_mappings/r40-pro-max.json"
	default:
		return nil, nil
	}

	overridePath := filepath.Join("overrides", "cfw", "arkos", filename)
	data, err := os.ReadFile(overridePath)
	if err != nil {
		data, err = embeddedInputMappings.ReadFile(filename)
		if err != nil {
			return nil, fmt.Errorf("failed to read embedded input mapping %s: %w", filename, err)
		}
	}

	return data, nil
}
