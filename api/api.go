package api

import (
	"context"
	"log/slog"

	"github.com/rommapp/grout"
	"github.com/rommapp/grout/core/firmware"
	"github.com/rommapp/grout/version"
)

const Protocol = 1

type FirmwareSpec struct {
	ID         string `json:"id,omitempty"`
	Definition string `json:"definition,omitempty"`
}

type OpenOptions struct {
	DataDir  string            `json:"data_dir"`
	Firmware FirmwareSpec      `json:"firmware"`
	Env      map[string]string `json:"env,omitempty"`
	Logger   *slog.Logger      `json:"-"`
}

type VersionInfo struct {
	Release  string `json:"release"`
	Protocol int    `json:"protocol"`
}

type Server struct {
	g     *grout.Grout
	extra map[string]handler
}

func Open(ctx context.Context, o OpenOptions) (*Server, error) {
	opts := grout.Options{
		DataDir:    o.DataDir,
		FirmwareID: o.Firmware.ID,
		Env:        o.Env,
		Logger:     o.Logger,
	}
	if o.Firmware.Definition != "" {
		opts.FirmwareDefinition = []byte(o.Firmware.Definition)
	}
	g, err := grout.Open(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &Server{g: g}, nil
}

func (s *Server) Close() error {
	return s.g.Close()
}

func Version() VersionInfo {
	return VersionInfo{Release: version.Version, Protocol: Protocol}
}

func ValidateDefinition(data []byte) []string {
	d, err := firmware.Parse(data)
	if err != nil {
		return []string{err.Error()}
	}
	known, err := firmware.KnownSlugs()
	if err != nil {
		return []string{err.Error()}
	}
	msgs := []string{}
	for _, e := range firmware.ValidateHost(d, known, firmware.Hooks()) {
		msgs = append(msgs, e.Error())
	}
	return msgs
}

func (s *Server) register(name string, h handler) {
	if s.extra == nil {
		s.extra = map[string]handler{}
	}
	s.extra[name] = h
}
