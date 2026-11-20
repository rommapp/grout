package grout

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/rommapp/grout/core/config"
	"github.com/rommapp/grout/core/romm"
)

type Status struct {
	ConfigSource  string
	Host          string
	ServerVersion string
	Snapshots     bool
}

func (g *Grout) Status(ctx context.Context) (Status, error) {
	ctx, end, err := g.begin(ctx)
	if err != nil {
		return Status{}, err
	}
	defer end()
	cfg, src := g.settings()
	st, _, err := connect(ctx, cfg, src)
	return st, err
}

func connect(ctx context.Context, cfg *config.Config, src config.Source) (Status, *romm.Client, error) {
	st := Status{ConfigSource: src.String()}
	client := newClient(cfg)
	if client == nil {
		return st, nil, newError(CodeNotConfigured, errors.New("no RomM host configured"))
	}
	st.Host = cfg.Hosts[0].URL()
	hb, err := client.Heartbeat(ctx)
	if err != nil {
		return st, nil, classify(ctx, err, CodeServerUnreachable)
	}
	st.ServerVersion = hb.System.Version
	st.Snapshots = hb.SaveSync.Snapshots
	if err := hb.RequireSnapshots(); err != nil {
		return st, nil, newError(CodeServerTooOld, err)
	}
	return st, client, nil
}

func (g *Grout) Settings() config.Config {
	cfg, _ := g.settings()
	return *cloneConfig(cfg)
}

func (g *Grout) SetHost(h config.Host) error {
	_, end, err := g.begin(context.Background())
	if err != nil {
		return err
	}
	defer end()
	u, err := url.Parse(h.RootURI)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return newError(CodeBadRequest, fmt.Errorf("host url %q must be an http or https url", h.RootURI))
	}
	return g.update(func(c *config.Config) error {
		if len(c.Hosts) == 0 {
			c.Hosts = []config.Host{h}
		} else {
			c.Hosts[0] = h
		}
		return nil
	})
}

func (g *Grout) SetDirectoryMapping(fsSlug string, m *config.DirectoryMapping) error {
	_, end, err := g.begin(context.Background())
	if err != nil {
		return err
	}
	defer end()
	if fsSlug == "" {
		return newError(CodeBadRequest, errors.New("platform slug is required"))
	}
	return g.update(func(c *config.Config) error {
		if m == nil {
			delete(c.DirectoryMappings, fsSlug)
			return nil
		}
		if c.DirectoryMappings == nil {
			c.DirectoryMappings = map[string]config.DirectoryMapping{}
		}
		c.DirectoryMappings[fsSlug] = *m
		return nil
	})
}
