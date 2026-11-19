package muos

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

const sdlMapBefore = "19000000010000000100000000010000,muOS-Keys,a:b3,b:b4,x:b6,y:b5,leftshoulder:b7,rightshoulder:b8,lefttrigger:b12,righttrigger:b13,guide:b11,start:b10,back:b9,dpup:h0.1,dpleft:h0.8,dpright:h0.2,dpdown:h0.4,volumedown:b1,volumeup:b2,platform:Linux,"

const sdlMapNow = "060000006d750000070700006f730000,muOS-Keys,a:b2,b:b3,x:b4,y:b5,leftshoulder:b6,rightshoulder:b7,lefttrigger:b8,righttrigger:b9,guide:b12,start:b11,back:b10,dpup:h0.1,dpleft:h0.8,dpright:h0.2,dpdown:h0.4,volumedown:b0,volumeup:b1,platform:Linux,"

type buttonMaps struct {
	Buttons map[string]int `json:"joystick_button_map"`
	Hats    map[string]int `json:"joystick_hat_map"`
}

func decode(t *testing.T, data []byte) buttonMaps {
	t.Helper()
	var maps buttonMaps
	if err := json.Unmarshal(data, &maps); err != nil {
		t.Fatal(err)
	}
	return maps
}

func embeddedAnbernic(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("input/anbernic.json")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestMappingFromSDLMapMatchesTheEmbeddedOne(t *testing.T) {
	built, ok := mappingFromSDLMap(sdlMapBefore)
	if !ok {
		t.Fatal("the old muOS line did not parse")
	}
	if got, want := decode(t, built), decode(t, embeddedAnbernic(t)); !reflect.DeepEqual(got, want) {
		t.Errorf("built %+v\nwant %+v", got, want)
	}
}

func TestMappingFromSDLMapFollowsTheNewNumbering(t *testing.T) {
	built, ok := mappingFromSDLMap(sdlMapNow)
	if !ok {
		t.Fatal("the new muOS line did not parse")
	}
	want := map[string]int{
		"2": buttonA, "3": buttonB, "4": buttonX, "5": buttonY,
		"6": buttonL1, "7": buttonR1, "8": buttonL2, "9": buttonR2,
		"10": buttonSelect, "11": buttonStart, "12": buttonMenu,
	}
	if got := decode(t, built).Buttons; !reflect.DeepEqual(got, want) {
		t.Errorf("buttons = %v, want %v", got, want)
	}
	wantHats := map[string]int{"1": buttonUp, "2": buttonRight, "4": buttonDown, "8": buttonLeft}
	if got := decode(t, built).Hats; !reflect.DeepEqual(got, wantHats) {
		t.Errorf("hats = %v, want %v", got, wantHats)
	}
}

func TestMappingFromSDLMapRejectsWhatIsNotAMapping(t *testing.T) {
	for _, line := range []string{"", "not,a,mapping", "guid,name,platform:Linux"} {
		if _, ok := mappingFromSDLMap(line); ok {
			t.Errorf("%q was accepted", line)
		}
	}
}

func TestSDLMapInputPrefersMuOSFile(t *testing.T) {
	h := emptyHost()
	data, err := SDLMapInput(h)
	if err != nil {
		t.Fatal(err)
	}
	if got := decode(t, data).Buttons["3"]; got != buttonA {
		t.Errorf("without muOS's file, 3 = %d, want the embedded A", got)
	}

	put(h, sdlMapFile, sdlMapNow+"\n")
	data, err = SDLMapInput(h)
	if err != nil {
		t.Fatal(err)
	}
	if got := decode(t, data).Buttons["2"]; got != buttonA {
		t.Errorf("with muOS's file, 2 = %d, want A", got)
	}
}

func TestSDLMapInputFallsBackOnAnUnparseableFile(t *testing.T) {
	h := emptyHost()
	put(h, sdlMapFile, "\n")
	data, err := SDLMapInput(h)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := decode(t, data), decode(t, embeddedAnbernic(t)); !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want the embedded mapping", got)
	}
}

func TestSDLMapInputMockFixture(t *testing.T) {
	data, err := SDLMapInput(mockHost(t))
	if err != nil {
		t.Fatal(err)
	}
	if got := decode(t, data).Buttons; got["2"] != buttonA || got["3"] != buttonB || got["11"] != buttonStart {
		t.Errorf("buttons = %v, want the mock line's numbering", got)
	}
}
