package firmware

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"testing/fstest"
)

func TestEveryDefinitionValidates(t *testing.T) {
	known, err := KnownSlugs()
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range append(IDs(), "testfw") {
		d, err := Load(id)
		if err != nil {
			t.Errorf("%s: %v", id, err)
			continue
		}
		inputs, err := fs.Sub(embedded, id+"/input")
		if err != nil {
			inputs = fstest.MapFS{}
		}
		for _, e := range Validate(d, known, Hooks(), inputs) {
			t.Errorf("%s: %v", id, e)
		}
		for _, f := range d.Packaging.Files {
			if _, err := os.Stat(filepath.Join(id, "scripts", f)); err != nil {
				t.Errorf("%s: packaging file %s: %v", id, f, err)
			}
		}
	}
}

func TestLaunchFirmwaresAreDefined(t *testing.T) {
	want := []string{"allium", "knulli", "leaf", "muos", "nextui", "onion", "rocknix", "spruce"}
	got := IDs()
	for _, id := range want {
		if !slices.Contains(got, id) {
			t.Errorf("%s has no definition", id)
		}
	}
}
