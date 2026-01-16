package knulli

import (
	"grout/internal/fileutil"
	"grout/internal/gamelist"
	"os"
	"path/filepath"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

const (
	GroutKnulliGamelistPath = "tools/gamelist.xml"
	GroutEntryGameListName  = "Grout"
)

func FirstRunSetup(romDir string) {
	path := filepath.Join(romDir, "tools", "gamelist.xml")
	gl := gamelist.New()

	if fileutil.FileExists(path) {
		data, err := os.ReadFile(path)
		if err != nil {
			gaba.GetLogger().Debug("Error reading gamelist.xml file", "error", err)
			return
		}

		if err := gl.Parse(data); err != nil {
			gaba.GetLogger().Debug("Knulli gamelist.xml not found or can't be parsed, skipping grout entry check", "path", path, "error", err)
			return
		}
	}

	gl.AdddOrUpdateEntry(GroutEntryGameListName, map[string]string{
		gamelist.NameElement:    GroutEntryGameListName,
		gamelist.DescElement:    "Download games wirelessly from your RomM instance",
		gamelist.ImageElement:   "./Grout/logo.png",
		gamelist.PlayersElement: "1",
		gamelist.GenreElement:   "Utility",
		gamelist.PathElement:    "./Grout/Grout.sh",
	})

	if err := gl.Save(path); err != nil {
		gaba.GetLogger().Debug("Unable to save gamelist.xml file", "error", err)
	}

	return
}
