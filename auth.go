package grout

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/rommapp/grout/core/auth"
	"github.com/rommapp/grout/core/config"
	"github.com/rommapp/grout/core/romm"
)

const followUpTimeout = 2 * time.Minute

var (
	errNoHost        = errors.New("no RomM host configured")
	errNoToken       = errors.New("not signed in to RomM")
	errNoPairing     = errors.New("no pairing in progress")
	errPairing       = errors.New("already waiting for this pairing")
	errNoDeviceName  = errors.New("device name is required")
	errPairingFailed = errors.New("pairing failed")
	errHostChanged   = errors.New("the RomM host changed since pairing started")
)

type pairing struct {
	pending auth.Pending
	name    string
	url     string
}

type PairPending struct {
	UserCode        string
	VerificationURL string
	ExpiresAt       time.Time
}

type PairResult struct {
	Outcome   auth.Outcome
	Username  string
	DeviceID  string
	ExpiresAt string
}

type LoginResult struct {
	Username  string
	DeviceID  string
	ExpiresAt string
	Scopes    []string
}

type DeviceInfo struct {
	DeviceID string
	Name     string
}

func requireHost(cfg *config.Config) (config.Host, error) {
	if len(cfg.Hosts) == 0 || cfg.Hosts[0].RootURI == "" {
		return config.Host{}, newError(CodeNotConfigured, errNoHost)
	}
	return cfg.Hosts[0], nil
}

func hostClient(cfg *config.Config, h config.Host) *romm.Client {
	return romm.New(romm.Options{
		BaseURL:            h.URL(),
		AuthHeader:         h.AuthHeader(),
		InsecureSkipVerify: h.InsecureSkipVerify,
		Timeout:            cfg.ApiTimeout.Duration(),
	})
}

func (g *Grout) updateHost(change func(h *config.Host)) error {
	return g.update(func(c *config.Config) error {
		if len(c.Hosts) == 0 || c.Hosts[0].RootURI == "" {
			return newError(CodeNotConfigured, errNoHost)
		}
		change(&c.Hosts[0])
		return nil
	})
}

func (g *Grout) updateHostAt(url string, change func(h *config.Host)) error {
	return g.update(func(c *config.Config) error {
		if len(c.Hosts) == 0 || c.Hosts[0].RootURI == "" || c.Hosts[0].URL() != url {
			return newError(CodeBadRequest, errHostChanged)
		}
		change(&c.Hosts[0])
		return nil
	})
}

func (g *Grout) detached(ctx context.Context) (context.Context, context.CancelFunc) {
	c, cancel := context.WithTimeout(context.WithoutCancel(ctx), followUpTimeout)
	stop := context.AfterFunc(g.root, cancel)
	return c, func() {
		stop()
		cancel()
	}
}

func newClientDeviceID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", newError(CodeInternal, fmt.Errorf("generating a client device id: %w", err))
	}
	return hex.EncodeToString(b), nil
}

