package firmware

import "testing"

func TestBiosRels(t *testing.T) {
	eq(t, BiosRels("DC_BOOT.BIN"), []string{"dc/dc_boot.bin"})
	eq(t, BiosRels("scph1001.bin"), []string{"scph1001.bin"})
	eq(t, BiosRels("program.rom"), []string{"SGB1.sfc/program.rom", "SGB2.sfc/program.rom"})
	eq(t, BiosRels("some/dir/scph1001.bin"), []string{"scph1001.bin"})
}

func TestBiosRelsTableShape(t *testing.T) {
	table, err := biosSubfolders()
	if err != nil {
		t.Fatal(err)
	}
	if len(table) == 0 {
		t.Fatal("empty table")
	}
	for k, rels := range table {
		if len(rels) == 0 {
			t.Fatalf("%s has no paths", k)
		}
	}
}
