package knulli

import (
	"fmt"
	"grout/internal/fileutil"
	"grout/internal/gamelist"
	"os"
	"path/filepath"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

const (
	GroutEntryGameListName = "Grout"
	flagPath               = "./knulli_restart_request"
)

func AddToToolsGameList() {
	path := filepath.Join(GetRomDirectory(), "tools", "gamelist.xml")
	gaba.GetLogger().Debug("using filepath for knulli gamelist.xml", "path", path)
	gl := gamelist.New()

	if fileutil.FileExists(path) {
		data, err := os.ReadFile(path)
		if err != nil {
			gaba.GetLogger().Debug("Error reading gamelist.xml file", "error", err)
		}

		if len(data) > 0 {
			gaba.GetLogger().Debug("Found gamelist.xml file", "data", string(data))
			if err := gl.Parse(data); err != nil {
				gaba.GetLogger().Debug("Knulli gamelist.xml not found or can't be parsed, skipping grout entry check", "path", path, "error", err)
				return
			}
		} else {
			gaba.GetLogger().Debug("gamelist.xml file is empty", "path", path)
		}
	}

	if gl.GameContainsElements(GroutEntryGameListName, []string{
		gamelist.PathElement, gamelist.DescElement,
		gamelist.ImageElement, gamelist.DeveloperElement,
		gamelist.PlayersElement, gamelist.GenreElement,
	}) {
		gaba.GetLogger().Debug("gamelist.xml already contains Grout entry, skipping addition", "path", path)
		return
	}

	gl.AdddOrUpdateEntry(GroutEntryGameListName, map[string]string{
		gamelist.NameElement:      GroutEntryGameListName,
		gamelist.DescElement:      "Download games wirelessly from your RomM instance",
		gamelist.ImageElement:     "./Grout/logo.png",
		gamelist.PlayersElement:   "1",
		gamelist.GenreElement:     "Rom Manager",
		gamelist.PathElement:      "./Grout/Grout.sh",
		gamelist.DeveloperElement: "Brandon Kowalski",
	})

	if err := gl.Save(path); err != nil {
		gaba.GetLogger().Debug("Unable to save gamelist.xml file", "error", err)
		return
	}

	gaba.GetLogger().Debug("Successfully saved gamelist.xml file with Grout entry", "path", path)

	err := ScheduleESRestart()
	if err != nil {
		gaba.GetLogger().Debug("Unable to schedule ES restart", "error", err)
		return
	}

	return
}

func ScheduleESRestart() error {
	file, err := os.Create(flagPath)
	if err != nil {
		return fmt.Errorf("unable to create restart flag file: %w", err)
	}
	defer file.Close()

	return nil
}
