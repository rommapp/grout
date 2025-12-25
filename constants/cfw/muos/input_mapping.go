package muos

import (
	"embed"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

//go:embed input_mappings/*.json
var embeddedInputMappings embed.FS

// Device represents the detected device type on muOS
type Device string

const (
	DeviceAnbernic       Device = "anbernic"
	DeviceTrimuiBrick    Device = "trimui_brick"
	DeviceTrimuiSmartPro Device = "trimui_smart_pro"
)

const trimuiMainUIPath = "/usr/trimui/bin/MainUI"

// DetectDevice detects the device type when running on muOS.
// Logic:
//   - If /usr/trimui/bin/MainUI doesn't exist → Anbernic
//   - If it exists and `strings MainUI | grep ^Trimui` returns "Trimui Brick" → Brick
//   - Otherwise → Smart Pro
func DetectDevice() Device {
	if _, err := os.Stat(trimuiMainUIPath); os.IsNotExist(err) {
		return DeviceAnbernic
	}

	// File exists, check if it's a Trimui Brick
	cmd := exec.Command("sh", "-c", fmt.Sprintf("strings %s | grep ^Trimui", trimuiMainUIPath))
	output, err := cmd.Output()
	if err != nil {
		// If strings/grep fails, default to Smart Pro
		return DeviceTrimuiSmartPro
	}

	trimmedOutput := strings.TrimSpace(string(output))
	if trimmedOutput == "Trimui Brick" {
		return DeviceTrimuiBrick
	}

	return DeviceTrimuiSmartPro
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
	case DeviceTrimuiBrick:
		filename = "input_mappings/trimui_brick.json"
	case DeviceTrimuiSmartPro:
		filename = "input_mappings/trimui_smart_pro.json"
	default:
		filename = "input_mappings/anbernic.json"
	}

	data, err := embeddedInputMappings.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded input mapping %s: %w", filename, err)
	}
	return data, nil
}
