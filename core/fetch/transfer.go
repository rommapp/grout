package fetch

import (
	"context"
	"errors"
	"io"
	"net/url"
	"time"

	"github.com/rommapp/grout/core/firmware/host"
	"github.com/rommapp/grout/core/library"
	"github.com/rommapp/grout/core/romm"
)

const progressEvery = 100 * time.Millisecond

type transfer struct {
	client   *romm.Client
	w        host.Writable
	clock    func() time.Time
	progress func(Progress)
	stage    library.Stage
	index    int
	count    int
	sink     io.Writer
}

func (t *transfer) download(ctx context.Context, p string, query url.Values, dest string, fallback int64) (string, error) {
	body, err := t.client.Download(ctx, p, query)
	if err != nil {
		return "", err
	}
	defer body.Close()
	total := fallback
	if body.Size >= 0 {
		total = body.Size
	}
	f, staged, err := StageFile(t.w, dest)
	if err != nil {
		return "", err
	}
	var dst io.Writer = f
	if t.sink != nil {
		dst = io.MultiWriter(f, t.sink)
	}
	done, err := t.copy(ctx, dst, body, total)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = ctx.Err()
	}
	if err != nil {
		Discard(t.w, staged)
		return "", err
	}
	t.progress(Progress{Stage: t.stage, Done: t.index + 1, Total: t.count, BytesDone: done, BytesTotal: total})
	return staged, nil
}

func (t *transfer) copy(ctx context.Context, dst io.Writer, src io.Reader, total int64) (int64, error) {
	buf := make([]byte, 64<<10)
	var done int64
	var last time.Time
	sent := false
	for {
		if err := ctx.Err(); err != nil {
			return done, err
		}
		n, rerr := src.Read(buf)
		if n > 0 {
			if _, err := dst.Write(buf[:n]); err != nil {
				return done, err
			}
			done += int64(n)
			now := t.clock()
			if !sent || now.Sub(last) >= progressEvery {
				last, sent = now, true
				t.progress(Progress{Stage: t.stage, Done: t.index, Total: t.count, BytesDone: done, BytesTotal: total})
			}
		}
		if errors.Is(rerr, io.EOF) {
			return done, nil
		}
		if rerr != nil {
			if err := ctx.Err(); err != nil {
				return done, err
			}
			return done, rerr
		}
	}
}
