package grout

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/rommapp/grout/core/config"
)

const (
	maxAPITimeout      = 300 * time.Second
	maxDownloadTimeout = 24 * time.Hour
)

type SettingsPatch struct {
	ShowRegularCollections *bool
	ShowSmartCollections   *bool
	ShowVirtualCollections *bool
	DownloadedGames        *config.DownloadedGamesMode
	APITimeout             *time.Duration
	DownloadTimeout        *time.Duration
	LogLevel               *config.LogLevel
	PlatformOrder          *[]string

	DownloadArt                  *bool
	UnzipDownloads               *bool
	ArtKind                      *config.ArtKind
	DownloadArtScreenshotPreview *bool
	DownloadSplashArt            *config.ArtKind
	AdditionalDownloads          *AdditionalDownloadsPatch
	GamelistOmitsRegion          *bool
}

type AdditionalDownloadsPatch struct {
	Marquee   *config.ArtKind
	Video     *bool
	Thumbnail *config.ArtKind
	Bezel     *bool
	Manual    *bool
	BoxBack   *bool
	Fanart    *bool
}

func (p *AdditionalDownloadsPatch) validate() error {
	if p == nil {
		return nil
	}
	if p.Thumbnail != nil && !slices.Contains(config.SplashArtKinds(), *p.Thumbnail) {
		return badSetting("unknown thumbnail art kind %q", *p.Thumbnail)
	}
	if p.Marquee != nil && !slices.Contains(config.MarqueeArtKinds(), *p.Marquee) {
		return badSetting("unknown marquee art kind %q", *p.Marquee)
	}
	return nil
}

func (p *AdditionalDownloadsPatch) apply(a *config.AdditionalDownloads) {
	if p == nil {
		return
	}
	setIf(&a.Marquee, p.Marquee)
	setIf(&a.Video, p.Video)
	setIf(&a.Thumbnail, p.Thumbnail)
	setIf(&a.Bezel, p.Bezel)
	setIf(&a.Manual, p.Manual)
	setIf(&a.BoxBack, p.BoxBack)
	setIf(&a.Fanart, p.Fanart)
}

func setIf[T any](dst *T, v *T) {
	if v != nil {
		*dst = *v
	}
}

func (p SettingsPatch) validate() error {
	if p.DownloadedGames != nil && !slices.Contains(config.DownloadedGamesModes(), *p.DownloadedGames) {
		return badSetting("unknown downloaded games mode %q", *p.DownloadedGames)
	}
	if p.LogLevel != nil && !slices.Contains(config.LogLevels(), *p.LogLevel) {
		return badSetting("unknown log level %q", *p.LogLevel)
	}
	if p.APITimeout != nil && (*p.APITimeout <= 0 || *p.APITimeout > maxAPITimeout) {
		return badSetting("api timeout %s must be above zero and at most %s", *p.APITimeout, maxAPITimeout)
	}
	if p.DownloadTimeout != nil && (*p.DownloadTimeout <= 0 || *p.DownloadTimeout > maxDownloadTimeout) {
		return badSetting("download timeout %s must be above zero and at most %s", *p.DownloadTimeout, maxDownloadTimeout)
	}
	if p.ArtKind != nil && !slices.Contains(config.CoverArtKinds(), *p.ArtKind) {
		return badSetting("unknown art kind %q", *p.ArtKind)
	}
	if p.DownloadSplashArt != nil && !slices.Contains(config.SplashArtKinds(), *p.DownloadSplashArt) {
		return badSetting("unknown splash art kind %q", *p.DownloadSplashArt)
	}
	if err := p.AdditionalDownloads.validate(); err != nil {
		return err
	}
	if p.PlatformOrder != nil {
		seen := map[string]bool{}
		for _, slug := range *p.PlatformOrder {
			if seen[slug] {
				return badSetting("platform %q appears twice in the platform order", slug)
			}
			seen[slug] = true
		}
	}
	return nil
}

func badSetting(format string, args ...any) error {
	return newError(CodeBadRequest, fmt.Errorf(format, args...))
}

func (p SettingsPatch) apply(c *config.Config) {
	if p.ShowRegularCollections != nil {
		c.ShowRegularCollections = *p.ShowRegularCollections
	}
	if p.ShowSmartCollections != nil {
		c.ShowSmartCollections = *p.ShowSmartCollections
	}
	if p.ShowVirtualCollections != nil {
		c.ShowVirtualCollections = *p.ShowVirtualCollections
	}
	if p.DownloadedGames != nil {
		c.DownloadedGames = *p.DownloadedGames
	}
	if p.APITimeout != nil {
		c.ApiTimeout = config.DurationSeconds(*p.APITimeout)
	}
	if p.DownloadTimeout != nil {
		c.DownloadTimeout = config.DurationSeconds(*p.DownloadTimeout)
	}
	if p.LogLevel != nil {
		c.LogLevel = *p.LogLevel
	}
	if p.PlatformOrder != nil {
		c.PlatformOrder = slices.Clone(*p.PlatformOrder)
	}
	setIf(&c.DownloadArt, p.DownloadArt)
	setIf(&c.UnzipDownloads, p.UnzipDownloads)
	setIf(&c.ArtKind, p.ArtKind)
	setIf(&c.DownloadArtScreenshotPreview, p.DownloadArtScreenshotPreview)
	setIf(&c.DownloadSplashArt, p.DownloadSplashArt)
	p.AdditionalDownloads.apply(&c.AdditionalDownloads)
	setIf(&c.GamelistOmitsRegion, p.GamelistOmitsRegion)
}

func (g *Grout) UpdateSettings(p SettingsPatch) (config.Config, error) {
	_, end, err := g.begin(context.Background())
	if err != nil {
		return config.Config{}, err
	}
	defer end()
	if err := p.validate(); err != nil {
		return config.Config{}, err
	}
	var saved *config.Config
	err = g.updateThen(func(c *config.Config) error {
		p.apply(c)
		saved = c
		return nil
	}, func(c *config.Config) {
		if p.LogLevel != nil && g.level != nil {
			g.level.Set(c.LogLevel.Slog())
		}
	})
	if err != nil {
		return config.Config{}, err
	}
	return *cloneConfig(saved), nil
}
