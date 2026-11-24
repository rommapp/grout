package sigil

import (
	"errors"

	binding "github.com/rommforge/argosy-sigil/bindings/go"

	"github.com/rommapp/grout/core/ident"
)

type Sigil struct{}

var _ ident.Extractor = Sigil{}

func Version() string {
	return binding.Version()
}

func (Sigil) Version() string {
	return binding.Version()
}

func (Sigil) Extract(path, platform string) ident.Result {
	r, err := binding.Extract(path, binding.PlatformFromSlug(platform), &binding.Options{})
	if err != nil {
		return ident.Result{Status: statusOf(err)}
	}
	return ident.Result{
		Status:       ident.StatusOK,
		TitleID:      r.TitleID,
		SaveID:       r.SaveID,
		RawSerial:    r.RawSerial,
		Platform:     r.PlatformSlug,
		Source:       ident.Source(r.Source.String()),
		Usage:        ident.Usage(r.Usage.String()),
		Features:     r.Features,
		Experimental: r.Experimental,
		N64Header:    r.N64Header,
		N64MD5:       r.N64MD5,
		N64MD5N64:    r.N64MD5N64,
	}
}

func statusOf(err error) ident.Status {
	switch {
	case errors.Is(err, binding.ErrNotFound):
		return ident.StatusNotFound
	case errors.Is(err, binding.ErrNeedsKey),
		errors.Is(err, binding.ErrKeysIncompatible),
		errors.Is(err, binding.ErrCrypto):
		return ident.StatusNeedsKey
	case errors.Is(err, binding.ErrUnsupportedFormat),
		errors.Is(err, binding.ErrUnknownPlatform):
		return ident.StatusUnsupported
	case errors.Is(err, binding.ErrIO):
		return ident.StatusIO
	default:
		return ident.StatusFailed
	}
}
