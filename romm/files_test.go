package romm

import (
	"encoding/json"
	"testing"
)

// RomM files a file under the category its folder names. Only the ones a
// player would launch are versions; a manual or a soundtrack is not.
func TestRomFile_Playable(t *testing.T) {
	for category, want := range map[any]bool{
		nil: true, "": true, "game": true, "hack": true, "mod": true,
		"translation": true, "prototype": true, "demo": true,
		"manual": false, "walkthrough": false, "soundtrack": false,
		"screenshot": false, "cheat": false, "patch": false,
		"update": false, "dlc": false,
	} {
		if got := (RomFile{Category: category}).Playable(); got != want {
			t.Errorf("category %v: playable = %v, want %v", category, got, want)
		}
	}
}

// The versions offered are the playable files, in RomM's order.
func TestRom_PlayableFiles(t *testing.T) {
	var rom Rom
	body := `{"files":[
		{"id":1,"file_name":"Game (USA).gba","category":null},
		{"id":2,"file_name":"Manual.pdf","category":"manual"},
		{"id":3,"file_name":"Game (Europe).gba","category":null},
		{"id":4,"file_name":"Game Hack.gba","category":"hack"}]}`
	if err := json.Unmarshal([]byte(body), &rom); err != nil {
		t.Fatal(err)
	}

	var ids []int
	for _, f := range rom.PlayableFiles() {
		ids = append(ids, f.ID)
	}
	if len(ids) != 3 || ids[0] != 1 || ids[1] != 3 || ids[2] != 4 {
		t.Errorf("playable files = %v, want [1 3 4]", ids)
	}
}

// A game whose files all look like extras still has something to download.
func TestRom_PlayableFilesFallsBackToAll(t *testing.T) {
	rom := Rom{Files: []RomFile{{ID: 1, Category: "manual"}, {ID: 2, Category: "patch"}}}
	if got := rom.PlayableFiles(); len(got) != 2 {
		t.Errorf("got %d files, want all 2 when none look playable", len(got))
	}
}

// RomM lists what it found inside an archive, with hashes grout has no use for.
// Older servers send no list, or null.
func TestRomFile_ArchiveMemberNames(t *testing.T) {
	var rom Rom
	body := `{"files":[
		{"id":1,"file_name":"Mario.zip","archive_members":[
			{"name":"Super Mario Bros (USA).nes","size":40976,"crc_hash":"3337ec46","md5_hash":"811b027e","sha1_hash":"ea343f4e"}
		]},
		{"id":2,"file_name":"Zelda.zip","archive_members":null},
		{"id":3,"file_name":"Metroid.nes"}
	]}`
	if err := json.Unmarshal([]byte(body), &rom); err != nil {
		t.Fatal(err)
	}

	got := rom.Files[0].ArchiveMemberNames()
	if len(got) != 1 || got[0] != "Super Mario Bros (USA).nes" {
		t.Errorf("members = %v, want the one file inside the archive", got)
	}
	for _, file := range rom.Files[1:] {
		if got := file.ArchiveMemberNames(); got != nil {
			t.Errorf("%s: members = %v, want none", file.FileName, got)
		}
	}
}
