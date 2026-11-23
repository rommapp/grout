package grout

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/rommapp/grout/core/auth"
	"github.com/rommapp/grout/core/config"
	"github.com/rommapp/grout/core/fetch"
	"github.com/rommapp/grout/core/firmware"
	"github.com/rommapp/grout/core/firmware/host"
	"github.com/rommapp/grout/core/gamelist"
	"github.com/rommapp/grout/core/library"
	"github.com/rommapp/grout/core/romm"
	"github.com/rommapp/grout/core/store"
	"github.com/rommapp/grout/version"
)

type Client struct {
	Name    string
	Version string
}

type Options struct {
	DataDir            string
	FirmwareID         string
	FirmwareDefinition []byte
	Env                map[string]string
	Host               host.Host
	Hooks              firmware.Registry
	Logger             *slog.Logger
	Client             Client
	Timing             auth.Timing
	ArtMaxSize         fetch.Size
	LauncherEntry      bool
}

type Grout struct {
	dataDir string
	lock    *os.File
	logFile *os.File
	log     *slog.Logger
	level   *slog.LevelVar
	client  Client
	timing  auth.Timing
	artMax  fetch.Size
	fw      *firmware.Firmware
	store   *store.Store
	lists   *gamelist.Writer

	root       context.Context
	cancel     context.CancelFunc
	closing    atomic.Bool
	life       sync.Mutex
	idle       *sync.Cond
	running    int
	callbacks  int
	releasing  bool
	released   chan struct{}
	releaseErr error

	setMu sync.Mutex
	cfgMu sync.RWMutex
	cfg   *config.Config
	src   config.Source

	writer   sync.RWMutex
	fetchMu  sync.Mutex
	fetching map[string]bool

	pairMu  sync.Mutex
	pending *pairing
	waiting bool
}

var errClosed = errors.New("grout is closed")

func Open(ctx context.Context, o Options) (*Grout, error) {
	if o.DataDir == "" {
		return nil, newError(CodeBadRequest, errors.New("data directory is required"))
	}
	if (o.FirmwareID == "") == (len(o.FirmwareDefinition) == 0) {
		return nil, newError(CodeBadDefinition, errors.New("exactly one of a firmware id or a firmware definition is required"))
	}
	if err := os.MkdirAll(o.DataDir, 0o755); err != nil {
		return nil, newError(CodeStorage, fmt.Errorf("creating %s: %w", o.DataDir, err))
	}
	lock, err := lockDataDir(o.DataDir)
	if err != nil {
		return nil, err
	}
	g := &Grout{dataDir: o.DataDir, lock: lock, client: clientIdentity(o.Client), timing: o.Timing, artMax: o.ArtMaxSize, fetching: map[string]bool{}}
	g.root, g.cancel = context.WithCancel(context.Background())
	g.idle = sync.NewCond(&g.life)
	g.released = make(chan struct{})
	if err := g.open(o); err != nil {
		g.cancel()
		g.release()
		return nil, err
	}
	g.log.Info("grout opened", "data_dir", o.DataDir, "firmware", g.fw.Def.ID, "card", g.fw.Card)
	return g, nil
}

func (g *Grout) open(o Options) error {
	cfg, src, err := config.Load(g.dataDir)
	if err != nil {
		return newError(CodeStorage, err)
	}
	g.cfg, g.src = cfg, src

	g.log = o.Logger
	if g.log == nil {
		g.level = new(slog.LevelVar)
		g.level.Set(cfg.LogLevel.Slog())
		if err := g.openLog(g.level); err != nil {
			return err
		}
	}

	h := o.Host
	if h == nil {
		env := o.Env
		h = host.OS(func(k string) string { return env[k] })
	}

	reg := firmware.MergeHooks(firmware.Hooks(), o.Hooks)
	var def *firmware.Definition
	if o.FirmwareID != "" {
		def, err = firmware.Load(o.FirmwareID)
	} else {
		def, err = firmware.LoadHost(o.FirmwareDefinition, reg)
	}
	if err != nil {
		return newError(CodeBadDefinition, err)
	}
	fw, err := firmware.ResolveWith(def, h, firmware.Overrides{MuOSRomStorage: string(cfg.MuOSRomStorage)}, reg)
	if err != nil {
		return newError(CodeBadDefinition, fmt.Errorf("firmware %q: %w", def.ID, err))
	}
	g.fw = fw

	s, err := library.Open(g.dataDir)
	if err != nil {
		return newError(CodeStorage, err)
	}
	g.store = s
	if w, ok := fw.Host.(host.Writable); ok {
		g.lists = gamelist.NewWriter(fw, w)
	}
	if o.LauncherEntry {
		g.writeLauncher()
	}
	return nil
}

func clientIdentity(c Client) Client {
	if c.Name == "" {
		c.Name = "grout"
	}
	if c.Version == "" {
		c.Version = version.Version
	}
	return c
}

