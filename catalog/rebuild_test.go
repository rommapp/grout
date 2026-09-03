package catalog

import "testing"

// Artwork is refetched lazily as screens ask for it, so clearing it alone must
// not trigger a full metadata refetch of the whole library.
func TestCacheScope_ClearsMetadata(t *testing.T) {
	tests := []struct {
		scope CacheScope
		want  bool
	}{
		{ScopeMetadata, true},
		{ScopeAll, true},
		{ScopeArtwork, false},
	}
	for _, tt := range tests {
		if got := tt.scope.ClearsMetadata(); got != tt.want {
			t.Errorf("scope %v ClearsMetadata = %v, want %v", tt.scope, got, tt.want)
		}
	}
}

func TestCacheScope_ClearsArtwork(t *testing.T) {
	tests := []struct {
		scope CacheScope
		want  bool
	}{
		{ScopeArtwork, true},
		{ScopeAll, true},
		{ScopeMetadata, false},
	}
	for _, tt := range tests {
		if got := tt.scope.clearsArtwork(); got != tt.want {
			t.Errorf("scope %v clearsArtwork = %v, want %v", tt.scope, got, tt.want)
		}
	}
}

// ScopeAll must clear both, or "All" quietly means "one of them".
func TestCacheScope_AllClearsEverything(t *testing.T) {
	if !ScopeAll.ClearsMetadata() || !ScopeAll.clearsArtwork() {
		t.Error("ScopeAll must clear metadata and artwork")
	}
}

// The zero value is what an unset or mis-typed selection yields. It has to be a
// safe choice rather than an accidental full wipe of artwork the user did not
// ask to lose.
func TestCacheScope_ZeroValueIsMetadataOnly(t *testing.T) {
	var scope CacheScope
	if scope != ScopeMetadata {
		t.Errorf("zero CacheScope = %v, want ScopeMetadata", scope)
	}
	if scope.clearsArtwork() {
		t.Error("the zero scope must not clear artwork")
	}
}
