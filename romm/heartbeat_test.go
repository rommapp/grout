package romm

import "testing"

func TestSupportsDeviceAuth(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{"5.0.0", true},
		{"5.1.2", true},
		{"v5.0.0", true},
		{"10.0.0", true},
		{"5.0.0-beta.1", true},
		{"4.9.1", false},
		{"4.10.0", false},
		{"", false},
		{"garbage", false},
	}
	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			var h HeartbeatResponse
			h.System.Version = tt.version
			if got := h.SupportsDeviceAuth(); got != tt.want {
				t.Errorf("SupportsDeviceAuth(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}
