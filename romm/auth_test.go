package romm

import (
	"reflect"
	"testing"
)

func TestMissingSyncScopes(t *testing.T) {
	tests := []struct {
		name string
		have []string
		want []string
	}{
		{
			name: "all present",
			have: []string{"assets.read", "assets.write", "devices.read", "devices.write", "extra"},
			want: nil,
		},
		{
			name: "missing device scopes",
			have: []string{"assets.read", "assets.write"},
			want: []string{"devices.read", "devices.write"},
		},
		{
			name: "empty",
			have: nil,
			want: []string{"assets.read", "assets.write", "devices.read", "devices.write"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MissingSyncScopes(tt.have); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MissingSyncScopes(%v) = %v, want %v", tt.have, got, tt.want)
			}
		})
	}
}
