package cfw

import "testing"

// Only a firmware that lets one game change core has saves in folders per core,
// which is what makes a save in two folders something to settle.
func TestOnlyFirmwaresThatKeepSavesPerCoreTreatTwoFoldersAsADoubt(t *testing.T) {
	if !Lookup(Onion).KeepsSavesPerCore() {
		t.Error("Onion keeps saves per core")
	}
	for _, id := range []CFW{MinUI, NextUI, MuOS, Knulli, Spruce, Allium} {
		if Lookup(id).KeepsSavesPerCore() {
			t.Errorf("%s was treated as keeping saves per core", id)
		}
	}
}
