package rocknix

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed input_mappings/*.json
var embeddedInputMappings embed.FS

type Device string

const (
	DeviceAnbernicRGDS Device = "rgds"
	DeviceGeneric      Device = "generic"
)

func DetectDevice() Device {
	compatible, err := os.ReadFile("/sys/firmware/devicetree/base/compatible")
	if err == nil && strings.Contains(string(compatible), "anbernic,rg-ds") {
		return DeviceAnbernicRGDS
	}

	return DeviceGeneric
}

func GetInputMappingBytes() ([]byte, error) {
	device := DetectDevice()

	var filename string
	switch device {
	case DeviceAnbernicRGDS:
		filename = "input_mappings/rgds.json"
	default:
		return nil, nil
	}

	overridePath := filepath.Join("overrides", "cfw", "rocknix", filename)
	data, err := os.ReadFile(overridePath)
	if err != nil {
		data, err = embeddedInputMappings.ReadFile(filename)
		if err != nil {
			return nil, fmt.Errorf("failed to read embedded input mapping %s: %w", filename, err)
		}
	}

	return data, nil
}
