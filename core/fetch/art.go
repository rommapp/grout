package fetch

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/rommapp/grout/core/config"
	"github.com/rommapp/grout/core/firmware"
	"github.com/rommapp/grout/core/firmware/host"
	"github.com/rommapp/grout/core/library"
	"github.com/rommapp/grout/core/romm"
)

const maxArtBytes = 16 << 20

var maxRawBytes int64 = 512 << 20

var ErrArtTooLarge = errors.New("artwork exceeds the size limit")

var ErrBadStem = errors.New("file name stem is not a safe name")

func safeStem(s string) bool {
	return safeElem(s) && !strings.HasPrefix(s, StagePrefix)
}

type Size struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type ArtInput struct {
	Detail      romm.RomDetail
	Platform    library.Platform
	RomDir      string
	Stem        string
	Settings    config.Config
	MaxSize     Size
	OnlyMissing bool
	Client      *romm.Client
	Firmware    *firmware.Firmware
	Host        host.Writable
}

type ArtFile struct {
	Kind firmware.ArtKind `json:"kind"`
	Path string           `json:"path"`
}

type ArtResult struct {
	Written []ArtFile          `json:"written"`
	Failed  []firmware.ArtKind `json:"failed"`
}

func Wanted(c config.Config) map[firmware.ArtKind]romm.ArtSourceSpec {
	out := map[firmware.ArtKind]romm.ArtSourceSpec{}
	if !c.DownloadArt {
		return out
	}
	out[firmware.ArtCover] = romm.ArtSourceSpec{Source: romm.SourceCover, Kind: c.ArtKind}
	if c.DownloadArtScreenshotPreview {
		out[firmware.ArtPreview] = romm.ArtSourceSpec{Source: romm.SourceScreenshot}
	}
	splash := c.AdditionalDownloads.Thumbnail
	if splash == config.ArtKindNone {
		splash = c.DownloadSplashArt
	}
	if splash != config.ArtKindNone {
		spec := romm.ArtSourceSpec{Source: romm.SourceSplash, Kind: splash}
		out[firmware.ArtSplash] = spec
		out[firmware.ArtThumbnail] = spec
	}
	if k := c.AdditionalDownloads.Marquee; k != config.ArtKindNone {
		out[firmware.ArtMarquee] = romm.ArtSourceSpec{Source: romm.SourceMarquee, Kind: k}
	}
	flags := []struct {
		on   bool
		kind firmware.ArtKind
		src  romm.ArtSource
	}{
		{c.AdditionalDownloads.Video, firmware.ArtVideo, romm.SourceVideo},
		{c.AdditionalDownloads.Bezel, firmware.ArtBezel, romm.SourceBezel},
		{c.AdditionalDownloads.Manual, firmware.ArtManual, romm.SourceManual},
		{c.AdditionalDownloads.BoxBack, firmware.ArtBoxback, romm.SourceBoxback},
		{c.AdditionalDownloads.Fanart, firmware.ArtFanart, romm.SourceFanart},
	}
	for _, f := range flags {
		if f.on {
			out[f.kind] = romm.ArtSourceSpec{Source: f.src}
		}
	}
	return out
}

func Skipped(in ArtInput) ArtResult {
	res := ArtResult{Written: []ArtFile{}, Failed: []firmware.ArtKind{}}
	wanted := Wanted(in.Settings)
	for _, kind := range firmware.ArtKinds() {
		if _, ok := wanted[kind]; !ok {
			continue
		}
		if _, ok := in.Firmware.ArtDir(kind, in.Platform.Slug, in.RomDir, in.Platform.Name); ok {
			res.Failed = append(res.Failed, kind)
		}
	}
	return res
}

type artJob struct {
	kind firmware.ArtKind
	url  string
	dest string
}

func Art(ctx context.Context, in ArtInput, progress func(Progress)) ArtResult {
	if progress == nil {
		progress = func(Progress) {}
	}
	res := ArtResult{Written: []ArtFile{}, Failed: []firmware.ArtKind{}}
	wanted := Wanted(in.Settings)
	if !safeStem(in.Stem) {
		return Skipped(in)
	}
	var jobs []artJob
	for _, kind := range firmware.ArtKinds() {
		spec, ok := wanted[kind]
		if !ok {
			continue
		}
		dir, ok := in.Firmware.ArtDir(kind, in.Platform.Slug, in.RomDir, in.Platform.Name)
		if !ok {
			continue
		}
		u := in.Detail.ArtSource(spec.Source, spec.Kind)
		if u == "" {
			continue
		}
		dest := path.Join(dir, in.Firmware.ArtFileName(kind, in.Stem))
		if in.OnlyMissing {
			if _, err := in.Host.Stat(dest); err == nil {
				continue
			}
		}
		jobs = append(jobs, artJob{kind: kind, url: u, dest: dest})
	}
	cache := map[string][]byte{}
	for i, job := range jobs {
		if ctx.Err() != nil {
			for _, rest := range jobs[i:] {
				res.Failed = append(res.Failed, rest.kind)
			}
			return res
		}
		if err := fetchArt(ctx, in, job, cache); err != nil {
			res.Failed = append(res.Failed, job.kind)
		} else {
			res.Written = append(res.Written, ArtFile{Kind: job.kind, Path: job.dest})
		}
		progress(Progress{Stage: library.StageArt, Done: i + 1, Total: len(jobs)})
	}
	return res
}

func isRaw(kind firmware.ArtKind) bool {
	return kind == firmware.ArtVideo || kind == firmware.ArtManual
}

func fetchArt(ctx context.Context, in ArtInput, job artJob, cache map[string][]byte) error {
	var src io.Reader
	if cached, ok := cache[job.url]; ok && !isRaw(job.kind) {
		src = bytes.NewReader(cached)
	} else {
		body, err := in.Client.DownloadURL(ctx, job.url)
		if err != nil {
			return err
		}
		defer body.Close()
		reader := &ctxReader{ctx: ctx, r: body}
		if isRaw(job.kind) {
			if body.Size > maxRawBytes {
				return ErrArtTooLarge
			}
			src = &capReader{r: reader, left: maxRawBytes}
		} else {
			data, err := io.ReadAll(io.LimitReader(reader, maxArtBytes+1))
			if err != nil {
				return err
			}
			if len(data) > maxArtBytes {
				return ErrArtTooLarge
			}
			encoded, err := ToPNG(data, in.MaxSize)
			if err != nil {
				return err
			}
			cache[job.url] = encoded
			src = bytes.NewReader(encoded)
		}
	}
	f, staged, err := StageFile(in.Host, job.dest)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, src)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = ctx.Err()
	}
	if err == nil {
		err = Commit(in.Host, staged, job.dest)
	}
	if err != nil {
		Discard(in.Host, staged)
		return fmt.Errorf("writing %s: %w", job.dest, err)
	}
	return nil
}

type capReader struct {
	r    io.Reader
	left int64
}

func (c *capReader) Read(p []byte) (int, error) {
	if c.left <= 0 {
		var one [1]byte
		n, err := c.r.Read(one[:])
		if n > 0 {
			return 0, ErrArtTooLarge
		}
		return 0, err
	}
	if int64(len(p)) > c.left {
		p = p[:c.left]
	}
	n, err := c.r.Read(p)
	c.left -= int64(n)
	return n, err
}

type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

func (c *ctxReader) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return c.r.Read(p)
}
