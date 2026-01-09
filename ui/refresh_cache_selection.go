package ui

import (
	"errors"
	"fmt"
	"grout/cache"
	"time"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	icons "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

// RefreshCacheType represents the type of cache to refresh
type RefreshCacheType int

const (
	RefreshCacheGames RefreshCacheType = iota
	RefreshCacheCollections
)

type RefreshCacheOutput struct {
	SelectedTypes []RefreshCacheType
}

type RefreshCacheScreen struct{}

func NewRefreshCacheScreen() *RefreshCacheScreen {
	return &RefreshCacheScreen{}
}

type refreshCacheItem struct {
	name        string
	cacheType   RefreshCacheType
	hasCache    bool
	metaKey     string
	lastRefresh time.Time
}

func (s *RefreshCacheScreen) Draw() (ScreenResult[RefreshCacheOutput], error) {
	output := RefreshCacheOutput{}

	cm := cache.GetCacheManager()

	// Get last refresh times
	refreshTimes := make(map[string]time.Time)
	if cm != nil {
		refreshTimes = cm.GetAllRefreshTimes()
	}

	caches := []refreshCacheItem{
		{
			name:        i18n.Localize(&goi18n.Message{ID: "cache_games", Other: "Games Cache"}, nil),
			cacheType:   RefreshCacheGames,
			hasCache:    cm != nil && cm.HasCache(),
			metaKey:     cache.MetaKeyGamesRefreshedAt,
			lastRefresh: refreshTimes[cache.MetaKeyGamesRefreshedAt],
		},
		{
			name:        i18n.Localize(&goi18n.Message{ID: "cache_collections", Other: "Collections Cache"}, nil),
			cacheType:   RefreshCacheCollections,
			hasCache:    cm != nil && cm.HasCollections(),
			metaKey:     cache.MetaKeyCollectionsRefreshedAt,
			lastRefresh: refreshTimes[cache.MetaKeyCollectionsRefreshedAt],
		},
	}

	// Build menu items for caches that exist
	items := make([]gaba.MenuItem, 0)
	availableCaches := make([]refreshCacheItem, 0)
	for _, c := range caches {
		if c.hasCache {
			text := c.name
			if !c.lastRefresh.IsZero() {
				text = fmt.Sprintf("%s (%s)", c.name, formatRelativeTime(c.lastRefresh))
			}
			items = append(items, gaba.MenuItem{Text: text})
			availableCaches = append(availableCaches, c)
		}
	}

	if len(items) == 0 {
		// No caches to refresh
		return back(output), nil
	}

	options := gaba.DefaultListOptions(
		i18n.Localize(&goi18n.Message{ID: "refresh_cache_title", Other: "Refresh Cache"}, nil),
		items,
	)
	options.FooterHelpItems = []gaba.FooterHelpItem{
		FooterCancel(),
		{ButtonName: icons.Start, HelpText: i18n.Localize(&goi18n.Message{ID: "button_confirm", Other: "Confirm"}, nil), IsConfirmButton: true},
	}
	options.StartInMultiSelectMode = true
	options.StatusBar = StatusBar()
	options.SmallTitle = true

	result, err := gaba.List(options)

	if err != nil {
		if errors.Is(err, gaba.ErrCancelled) {
			return back(output), nil
		}
		return withCode(output, gaba.ExitCodeError), err
	}

	for _, idx := range result.Selected {
		if idx >= 0 && idx < len(availableCaches) {
			output.SelectedTypes = append(output.SelectedTypes, availableCaches[idx].cacheType)
		}
	}

	if len(output.SelectedTypes) == 0 {
		return back(output), nil
	}

	return success(output), nil
}

// formatRelativeTime formats a time as a human-readable relative string
func formatRelativeTime(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	switch {
	case diff < time.Minute:
		return i18n.Localize(&goi18n.Message{ID: "time_just_now", Other: "just now"}, nil)
	case diff < time.Hour:
		mins := int(diff.Minutes())
		if mins == 1 {
			return i18n.Localize(&goi18n.Message{ID: "time_1_minute_ago", Other: "1 min ago"}, nil)
		}
		return i18n.Localize(&goi18n.Message{ID: "time_minutes_ago", Other: "{{.Count}} mins ago"}, map[string]interface{}{"Count": mins})
	case diff < 24*time.Hour:
		hours := int(diff.Hours())
		if hours == 1 {
			return i18n.Localize(&goi18n.Message{ID: "time_1_hour_ago", Other: "1 hour ago"}, nil)
		}
		return i18n.Localize(&goi18n.Message{ID: "time_hours_ago", Other: "{{.Count}} hours ago"}, map[string]interface{}{"Count": hours})
	case diff < 7*24*time.Hour:
		days := int(diff.Hours() / 24)
		if days == 1 {
			return i18n.Localize(&goi18n.Message{ID: "time_1_day_ago", Other: "1 day ago"}, nil)
		}
		return i18n.Localize(&goi18n.Message{ID: "time_days_ago", Other: "{{.Count}} days ago"}, map[string]interface{}{"Count": days})
	default:
		return t.Format("Jan 2")
	}
}
