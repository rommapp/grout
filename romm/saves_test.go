package romm

import (
	"strings"
	"testing"

	"github.com/sonh/qs"
)

// optimistic=false must be transmitted, not dropped by omitempty — the server defaults
// it to true, which would mark the device synced before the file is written.
func TestSaveContentQuery_OptimisticFalseIsEncoded(t *testing.T) {
	v, err := qs.NewEncoder().Values(SaveContentQuery{DeviceID: "dev-1", Optimistic: false})
	if err != nil {
		t.Fatal(err)
	}
	if got := v.Encode(); !strings.Contains(got, "optimistic=false") {
		t.Errorf("expected optimistic=false in query, got %q", got)
	}
}

func TestSaveContentQuery_OptimisticTrueIsEncoded(t *testing.T) {
	v, err := qs.NewEncoder().Values(SaveContentQuery{DeviceID: "dev-1", Optimistic: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := v.Encode(); !strings.Contains(got, "optimistic=true") {
		t.Errorf("expected optimistic=true in query, got %q", got)
	}
}
