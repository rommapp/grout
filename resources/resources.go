package resources

import (
	"embed"
	"fmt"

	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
)

//go:embed locales/*.toml splash.png
var embeddedFiles embed.FS

type LocaleFile struct {
	Name string
	Path string
}

var localeFiles = []LocaleFile{
	{Name: "active.en.toml", Path: "locales/active.en.toml"},
	{Name: "active.es.toml", Path: "locales/active.es.toml"},
	{Name: "active.fr.toml", Path: "locales/active.fr.toml"},
}

func GetLocaleMessageFiles() ([]i18n.MessageFile, error) {
	var messageFiles []i18n.MessageFile

	for _, localeFile := range localeFiles {
		content, err := embeddedFiles.ReadFile(localeFile.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to read embedded locale file %s: %w", localeFile.Path, err)
		}

		messageFiles = append(messageFiles, i18n.MessageFile{
			Name:    localeFile.Name,
			Content: content,
		})
	}

	return messageFiles, nil
}

func GetSplashImageBytes() ([]byte, error) {
	data, err := embeddedFiles.ReadFile("splash.png")
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded splash image: %w", err)
	}
	return data, nil
}
