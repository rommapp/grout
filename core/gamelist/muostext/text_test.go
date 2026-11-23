package muostext

import (
	"testing"
	"time"

	"github.com/rommapp/grout/core/gamelist/entry"
)

func TestRenderMuOSText(t *testing.T) {
	e := entry.Entry{
		Path:        "./Game.gba",
		Name:        "Game (USA, Europe)",
		Desc:        "First line\nSecond line",
		ReleaseDate: time.Date(2001, 6, 11, 0, 0, 0, 0, time.UTC),
		Genres:      []string{"Action", "Platform"},
		Languages:   []string{"English", "French"},
		Developers:  []string{"Dev"},
		Players:     "1",
	}
	want := "Name: Game (USA, Europe)\n" +
		"Release Date: 2001-06-11\n" +
		"Languages: English, French\n" +
		"Genres: Action, Platform\n" +
		"\n" +
		"Description: First line\nSecond line\n"
	if got := string(Render(e)); got != want {
		t.Fatalf("Render =\n%q\nwant\n%q", got, want)
	}
}

func TestRenderMuOSTextSkipsEmpty(t *testing.T) {
	e := entry.Entry{Name: "Game"}
	want := "Name: Game\n\nDescription: \n"
	if got := string(Render(e)); got != want {
		t.Fatalf("Render = %q, want %q", got, want)
	}
}
