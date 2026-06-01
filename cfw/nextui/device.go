package nextui

import (
	"os"
	"runtime"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

const DeviceType = "NEXTUI_DEVICE"

type Device string

const (
	DeviceMiyooFlip Device = "miyooflip"
	DeviceTrimui    Device = "trimui"
	DeviceGeneric   Device = "generic"
)

func detectDeviceByEnv() Device {
	logger := gaba.GetLogger()
	logger.Debug("Detecting NextUI device type", "env", DeviceType)
	deviceType := os.Getenv(DeviceType)

	switch deviceType {
	case "tg5040", "tg5050":
		return DeviceTrimui
	case "my355":
		return DeviceMiyooFlip
	default:
		logger.Warn("Unknown NextUI device type", "value", deviceType)
		return DeviceGeneric
	}
}

func DetectDevice() Device {
	logger := gaba.GetLogger()
	logger.Debug("Detecting NextUI device type", "arch", runtime.GOARCH)

	deviceType := detectDeviceByEnv()
	switch deviceType {
	case DeviceMiyooFlip:
		return deviceType
	}

	return DeviceGeneric
}