func (g *Grout) ensureClientDeviceID(url string) (string, error) {
	cfg, _ := g.settings()
	h, err := requireHost(cfg)
	if err != nil {
		return "", err
	}
	if h.URL() != url {
		return "", newError(CodeBadRequest, errHostChanged)
	}
	if h.ClientDeviceID != "" {
		return h.ClientDeviceID, nil
	}
	id, err := newClientDeviceID()
	if err != nil {
		return "", err
	}
	err = g.updateHostAt(url, func(h *config.Host) {
		if h.ClientDeviceID == "" {
			h.ClientDeviceID = id
		}
		id = h.ClientDeviceID
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

func (g *Grout) PairStart(ctx context.Context, deviceName string) (PairPending, error) {
	ctx, end, err := g.begin(ctx)
	if err != nil {
		return PairPending{}, err
	}
	defer end()
	if deviceName == "" {
		return PairPending{}, newError(CodeBadRequest, errNoDeviceName)
	}
	cfg, src := g.settings()
	h, err := requireHost(cfg)
	if err != nil {
		return PairPending{}, err
	}
	url := h.URL()
	anon := cloneConfig(cfg)
	anon.Hosts[0].Token = ""
	_, client, err := connect(ctx, anon, src)
	if err != nil {
		return PairPending{}, err
	}
	clientDeviceID, err := g.ensureClientDeviceID(url)
	if err != nil {
		return PairPending{}, err
	}
	p, err := auth.Start(ctx, client, url, romm.DeviceInitRequest{
		ClientDeviceIdentifier: clientDeviceID,
		Name:                   deviceName,
		Client:                 g.client.Name,
		Platform:               g.fw.Def.ID,
		ClientVersion:          g.client.Version,
		RequestedScopes:        romm.Scopes,
	}, g.timing)
	if err != nil {
		return PairPending{}, classify(ctx, err, CodeServerUnreachable)
	}
	g.pairMu.Lock()
	g.pending = &pairing{pending: p, name: deviceName, url: url}
	g.pairMu.Unlock()
	g.log.Info("pairing started", "device", deviceName)
	return PairPending{UserCode: p.UserCode, VerificationURL: p.VerificationURL, ExpiresAt: p.ExpiresAt}, nil
}

func (g *Grout) claimPairing() (*pairing, error) {
	g.pairMu.Lock()
	defer g.pairMu.Unlock()
	if g.waiting {
		return nil, newError(CodeBusy, errPairing)
	}
	if g.pending == nil {
		return nil, newError(CodeBadRequest, errNoPairing)
	}
	g.waiting = true
	return g.pending, nil
}

func (g *Grout) releasePairing() {
	g.pairMu.Lock()
	g.waiting = false
	g.pairMu.Unlock()
}

func (g *Grout) dropPairing(p *pairing) {
	g.pairMu.Lock()
	if g.pending == p {
		g.pending = nil
	}
	g.pairMu.Unlock()
}

func (g *Grout) PairWait(ctx context.Context) (PairResult, error) {
	ctx, end, err := g.begin(ctx)
	if err != nil {
		return PairResult{}, err
	}
	defer end()
	p, err := g.claimPairing()
	if err != nil {
		return PairResult{}, err
	}
	defer g.releasePairing()
	cfg, _ := g.settings()
	h, err := requireHost(cfg)
	if err != nil {
		return PairResult{}, err
	}
	if h.URL() != p.url {
		g.dropPairing(p)
		return PairResult{}, newError(CodeBadRequest, errHostChanged)
	}
	h.Token = ""
	approval, outcome, err := auth.Wait(ctx, hostClient(cfg, h), p.pending, g.timing)
	switch outcome {
	case auth.OutcomeApproved:
		return g.finishPairing(ctx, p, approval)
	case auth.OutcomeDenied, auth.OutcomeExpired:
		g.dropPairing(p)
		g.log.Info("pairing ended", "outcome", outcome)
		return PairResult{Outcome: outcome}, nil
	case auth.OutcomeFailed:
	}
	if err == nil {
		err = errPairingFailed
	}
	if ctx.Err() != nil || errors.Is(err, context.Canceled) {
		return PairResult{}, newError(CodeCancelled, err)
	}
	g.log.Error("pairing failed", "err", err)
	return PairResult{Outcome: auth.OutcomeFailed}, newError(CodeServerUnreachable, err)
}

func (g *Grout) finishPairing(ctx context.Context, p *pairing, a auth.Approval) (PairResult, error) {
	var savedCfg *config.Config
	err := g.update(func(c *config.Config) error {
		if len(c.Hosts) == 0 || c.Hosts[0].URL() != p.url {
			return newError(CodeBadRequest, errHostChanged)
		}
		h := &c.Hosts[0]
		h.Token = a.Token
		h.TokenName = p.name
		h.TokenExpiresAt = a.ExpiresAt
		h.Username = ""
		h.DeviceID = a.DeviceID
		h.DeviceName = p.name
		h.DeviceClientVersion = g.client.Version
		savedCfg = c
		return nil
	})
	g.dropPairing(p)
	if err != nil {
		g.log.Info("pairing approval not saved", "err", err)
		return PairResult{}, err
	}
	res := PairResult{Outcome: auth.OutcomeApproved, DeviceID: a.DeviceID, ExpiresAt: a.ExpiresAt}
	fctx, cancel := g.detached(ctx)
	defer cancel()
	client := hostClient(savedCfg, savedCfg.Hosts[0])
	_ = auth.Settle(fctx, func(c context.Context) error {
		_, err := client.Platforms(c)
		return err
	}, g.timing)
	u, err := client.Me(fctx)
	if err != nil || u.Username == "" {
		g.log.Info("paired without a username", "err", err)
		return res, nil
	}
	err = g.updateHostAt(p.url, func(h *config.Host) {
		if h.Token == a.Token {
			h.Username = u.Username
		}
	})
	if err != nil {
		return res, err
	}
	res.Username = u.Username
	g.log.Info("paired", "user", u.Username, "device", a.DeviceID)
	return res, nil
}

func exchangeError(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, auth.ErrNoCode):
		return newError(CodeBadRequest, err)
	case errors.Is(err, auth.ErrInvalidCode):
		return newError(CodeInvalidCode, err)
	case errors.Is(err, auth.ErrRateLimited):
		return newError(CodeRateLimited, err)
	}
	return classify(ctx, err, CodeServerUnreachable)
}

func clearSignIn(h *config.Host) {
	h.Token = ""
	h.TokenName = ""
	h.TokenExpiresAt = ""
	h.Username = ""
	h.DeviceID = ""
	h.DeviceName = ""
	h.DeviceClientVersion = ""
}

func (g *Grout) ExchangeCode(ctx context.Context, code string) (LoginResult, error) {
	ctx, end, err := g.begin(ctx)
	if err != nil {
		return LoginResult{}, err
	}
	defer end()
	cfg, _ := g.settings()
	h, err := requireHost(cfg)
	if err != nil {
		return LoginResult{}, err
	}
	url := h.URL()
	h.Token = ""
	tok, err := auth.Exchange(ctx, hostClient(cfg, h), code, g.timing)
	if err != nil {
		return LoginResult{}, exchangeError(ctx, err)
	}
	if tok.RawToken == "" {
		return LoginResult{}, newError(CodeUnauthorized, errors.New("RomM returned no token"))
	}
	var deviceID string
	err = g.updateHostAt(url, func(h *config.Host) {
		h.Token = tok.RawToken
		h.TokenName = tok.Name
		h.TokenExpiresAt = tok.ExpiresAt
		h.Username = ""
		switch {
		case tok.DeviceID == "":
			h.DeviceID = ""
			h.DeviceName = ""
			h.DeviceClientVersion = ""
		case tok.DeviceID != h.DeviceID:
			h.DeviceID = tok.DeviceID
			h.DeviceName = ""
			h.DeviceClientVersion = ""
		}
		deviceID = h.DeviceID
	})
	if err != nil {
		return LoginResult{}, err
	}
	h.Token = tok.RawToken
	client := hostClient(cfg, h)
	if _, err := client.Platforms(ctx); err != nil {
		if ctx.Err() == nil && romm.IsUnauthorized(err) {
			_ = g.updateHostAt(url, func(h *config.Host) {
				if h.Token == tok.RawToken {
					clearSignIn(h)
				}
			})
			return LoginResult{}, newError(CodeUnauthorized, fmt.Errorf("the new token was not accepted: %w", err))
		}
		g.log.Info("signed in but the token could not be checked", "err", err)
		return LoginResult{}, classify(ctx, err, CodeServerUnreachable)
	}
	username := ""
	if u, err := client.Me(ctx); err == nil {
		username = u.Username
	} else {
		g.log.Info("signed in without a username", "err", err)
	}
	if username != "" {
		err = g.updateHostAt(url, func(h *config.Host) {
			if h.Token == tok.RawToken {
				h.Username = username
			}
		})
		if err != nil {
			return LoginResult{}, err
		}
	}
	g.log.Info("signed in with a pairing code", "user", username)
	return LoginResult{Username: username, DeviceID: deviceID, ExpiresAt: tok.ExpiresAt, Scopes: tok.Scopes}, nil
}

func (g *Grout) Logout() error {
	_, end, err := g.begin(context.Background())
	if err != nil {
		return err
	}
	defer end()
	err = g.updateHost(clearSignIn)
	if err == nil {
		g.log.Info("signed out")
	}
	return err
}

func (g *Grout) RegisterDevice(ctx context.Context, name string) (DeviceInfo, error) {
	ctx, end, err := g.begin(ctx)
	if err != nil {
		return DeviceInfo{}, err
	}
	defer end()
	cfg, _ := g.settings()
	h, err := requireHost(cfg)
	if err != nil {
		return DeviceInfo{}, err
	}
	if h.Token == "" {
		return DeviceInfo{}, newError(CodeNotConfigured, errNoToken)
	}
	if name == "" {
		return DeviceInfo{}, newError(CodeBadRequest, errNoDeviceName)
	}
	client := hostClient(cfg, h)
	id := h.DeviceID
	if id == "" {
		ref, err := client.RegisterDevice(ctx, romm.DeviceRegistration{
			Name:          name,
			Platform:      g.fw.Def.ID,
			Client:        g.client.Name,
			ClientVersion: g.client.Version,
			SyncMode:      romm.SyncModeAPI,
		})
		if err != nil {
			return DeviceInfo{}, classify(ctx, err, CodeServerUnreachable)
		}
		if ref.DeviceID == "" {
			return DeviceInfo{}, newError(CodeServerUnreachable, errors.New("RomM returned no device id"))
		}
		id = ref.DeviceID
		err = g.updateHostAt(h.URL(), func(cur *config.Host) {
			cur.DeviceID = id
			cur.DeviceName = name
			cur.DeviceClientVersion = ""
		})
		if err != nil {
			return DeviceInfo{}, err
		}
		if err := client.UpdateDevice(ctx, id, romm.DeviceUpdate{ClientVersion: g.client.Version}); err != nil {
			return DeviceInfo{}, classify(ctx, err, CodeServerUnreachable)
		}
	} else if h.DeviceClientVersion != g.client.Version || h.DeviceName != name {
		if err := client.UpdateDevice(ctx, id, romm.DeviceUpdate{Name: name, ClientVersion: g.client.Version}); err != nil {
			return DeviceInfo{}, classify(ctx, err, CodeServerUnreachable)
		}
	} else {
		return DeviceInfo{DeviceID: id, Name: name}, nil
	}
	err = g.updateHostAt(h.URL(), func(cur *config.Host) {
		cur.DeviceID = id
		cur.DeviceName = name
		cur.DeviceClientVersion = g.client.Version
	})
	if err != nil {
		return DeviceInfo{}, err
	}
	g.log.Info("device registered", "device", id, "name", name)
	return DeviceInfo{DeviceID: id, Name: name}, nil
}
