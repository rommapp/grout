package muos

import (
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

//go:embed input_mappings/*.json
var embeddedInputMappings embed.FS

// Device represents the detected device type on muOS
type Device string

const (
	DeviceAnbernic Device = "anbernic"
	DeviceTrimui   Device = "trimui"
)

// DetectDevice detects the device type when running on muOS by checking input devices.
// Returns DeviceTrimui if "TRIMUI" is found in /proc/bus/input/devices, otherwise DeviceAnbernic.
func DetectDevice() Device {
	logger := gaba.GetLogger()
	logger.Info("Detecting muOS device type...")

	cmd := exec.Command("sh", "-c", "cat /proc/bus/input/devices | grep TRIMUI")
	output, err := cmd.Output()

	if err != nil || len(output) == 0 {
		return DeviceAnbernic
	}

	return DeviceTrimui
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
	case DeviceAnbernic:
		filename = "input_mappings/anbernic.json"
	case DeviceTrimui:
		filename = "input_mappings/trimui.json"
	default:
		filename = "input_mappings/anbernic.json"
	}

	overridePath := filepath.Join("overrides", "cfw", "muos", filename)
	data, err := os.ReadFile(overridePath)
	if err != nil {
		data, err = embeddedInputMappings.ReadFile(filename)
		if err != nil {
			return nil, fmt.Errorf("failed to read embedded input mapping %s: %w", filename, err)
		}
	}

	return data, nil
}
