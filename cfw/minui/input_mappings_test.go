package minui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// --- detectDeviceByEnv ---

func TestDetectDeviceByEnv(t *testing.T) {
	cases := []struct {
		name string
		env  string
		want Device
	}{
		{"tg5040 maps to Trimui", "tg5040", DeviceTrimui},
		{"zero28 maps to Zero28", "zero28", DeviceZero28},
		{"my355 maps to MiyooFlip", "my355", DeviceMiyooFlip},
		{"unknown value maps to Generic", "unknown-device", DeviceGeneric},
		{"empty value maps to Generic", "", DeviceGeneric},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(DeviceType, tc.env)
			if got := detectDeviceByEnv(); got != tc.want {
				t.Errorf("detectDeviceByEnv() = %q, want %q", got, tc.want)
			}
		})
	}
}

// --- DetectDevice ---

// withFakeDevicetree sets devicetreeCompatiblePath to a temp file containing the given
// content (or a non-existent path if content is empty) and restores it on cleanup.
func withFakeDevicetree(t *testing.T, content string) {
	t.Helper()
	original := devicetreeCompatiblePath
	t.Cleanup(func() { devicetreeCompatiblePath = original })

	if content == "" {
		devicetreeCompatiblePath = filepath.Join(t.TempDir(), "nonexistent")
		return
	}

	dir := t.TempDir()
	fakePath := filepath.Join(dir, "compatible")
	if err := os.WriteFile(fakePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write fake devicetree file: %v", err)
	}
	devicetreeCompatiblePath = fakePath
}

// DetectDevice returns DeviceMiyoo immediately when runtime.GOARCH == "arm" (32-bit),
// so on non-arm hosts (where tests run) we can exercise the env-var + devicetree logic.
func TestDetectDevice_NonArm(t *testing.T) {
	t.Run("tg5040 returns DeviceTrimui", func(t *testing.T) {
		t.Setenv(DeviceType, "tg5040")
		withFakeDevicetree(t, "")
		if got := DetectDevice(); got != DeviceTrimui {
			t.Errorf("DetectDevice() = %q, want %q", got, DeviceTrimui)
		}
	})

	t.Run("my355 returns DeviceMiyooFlip", func(t *testing.T) {
		t.Setenv(DeviceType, "my355")
		withFakeDevicetree(t, "")
		if got := DetectDevice(); got != DeviceMiyooFlip {
			t.Errorf("DetectDevice() = %q, want %q", got, DeviceMiyooFlip)
		}
	})

	t.Run("unknown device returns DeviceGeneric", func(t *testing.T) {
		t.Setenv(DeviceType, "unknown")
		withFakeDevicetree(t, "")
		if got := DetectDevice(); got != DeviceGeneric {
			t.Errorf("DetectDevice() = %q, want %q", got, DeviceGeneric)
		}
	})
}

func TestDetectDevice_AnbernicViaDevicetree(t *testing.T) {
	t.Setenv(DeviceType, "unknown")
	withFakeDevicetree(t, "allwinner,h616\x00anbernic,rg35xx")

	if got := DetectDevice(); got != DeviceAnbernic {
		t.Errorf("DetectDevice() = %q, want %q (h616 SoC should detect Anbernic)", got, DeviceAnbernic)
	}
}

func TestDetectDevice_Zero28ViaDevicetree(t *testing.T) {
	t.Setenv(DeviceType, "zero28")
	withFakeDevicetree(t, "allwinner,a133\x00magicx,zero28")

	if got := DetectDevice(); got != DeviceZero28 {
		t.Errorf("DetectDevice() = %q, want %q (a133 SoC + zero28 env should detect Zero28)", got, DeviceZero28)
	}
}

// Regression test for issue #252: on the TrimUI Smart Pro (tg5040, Allwinner A133),
// DetectDevice must return DeviceTrimui, not DeviceGeneric. The a133+zero28 check must
// NOT match because the env is tg5040, not zero28.
func TestDetectDevice_TrimuiSmartPro(t *testing.T) {
	t.Setenv(DeviceType, "tg5040")
	withFakeDevicetree(t, "allwinner,a133\x00trimui,tg5040")

	if got := DetectDevice(); got != DeviceTrimui {
		t.Errorf("DetectDevice() = %q, want %q (tg5040 should detect Trimui even with a133 SoC)", got, DeviceTrimui)
	}
}

