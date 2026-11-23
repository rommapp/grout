package gamelist

import (
	"context"
	"path"

	"github.com/rommapp/grout/core/firmware"
	"github.com/rommapp/grout/core/gamelist/entry"
)

func launcherEntry(p string) entry.Entry {
	return entry.Entry{
		Path:       p,
		Name:       "Grout",
		Desc:       "Download games wirelessly from your RomM instance",
		Players:    "1",
		Genres:     []string{"Rom Manager"},
		Developers: []string{"The RomM Community"},
		Art:        map[firmware.ArtKind]string{firmware.ArtCover: "./" + path.Join(path.Dir(p), "logo.png")},
	}
}

func (w *Writer) Launcher(ctx context.Context) (Result, error) {
	res := newResult()
	format := w.fw.GamelistFormat()
	if format != firmware.FormatEmulationStation && format != firmware.FormatMiyoo {
		return res, nil
	}
	le, ok := w.fw.LauncherEntry()
	if !ok {
		return res, nil
	}
	e := launcherEntry(le.Path)
	written, err := w.update(ctx, le.Gamelist, func(existing []byte) ([]byte, bool, error) {
		return render(format, existing, []entry.Entry{e})
	})
	if err != nil {
		return res, err
	}
	if written {
		res.Written = append(res.Written, le.Gamelist)
	}
	return res, nil
}
