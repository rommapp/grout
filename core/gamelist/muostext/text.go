package muostext

import (
	"strings"

	"github.com/rommapp/grout/core/gamelist/entry"
)

const dateLayout = "2006-01-02"

func Render(e entry.Entry) []byte {
	var b strings.Builder
	line := func(label, value string) {
		if value == "" {
			return
		}
		b.WriteString(label + ": " + value + "\n")
	}
	line("Name", e.Name)
	if !e.ReleaseDate.IsZero() {
		line("Release Date", e.ReleaseDate.Format(dateLayout))
	}
	line("Languages", strings.Join(e.Languages, ", "))
	line("Genres", strings.Join(e.Genres, ", "))
	b.WriteString("\nDescription: " + e.Desc + "\n")
	return []byte(b.String())
}
