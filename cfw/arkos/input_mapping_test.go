package arkos

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// Device tree compatible strings are NUL-separated, as read from /sys/firmware/devicetree/base/compatible.
func TestDetectDeviceFromCompatible(t *testing.T) {
	tests := []struct {
		name       string
		compatible string
		want       Device
	}{
		{"R40 Pro Max", "rockchip,rk3326-odroidgo3-linux\x00rockchip,rk3326\x00", DeviceR40ProMax},
		{"R36S clone", "rockchip,rk3326-r36s-linux\x00rockchip,rk3326\x00", DeviceR36SClone},
		{"unknown RK3326", "rockchip,rk3326-rg351p-linux\x00rockchip,rk3326\x00", DeviceGeneric},
		{"empty", "", DeviceGeneric},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := detectDeviceFromCompatible(tt.compatible); got != tt.want {
				t.Errorf("detectDeviceFromCompatible(%q) = %q, want %q", tt.compatible, got, tt.want)
			}
		})
	}
}

// The R36S clone reports Start as BTN_TRIGGER_HAPPY2, which only reaches Grout through the joystick path.
func TestGetInputMappingBytesForDevice_R36SCloneGetsAMapping(t *testing.T) {
	t.Chdir(t.TempDir())

	data, err := GetInputMappingBytesForDevice(DeviceR36SClone)
	if err != nil {
		t.Fatal(err)
	}
	want, err := embeddedInputMappings.ReadFile("input_mappings/r40-pro-max.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, want) {
		t.Errorf("R36S clone mapping = %s, want the R40 Pro Max mapping", data)
	}
}

func TestGetInputMappingBytesForDevice_GenericWithoutOverride(t *testing.T) {
	t.Chdir(t.TempDir())

	data, err := GetInputMappingBytesForDevice(DeviceGeneric)
	if err != nil {
		t.Fatal(err)
	}
	if data != nil {
		t.Errorf("generic mapping = %s, want nil", data)
	}
}

func TestGetInputMappingBytesForDevice_GenericUsesOverride(t *testing.T) {
	t.Chdir(t.TempDir())

	override := []byte(`{"joystick_button_map": {"13": 13}}`)
	path := filepath.Join("overrides", "cfw", "arkos", genericMappingFile)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, override, 0o644); err != nil {
		t.Fatal(err)
	}

	data, err := GetInputMappingBytesForDevice(DeviceGeneric)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, override) {
		t.Errorf("generic mapping = %s, want the override", data)
	}
}
