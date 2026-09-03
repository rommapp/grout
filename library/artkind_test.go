package library

import (
	"encoding/json"
	"testing"
)

// These strings are written to config.json as art_kind. Changing one silently
// resets the setting for every existing install, so this test pins them.
func TestArtKind_PersistedValues(t *testing.T) {
	want := map[ArtKind]string{
		ArtKindNone:       "",
		ArtKindDefault:    "Default",
		ArtKindBox2D:      "Box2D",
		ArtKindBox3D:      "Box3D",
		ArtKindMixImage:   "Miximage",
		ArtKindMarquee:    "Marquee",
		ArtKindLogo:       "Logo",
		ArtKindTitle:      "Title",
		ArtKindScreenshot: "Screenshot",
		ArtKindVideo:      "Video",
	}

	for kind, s := range want {
		if string(kind) != s {
			t.Errorf("ArtKind %q should serialise as %q", string(kind), s)
		}
	}
}

// A config written before this type moved packages must still load.
func TestArtKind_RoundTripsThroughJSON(t *testing.T) {
	var settings struct {
		ArtKind ArtKind `json:"art_kind,omitempty"`
	}

	if err := json.Unmarshal([]byte(`{"art_kind":"Default"}`), &settings); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if settings.ArtKind != ArtKindDefault {
		t.Errorf("ArtKind = %q, want %q", settings.ArtKind, ArtKindDefault)
	}

	out, err := json.Marshal(settings)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(out) != `{"art_kind":"Default"}` {
		t.Errorf("marshalled to %s, want {\"art_kind\":\"Default\"}", out)
	}

	// The empty kind is omitted rather than written as "".
	settings.ArtKind = ArtKindNone
	if out, err = json.Marshal(settings); err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(out) != `{}` {
		t.Errorf("marshalled to %s, want {} for the none kind", out)
	}
}

func TestArtKind_Enabled(t *testing.T) {
	if ArtKindNone.Enabled() {
		t.Error("the none kind must not be enabled")
	}
	for _, k := range []ArtKind{ArtKindDefault, ArtKindBox2D, ArtKindMarquee, ArtKindVideo} {
		if !k.Enabled() {
			t.Errorf("%q should be enabled", k)
		}
	}
}