// --- GetInputMappingBytes ---

// chdirTemp changes to a temp dir (so no override file is found) and restores the
// original working directory on cleanup.
func chdirTemp(t *testing.T) {
	t.Helper()
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to getwd: %v", err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(origWd) })
}

func TestGetInputMappingBytes_LoadsEmbeddedMapping(t *testing.T) {
	// devicetree is the fake compatible string content; empty means no devicetree file.
	cases := []struct {
		name       string
		env        string
		devicetree string
		wantKeys   []string // top-level keys that must be present; nil means expect nil data
	}{
		{
			name:       "trimui",
			env:        "tg5040",
			devicetree: "",
			wantKeys:   []string{"controller_button_map", "joystick_button_map"},
		},
		{
			name:       "anbernic",
			env:        "unknown",
			devicetree: "allwinner,h616\x00anbernic",
			wantKeys:   []string{"joystick_button_map", "joystick_hat_map"},
		},
		{
			name:       "zero28",
			env:        "zero28",
			devicetree: "allwinner,a133\x00magicx,zero28",
			wantKeys:   []string{"controller_button_map", "joystick_button_map"},
		},
		{
			name:       "miyoo_flip returns nil (standard SDL input)",
			env:        "my355",
			devicetree: "",
			wantKeys:   nil,
		},
		{
			name:       "generic returns nil (standard SDL input)",
			env:        "unknown",
			devicetree: "",
			wantKeys:   nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(DeviceType, tc.env)
			withFakeDevicetree(t, tc.devicetree)
			chdirTemp(t)

			data, err := GetInputMappingBytes()
			if err != nil {
				t.Fatalf("GetInputMappingBytes() error = %v", err)
			}

			if tc.wantKeys == nil {
				if data != nil {
					t.Errorf("GetInputMappingBytes() = %d bytes, want nil", len(data))
				}
				return
			}

			if data == nil {
				t.Fatalf("GetInputMappingBytes() = nil, want non-nil mapping data")
			}

			var mapping map[string]json.RawMessage
			if err := json.Unmarshal(data, &mapping); err != nil {
				t.Fatalf("failed to parse mapping JSON: %v", err)
			}
			for _, key := range tc.wantKeys {
				if _, ok := mapping[key]; !ok {
					t.Errorf("mapping JSON missing key %q", key)
				}
			}
		})
	}
}

// Verify the trimui.json embedded mapping has the correct structure matching the
// TrimUI Smart Pro's controller layout (issue #252).
func TestGetInputMappingBytes_TrimuiMappingContent(t *testing.T) {
	t.Setenv(DeviceType, "tg5040")
	withFakeDevicetree(t, "")
	chdirTemp(t)

	data, err := GetInputMappingBytes()
	if err != nil {
		t.Fatalf("GetInputMappingBytes() error = %v", err)
	}
	if data == nil {
		t.Fatalf("GetInputMappingBytes() = nil, want trimui mapping data")
	}

	var mapping struct {
		KeyboardMap         map[string]int `json:"keyboard_map"`
		ControllerButtonMap map[string]int `json:"controller_button_map"`
		ControllerHatMap    map[string]int `json:"controller_hat_map"`
		JoystickAxisMap     map[string]any `json:"joystick_axis_map"`
		JoystickButtonMap   map[string]int `json:"joystick_button_map"`
		JoystickHatMap      map[string]int `json:"joystick_hat_map"`
	}
	if err := json.Unmarshal(data, &mapping); err != nil {
		t.Fatalf("failed to parse trimui mapping JSON: %v", err)
	}

	// The TrimUI Smart Pro maps 13 controller buttons (A, B, X, Y, L1, R1, Start,
	// Select, Menu, Up, Down, Left, Right) and 2 joystick buttons (L2, R2 as analog
	// triggers). No keyboard events — the device uses SDL controller/joystick events.
	if len(mapping.ControllerButtonMap) != 13 {
		t.Errorf("controller_button_map has %d entries, want 13", len(mapping.ControllerButtonMap))
	}
	if len(mapping.JoystickButtonMap) != 2 {
		t.Errorf("joystick_button_map has %d entries, want 2", len(mapping.JoystickButtonMap))
	}
	if len(mapping.KeyboardMap) != 0 {
		t.Errorf("keyboard_map has %d entries, want 0 (TrimUI uses controller events)", len(mapping.KeyboardMap))
	}
}
