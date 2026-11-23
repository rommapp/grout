package romm

import (
	"context"
	"fmt"
	"strings"

	"github.com/rommapp/grout/core/config"
)

type ArtSource string

const (
	SourceCover      ArtSource = "cover"
	SourceScreenshot ArtSource = "screenshot"
	SourceSplash     ArtSource = "splash"
	SourceMarquee    ArtSource = "marquee"
	SourceVideo      ArtSource = "video"
	SourceBezel      ArtSource = "bezel"
	SourceManual     ArtSource = "manual"
	SourceBoxback    ArtSource = "boxback"
	SourceFanart     ArtSource = "fanart"
)

type ArtSourceSpec struct {
	Source ArtSource
	Kind   config.ArtKind
}

type SSMetadata struct {
	BezelURL       string `json:"bezel_url"`
	Box2DURL       string `json:"box2d_url"`
	Box2DBackURL   string `json:"box2d_back_url"`
	Box3DURL       string `json:"box3d_url"`
	FanartURL      string `json:"fanart_url"`
	LogoURL        string `json:"logo_url"`
	ManualURL      string `json:"manual_url"`
	MarqueeURL     string `json:"marquee_url"`
	MiximageURL    string `json:"miximage_url"`
	MiximageV2URL  string `json:"miximage_v2_url"`
	ScreenshotURL  string `json:"screenshot_url"`
	TitleScreenURL string `json:"title_screen_url"`
	VideoURL       string `json:"video_url"`

	BezelPath       string `json:"bezel_path"`
	Box2DPath       string `json:"box2d_path"`
	Box2DBackPath   string `json:"box2d_back_path"`
	Box3DPath       string `json:"box3d_path"`
	FanartPath      string `json:"fanart_path"`
	LogoPath        string `json:"logo_path"`
	MarqueePath     string `json:"marquee_path"`
	MiximagePath    string `json:"miximage_path"`
	MiximageV2Path  string `json:"miximage_v2_path"`
	TitleScreenPath string `json:"title_screen_path"`
	VideoPath       string `json:"video_path"`
}

type UserScreenshot struct {
	URLPath string `json:"url_path"`
}

type RomDetail struct {
	Rom
	PathScreenshots   []string         `json:"path_screenshots"`
	MergedScreenshots []string         `json:"merged_screenshots"`
	UserScreenshots   []UserScreenshot `json:"user_screenshots"`
	PathManual        string           `json:"path_manual"`
	URLManual         string           `json:"url_manual"`
	SSMetadata        *SSMetadata      `json:"ss_metadata"`

	client *Client
}

func (c *Client) RomDetail(ctx context.Context, id int) (RomDetail, error) {
	var d RomDetail
	if err := c.getJSON(ctx, fmt.Sprintf("/api/roms/%d", id), nil, &d); err != nil {
		return RomDetail{}, err
	}
	d.client = c
	return d, nil
}

func (d RomDetail) resolve(p, fallback string) string {
	if d.client == nil {
		return ""
	}
	if p != "" {
		return d.client.AssetURL(p)
	}
	return strings.ReplaceAll(fallback, " ", "%20")
}

func (d RomDetail) ArtSource(src ArtSource, kind config.ArtKind) string {
	ss := d.SSMetadata
	if ss == nil {
		ss = &SSMetadata{}
	}
	switch src {
	case SourceCover:
		return d.cover(kind, ss)
	case SourceScreenshot:
		switch {
		case len(d.UserScreenshots) > 0:
			return d.resolve(d.UserScreenshots[0].URLPath, "")
		case len(d.MergedScreenshots) > 0:
			return d.resolve(d.MergedScreenshots[0], "")
		default:
			return d.resolve("", ss.ScreenshotURL)
		}
	case SourceSplash:
		switch kind {
		case config.ArtKindMarquee:
			return d.resolve(ss.MarqueePath, ss.MarqueeURL)
		case config.ArtKindTitle:
			return d.resolve("", ss.TitleScreenURL)
		}
	case SourceMarquee:
		switch kind {
		case config.ArtKindMarquee:
			return d.resolve(ss.MarqueePath, ss.MarqueeURL)
		case config.ArtKindLogo:
			return d.resolve(ss.LogoPath, ss.LogoURL)
		}
	case SourceVideo:
		return d.resolve(ss.VideoPath, ss.VideoURL)
	case SourceBezel:
		return d.resolve(ss.BezelPath, ss.BezelURL)
	case SourceManual:
		if d.HasManual {
			if u := d.resolve(d.PathManual, d.URLManual); u != "" {
				return u
			}
		}
		return d.resolve("", ss.ManualURL)
	case SourceBoxback:
		return d.resolve(ss.Box2DBackPath, ss.Box2DBackURL)
	case SourceFanart:
		return d.resolve(ss.FanartPath, ss.FanartURL)
	}
	return ""
}

func (d RomDetail) cover(kind config.ArtKind, ss *SSMetadata) string {
	var u string
	switch kind {
	case config.ArtKindBox2D:
		u = d.resolve("", ss.Box2DURL)
	case config.ArtKindBox3D:
		u = d.resolve(ss.Box3DPath, ss.Box3DURL)
	case config.ArtKindMixImage:
		u = d.resolve(ss.MiximagePath, ss.MiximageURL)
	case config.ArtKindMixImageV2:
		u = d.resolve(ss.MiximageV2Path, ss.MiximageV2URL)
	}
	if kind == config.ArtKindDefault || u == "" {
		switch {
		case d.PathCoverSmall != "":
			u = d.resolve(d.PathCoverSmall, "")
		case d.PathCoverLarge != "":
			u = d.resolve(d.PathCoverLarge, "")
		default:
			u = d.resolve("", d.URLCover)
		}
	}
	return u
}
