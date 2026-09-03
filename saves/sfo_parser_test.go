package saves

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSFO_BustAMove(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "testdata", "PARAM.SFO"))
	if err != nil {
		t.Skip("testdata/PARAM.SFO not found")
	}

	params, err := ParseSFO(data)
	if err != nil {
		t.Fatalf("ParseSFO failed: %v", err)
	}

	if params["TITLE"] != "BUST A MOVE DELUXE" {
		t.Errorf("TITLE = %q, want 'BUST A MOVE DELUXE'", params["TITLE"])
	}
	if params["SAVEDATA_DIRECTORY"] != "ULUS100570000" {
		t.Errorf("SAVEDATA_DIRECTORY = %q, want 'ULUS100570000'", params["SAVEDATA_DIRECTORY"])
	}
}

func TestParseSFO_FieldCommander(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "testdata", "PARAM-FieldCommander.SFO"))
	if err != nil {
		t.Skip("testdata/PARAM-FieldCommander.SFO not found")
	}

	params, err := ParseSFO(data)
	if err != nil {
		t.Fatalf("ParseSFO failed: %v", err)
	}

	if params["TITLE"] != "Field Commander™" {
		t.Errorf("TITLE = %q, want 'Field Commander™'", params["TITLE"])
	}
	if params["SAVEDATA_DIRECTORY"] != "ULUS10088010000" {
		t.Errorf("SAVEDATA_DIRECTORY = %q, want 'ULUS10088010000'", params["SAVEDATA_DIRECTORY"])
	}
	if params["SAVEDATA_DETAIL"] != "Campaign Progress: 0%" {
		t.Errorf("SAVEDATA_DETAIL = %q, want 'Campaign Progress: 0%%'", params["SAVEDATA_DETAIL"])
	}
}

func TestParseSFO_InvalidMagic(t *testing.T) {
	_, err := ParseSFO([]byte("not a valid SFO file at all"))
	if err == nil {
		t.Error("expected error for invalid magic")
	}
}

func TestParseSFO_TooShort(t *testing.T) {
	_, err := ParseSFO([]byte{0, 1, 2})
	if err == nil {
		t.Error("expected error for short data")
	}
}

func TestReadPSPSaveTitle_BustAMove(t *testing.T) {
	sfoData, err := os.ReadFile(filepath.Join("..", "testdata", "PARAM.SFO"))
	if err != nil {
		t.Skip("testdata/PARAM.SFO not found")
	}

	dir := t.TempDir()
	saveDir := filepath.Join(dir, "ULUS100570000")
	os.MkdirAll(saveDir, 0755)
	os.WriteFile(filepath.Join(saveDir, "PARAM.SFO"), sfoData, 0644)

	title, ok := ReadPSPSaveTitle(saveDir)
	if !ok {
		t.Fatal("expected to find title")
	}
	if title != "BUST A MOVE DELUXE" {
		t.Errorf("title = %q, want 'BUST A MOVE DELUXE'", title)
	}
}

func TestReadPSPSaveTitle_FieldCommander(t *testing.T) {
	sfoData, err := os.ReadFile(filepath.Join("..", "testdata", "PARAM-FieldCommander.SFO"))
	if err != nil {
		t.Skip("testdata/PARAM-FieldCommander.SFO not found")
	}

	dir := t.TempDir()
	saveDir := filepath.Join(dir, "ULUS10088010000")
	os.MkdirAll(saveDir, 0755)
	os.WriteFile(filepath.Join(saveDir, "PARAM.SFO"), sfoData, 0644)

	title, ok := ReadPSPSaveTitle(saveDir)
	if !ok {
		t.Fatal("expected to find title")
	}
	if title != "Field Commander" {
		t.Errorf("title = %q, want 'Field Commander'", title)
	}
}

func TestReadPSPSaveTitle_NoSFO(t *testing.T) {
	dir := t.TempDir()
	_, ok := ReadPSPSaveTitle(dir)
	if ok {
		t.Error("expected not found when PARAM.SFO is missing")
	}
}
