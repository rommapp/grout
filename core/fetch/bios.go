package fetch

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/rommapp/grout/core/firmware"
	"github.com/rommapp/grout/core/firmware/host"
	"github.com/rommapp/grout/core/library"
	"github.com/rommapp/grout/core/romm"
)

type BiosCheck string

const (
	BiosVerified   BiosCheck = "verified"
	BiosMismatched BiosCheck = "mismatched"
	BiosUnchecked  BiosCheck = "unchecked"
)

func BiosChecks() []BiosCheck {
	return []BiosCheck{BiosVerified, BiosMismatched, BiosUnchecked}
}

type BiosFile struct {
	FirmwareID int       `json:"firmware_id"`
	FileName   string    `json:"file_name"`
	Paths      []string  `json:"paths"`
	Check      BiosCheck `json:"check"`
	IsVerified bool      `json:"is_verified"`
	Present    bool      `json:"present"`
}

type BiosInput struct {
	PlatformSlug string
	Entries      []romm.Firmware
	Client       *romm.Client
	Firmware     *firmware.Firmware
	Host         host.Writable
	Clock        func() time.Time
}

func BiosDestinations(fw *firmware.Firmware, slug, fileName string) []string {
	var out []string
	seen := map[string]bool{}
	for _, rel := range firmware.BiosRels(fileName) {
		for _, p := range fw.BiosPaths(slug, rel) {
			if !seen[p] {
				seen[p] = true
				out = append(out, p)
			}
		}
	}
	return out
}

func biosNameOK(name string) bool {
	return safeElem(name) && !strings.HasPrefix(name, StagePrefix)
}

func biosPresent(h host.Host, dests []string) bool {
	if len(dests) == 0 {
		return false
	}
	for _, d := range dests {
		info, err := h.Stat(d)
		if err != nil || !info.Mode().IsRegular() {
			return false
		}
	}
	return true
}

func BiosStatus(h host.Host, fw *firmware.Firmware, slug string, entries []romm.Firmware) []BiosFile {
	out := make([]BiosFile, 0, len(entries))
	for _, e := range entries {
		f := BiosFile{FirmwareID: e.ID, FileName: e.FileName, Check: BiosUnchecked, IsVerified: e.IsVerified}
		if biosNameOK(e.FileName) {
			f.Paths = BiosDestinations(fw, slug, e.FileName)
			f.Present = biosPresent(h, f.Paths)
		}
		out = append(out, f)
	}
	return out
}

func Bios(ctx context.Context, in BiosInput, progress func(Progress)) ([]BiosFile, error) {
	if progress == nil {
		progress = func(Progress) {}
	}
	out := make([]BiosFile, 0, len(in.Entries))
	total := len(in.Entries)
	clock := in.Clock
	if clock == nil {
		clock = time.Now
	}
	t := &transfer{client: in.Client, w: in.Host, clock: clock, progress: progress, stage: library.StageBios, count: total}
	for i, e := range in.Entries {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		f := BiosFile{FirmwareID: e.ID, FileName: e.FileName, Check: BiosUnchecked, IsVerified: e.IsVerified}
		if !biosNameOK(e.FileName) {
			out = append(out, f)
			progress(Progress{Stage: library.StageBios, Done: i + 1, Total: total})
			continue
		}
		f.Paths = BiosDestinations(in.Firmware, in.PlatformSlug, e.FileName)
		if len(f.Paths) == 0 {
			out = append(out, f)
			progress(Progress{Stage: library.StageBios, Done: i + 1, Total: total})
			continue
		}
		t.index = i
		t.sink = nil
		var placed []string
		check, err := placeBios(ctx, t, in, e, f.Paths, &placed)
		if err != nil && len(placed) == 0 && ctx.Err() == nil && romm.IsNotFound(err) {
			f.Present = biosPresent(in.Host, f.Paths)
			out = append(out, f)
			progress(Progress{Stage: library.StageBios, Done: i + 1, Total: total})
			continue
		}
		if err != nil {
			if len(placed) > 0 {
				f.Paths = placed
				f.Present = biosPresent(in.Host, placed)
				out = append(out, f)
			}
			return out, err
		}
		f.Check = check
		f.Present = biosPresent(in.Host, f.Paths)
		out = append(out, f)
	}
	return out, nil
}

func placeBios(ctx context.Context, t *transfer, in BiosInput, e romm.Firmware, dests []string, placed *[]string) (BiosCheck, error) {
	hash := md5.New()
	t.sink = hash
	staged, err := t.download(ctx, romm.FirmwareContentPath(e.ID, e.FileName), url.Values{"purpose": {"download"}}, dests[0], e.SizeBytes)
	if err != nil {
		return "", err
	}
	check := BiosUnchecked
	if e.MD5 != "" {
		if !strings.EqualFold(strings.TrimSpace(e.MD5), hex.EncodeToString(hash.Sum(nil))) {
			Discard(in.Host, staged)
			return BiosMismatched, nil
		}
		check = BiosVerified
	}
	if err := Commit(in.Host, staged, dests[0]); err != nil {
		Discard(in.Host, staged)
		return "", err
	}
	*placed = append(*placed, dests[0])
	for _, d := range dests[1:] {
		if err := copyBios(ctx, in.Host, dests[0], d); err != nil {
			return "", err
		}
		*placed = append(*placed, d)
	}
	return check, nil
}

func copyBios(ctx context.Context, w host.Writable, from, to string) error {
	realFrom, err := w.Real(from)
	if err != nil {
		return err
	}
	src, err := os.Open(realFrom)
	if err != nil {
		return err
	}
	defer src.Close()
	f, staged, err := StageFile(w, to)
	if err != nil {
		return err
	}
	quiet := &transfer{clock: time.Now, progress: func(Progress) {}}
	_, err = quiet.copy(ctx, f, src, 0)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		Discard(w, staged)
		return err
	}
	if err := Commit(w, staged, to); err != nil {
		Discard(w, staged)
		return err
	}
	return nil
}
