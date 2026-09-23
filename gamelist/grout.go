package gamelist

import (
	"grout/files"
	"log/slog"
	"os"
)

const (
	GroutEntryGameListName = "Grout"
)

func AddGroutEntry(path string, groutEntryPath string) {
	slog.Default().Debug("looking for correct gamelist.xml path", "path", path)
	gl := New()

	if files.FileExists(path) {
		data, err := os.ReadFile(path)
		if err != nil {
			slog.Default().Debug("Error reading gamelist.xml file", "error", err)
		}

		if len(data) > 0 {
			if err := gl.Parse(data); err != nil {
				slog.Default().Debug("gamelist.xml not found or can't be parsed, skipping grout entry check", "path", path, "error", err)
				return
			}
		} else {
			slog.Default().Debug("gamelist.xml file is empty", "path", path)
		}
	}

	if gl.GameContainsElements(GroutEntryGameListName, []string{
		PathElement, DescElement,
		ImageElement, DeveloperElement,
		PlayersElement, GenreElement,
	}) {
		slog.Default().Debug("gamelist.xml already contains Grout entry, skipping addition", "path", path)
		return
	}

	gl.AddOrUpdateEntry(GroutEntryGameListName, map[string]string{
		NameElement:      GroutEntryGameListName,
		DescElement:      "Download games wirelessly from your RomM instance",
		ImageElement:     "./Grout/logo.png",
		PlayersElement:   "1",
		GenreElement:     "Rom Manager",
		PathElement:      groutEntryPath,
		DeveloperElement: "The RomM Community",
	})

	if err := gl.Save(path); err != nil {
		slog.Default().Debug("Unable to save gamelist.xml file", "error", err)
		return
	}

	slog.Default().Debug("Successfully saved gamelist.xml file with Grout entry", "path", path)

	return
}
