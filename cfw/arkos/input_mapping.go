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

// Device represents the detected device type on ArkOS / dArkOS
type Device string

const (
	DeviceR40ProMax Device = "R40ProMax"
	DeviceR36SClone Device = "R36SClone"
	DeviceGeneric   Device = "generic"
)

const genericMappingFile = "input_mappings/generic.json"

// DetectDevice detects the device type when running on ArkOS by reading the device tree compatible string.
// Returns DeviceGeneric when the device is not recognized.
func DetectDevice() Device {
	compatible, err := os.ReadFile("/sys/firmware/devicetree/base/compatible")
	if err != nil {
		return DeviceGeneric
	}
	return detectDeviceFromCompatible(string(compatible))
}

func detectDeviceFromCompatible(compatible string) Device {
	switch {
	case strings.Contains(compatible, "rk3326-odroidgo3-linux"):
		return DeviceR40ProMax
	case strings.Contains(compatible, "rk3326-r36s-linux"):
		return DeviceR36SClone
	default:
		return DeviceGeneric
	}
}

// GetInputMappingBytes returns the input mapping JSON for the detected ArkOS device
func GetInputMappingBytes() ([]byte, error) {
	device := DetectDevice()
	return GetInputMappingBytesForDevice(device)
}

// GetInputMappingBytesForDevice returns the input mapping JSON for a specific device.
// An override in overrides/cfw/arkos/ takes precedence over the embedded mapping. Generic devices
// have no embedded mapping and fall back to the default SDL mapping unless an override is provided.
func GetInputMappingBytesForDevice(device Device) ([]byte, error) {
	var filename string
	switch device {
	case DeviceR40ProMax, DeviceR36SClone:
		// Both use the odroidgo3-joypad driver, which reports Select/Start as BTN_TRIGGER_HAPPY1/2
		filename = "input_mappings/r40-pro-max.json"
	default:
		filename = genericMappingFile
	}

	overridePath := filepath.Join("overrides", "cfw", "arkos", filename)
	data, err := os.ReadFile(overridePath)
	if err == nil {
		return data, nil
	}

	if filename == genericMappingFile {
		return nil, nil
	}

	data, err = embeddedInputMappings.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded input mapping %s: %w", filename, err)
	}

	return data, nil
}
