package minui

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

//go:embed input_mappings/*.json
var embeddedInputMappings embed.FS

type Device string

const (
	DeviceMiyoo    Device = "miyoo"
	DeviceAnbernic Device = "anbernic"
	DeviceZero28   Device = "zero28"
	DeviceGeneric  Device = "generic"
)

func DetectDevice() Device {
	logger := gaba.GetLogger()
	logger.Debug("Detecting MinUI device type", "arch", runtime.GOARCH)

	if runtime.GOARCH == "arm" {
		return DeviceMiyoo
	}

	// Anbernic devices use the Allwinner H616 SoC
	compatible, err := os.ReadFile("/sys/firmware/devicetree/base/compatible")
	if err == nil && strings.Contains(string(compatible), "allwinner,h616") {
		return DeviceAnbernic
	}
	if err == nil && strings.Contains(string(compatible), "allwinner,a133") {
		return DeviceZero28
	}

	// TODO discover if Miyoo Flip and Others are fine with standard config
	// TrimUI (a133p), Miyoo Flip (potentially idk don't own one), and others use standard SDL controller input
	return DeviceGeneric
}

func GetInputMappingBytes() ([]byte, error) {
	device := DetectDevice()

	var filename string
	switch device {
	case DeviceMiyoo:
		filename = "input_mappings/miyoo.json"
	case DeviceAnbernic:
		filename = "input_mappings/anbernic.json"
	case DeviceZero28:
		filename = "input_mappings/zero28.json"
	default:
		return nil, nil
	}

	overridePath := filepath.Join("overrides", "cfw", "minui", filename)
	data, err := os.ReadFile(overridePath)
	if err != nil {
		data, err = embeddedInputMappings.ReadFile(filename)
		if err != nil {
			return nil, fmt.Errorf("failed to read embedded input mapping %s: %w", filename, err)
		}
	}

	return data, nil
}
