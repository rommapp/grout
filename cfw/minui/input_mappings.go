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

const DeviceType = "MINUI_DEVICE"

// devicetreeCompatiblePath is the path to the device-tree compatible string. It is a
// package-level variable so tests can override it with a temp file.
var devicetreeCompatiblePath = "/sys/firmware/devicetree/base/compatible"

// devicetreeModelPath is the path to the device-tree model string. Used to distinguish
// the TrimUI Brick (1024×768 IPS) from the TrimUI Smart Pro (1280×720), both
// of which report MINUI_DEVICE=tg5040.
var devicetreeModelPath = "/sys/firmware/devicetree/base/model"

//go:embed input_mappings/*.json
var embeddedInputMappings embed.FS

type Device string

const (
	DeviceMiyoo       Device = "miyoo"
	DeviceMiyooFlip   Device = "miyooflip"
	DeviceAnbernic    Device = "anbernic"
	DeviceZero28      Device = "zero28"
	DeviceTrimui      Device = "trimui"
	DeviceTrimuiBrick Device = "trimui-brick"
	DeviceGeneric     Device = "generic"
)

func detectDeviceByEnv() Device {
	logger := gaba.GetLogger()
	logger.Debug("Detecting MinUI device type", "env", DeviceType)
	deviceType := os.Getenv(DeviceType)

	switch deviceType {
	case "tg5040":
		return DeviceTrimui
	case "zero28":
		return DeviceZero28
	case "my355":
		return DeviceMiyooFlip
	default:
		logger.Warn("Unknown MinUI device type", "value", deviceType)
		return DeviceGeneric
	}
}

func DetectDevice() Device {
	logger := gaba.GetLogger()
	logger.Debug("Detecting MinUI device type", "arch", runtime.GOARCH)

	if runtime.GOARCH == "arm" {
		return DeviceMiyoo
	}

	minuiDeviceType := detectDeviceByEnv()
	// Anbernic devices use the Allwinner H616 SoC
	compatible, err := os.ReadFile(devicetreeCompatiblePath)
	if err == nil && strings.Contains(string(compatible), "allwinner,h616") {
		return DeviceAnbernic
	}
	if err == nil && strings.Contains(string(compatible), "allwinner,a133") && minuiDeviceType == DeviceZero28 {
		return DeviceZero28
	}

	switch minuiDeviceType {
	case DeviceMiyooFlip:
		return minuiDeviceType
	case DeviceTrimui:
		// Both the TrimUI Smart Pro and TrimUI Brick report tg5040. The Smart Pro has a
		// 1280×720 display while the Brick has a 1024×768 IPS display. We
		// distinguish them via the device-tree model string.
		model, modelErr := os.ReadFile(devicetreeModelPath)
		if modelErr == nil && strings.Contains(strings.ToLower(string(model)), "brick") {
			return DeviceTrimuiBrick
		}
		return DeviceTrimui
	}

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
	case DeviceTrimui, DeviceTrimuiBrick:
		filename = "input_mappings/trimui.json"
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
