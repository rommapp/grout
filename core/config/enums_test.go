package config

import (
	"reflect"
	"testing"
)

func TestArtKindLists(t *testing.T) {
	if got, want := CoverArtKinds(), []ArtKind{ArtKindDefault, ArtKindBox2D, ArtKindBox3D, ArtKindMixImage, ArtKindMixImageV2}; !reflect.DeepEqual(got, want) {
		t.Fatalf("cover = %v", got)
	}
	if got, want := SplashArtKinds(), []ArtKind{ArtKindNone, ArtKindMarquee, ArtKindTitle}; !reflect.DeepEqual(got, want) {
		t.Fatalf("splash = %v", got)
	}
	if got, want := MarqueeArtKinds(), []ArtKind{ArtKindNone, ArtKindMarquee, ArtKindLogo}; !reflect.DeepEqual(got, want) {
		t.Fatalf("marquee = %v", got)
	}
}