func (g *Grout) openLog(level slog.Leveler) error {
	dir := filepath.Join(g.dataDir, "logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return newError(CodeStorage, fmt.Errorf("creating %s: %w", dir, err))
	}
	path := filepath.Join(dir, "grout.log")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return newError(CodeStorage, fmt.Errorf("opening %s: %w", path, err))
	}
	g.logFile = f
	g.log = slog.New(slog.NewTextHandler(f, &slog.HandlerOptions{Level: level}))
	return nil
}

func (g *Grout) release() error {
	var errs []error
	if g.store != nil {
		errs = append(errs, g.store.DB.Close())
	}
	if g.logFile != nil {
		errs = append(errs, g.logFile.Close())
	}
	if g.lock != nil {
		errs = append(errs, unlockDataDir(g.lock))
	}
	return errors.Join(errs...)
}

func (g *Grout) Close() error {
	g.closing.Store(true)
	g.cancel()
	g.life.Lock()
	for g.running > 0 {
		g.idle.Wait()
	}
	if g.callbacks > 0 {
		g.life.Unlock()
		return nil
	}
	if g.claimRelease() {
		return g.finishRelease()
	}
	g.life.Unlock()
	<-g.released
	return g.releaseErr
}

func (g *Grout) claimRelease() bool {
	if g.releasing || !g.closing.Load() || g.running > 0 || g.callbacks > 0 {
		return false
	}
	g.releasing = true
	g.life.Unlock()
	return true
}

func (g *Grout) finishRelease() error {
	g.log.Info("grout closed")
	if err := g.release(); err != nil {
		g.releaseErr = newError(CodeStorage, err)
	}
	close(g.released)
	return g.releaseErr
}

func (g *Grout) Firmware() *firmware.Firmware {
	return g.fw
}

func (g *Grout) begin(ctx context.Context) (context.Context, func(), error) {
	if g.closing.Load() {
		return nil, nil, newError(CodeBadRequest, errClosed)
	}
	g.life.Lock()
	if g.closing.Load() {
		g.life.Unlock()
		return nil, nil, newError(CodeBadRequest, errClosed)
	}
	g.running++
	g.life.Unlock()
	opCtx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(g.root, cancel)
	return opCtx, func() {
		stop()
		cancel()
		g.end()
	}, nil
}

func (g *Grout) end() {
	g.life.Lock()
	g.running--
	if g.running == 0 {
		g.idle.Broadcast()
	}
	if g.claimRelease() {
		g.finishRelease()
		return
	}
	g.life.Unlock()
}

func (g *Grout) enterCallback() {
	g.life.Lock()
	g.running--
	g.callbacks++
	if g.running == 0 {
		g.idle.Broadcast()
	}
	g.life.Unlock()
}

func (g *Grout) exitCallback() {
	g.life.Lock()
	g.callbacks--
	g.running++
	g.life.Unlock()
}

func (g *Grout) progress(fn func(library.Progress)) func(library.Progress) {
	if fn == nil {
		return nil
	}
	return func(p library.Progress) {
		g.enterCallback()
		defer g.exitCallback()
		fn(p)
	}
}

func (g *Grout) settings() (*config.Config, config.Source) {
	g.cfgMu.RLock()
	defer g.cfgMu.RUnlock()
	return g.cfg, g.src
}

func (g *Grout) update(change func(c *config.Config) error) error {
	return g.updateThen(change, nil)
}

func (g *Grout) updateThen(change func(c *config.Config) error, saved func(c *config.Config)) error {
	g.setMu.Lock()
	defer g.setMu.Unlock()
	cur, _ := g.settings()
	next := cloneConfig(cur)
	if err := change(next); err != nil {
		return err
	}
	if err := config.Save(g.dataDir, next); err != nil {
		return newError(CodeStorage, err)
	}
	g.cfgMu.Lock()
	g.cfg, g.src = next, config.SourceSettings
	g.cfgMu.Unlock()
	if saved != nil {
		saved(next)
	}
	return nil
}

func cloneConfig(c *config.Config) *config.Config {
	out := *c
	out.Hosts = slices.Clone(c.Hosts)
	out.DirectoryMappings = maps.Clone(c.DirectoryMappings)
	out.PlatformOrder = slices.Clone(c.PlatformOrder)
	out.SaveDirectoryMappings = maps.Clone(c.SaveDirectoryMappings)
	out.ChannelPreferences = maps.Clone(c.ChannelPreferences)
	return &out
}

func newClient(cfg *config.Config) *romm.Client {
	if len(cfg.Hosts) == 0 {
		return nil
	}
	h := cfg.Hosts[0]
	return romm.New(romm.Options{
		BaseURL:            h.URL(),
		AuthHeader:         h.AuthHeader(),
		InsecureSkipVerify: h.InsecureSkipVerify,
		Timeout:            cfg.ApiTimeout.Duration(),
	})
}

func (g *Grout) library(cfg *config.Config, client *romm.Client) *library.Library {
	l := library.New(g.store, client, cfg, g.fw)
	l.Log = g.log
	return l
}

func (g *Grout) snapshot() *library.Library {
	cfg, _ := g.settings()
	return g.library(cfg, newClient(cfg))
}
