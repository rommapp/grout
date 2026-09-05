package saves

import (
	"testing"
	"time"

	"grout/cache"
)

func record(romID int, name, action string, at time.Time) cache.SaveSyncRecord {
	return cache.SaveSyncRecord{RomID: romID, RomName: name, Action: action, SyncedAt: at}
}

// The newest sync is the one the user is looking for, so it is at the top of
// the newest day.
func TestGroupByDay_NewestFirst(t *testing.T) {
	morning := time.Date(2026, 9, 3, 9, 0, 0, 0, time.Local)
	evening := time.Date(2026, 9, 3, 21, 0, 0, 0, time.Local)
	nextDay := time.Date(2026, 9, 4, 10, 0, 0, 0, time.Local)

	days := groupByDay([]cache.SaveSyncRecord{
		record(1, "Mario", "upload", morning),
		record(2, "Zelda", "download", nextDay),
		record(3, "Sonic", "upload", evening),
	}, nil)

	if len(days) != 2 {
		t.Fatalf("got %d days, want one per calendar day", len(days))
	}
	if !days[0].Date.After(days[1].Date) {
		t.Error("days are not newest first")
	}
	if days[1].Events[0].RomName != "Sonic" {
		t.Errorf("first event of the older day is %q, want the later of the two", days[1].Events[0].RomName)
	}
}

// A sync just after midnight belongs to the night the user remembers, not to
// the day the stored UTC timestamp happens to fall on.
func TestGroupByDay_SplitsOnTheLocalDay(t *testing.T) {
	lateNight := time.Date(2026, 9, 3, 23, 30, 0, 0, time.Local)
	afterMidnight := lateNight.Add(time.Hour)

	days := groupByDay([]cache.SaveSyncRecord{
		record(1, "Mario", "upload", afterMidnight),
		record(2, "Zelda", "upload", lateNight),
	}, nil)

	if len(days) != 2 {
		t.Fatalf("got %d days, want the two sides of midnight apart", len(days))
	}
}

// Two syncs in the same minute used to sort by name because the time was
// compared as "15:04". The real timestamp orders them.
func TestGroupByDay_OrdersWithinTheMinute(t *testing.T) {
	// The later sync also sorts later by name, so a comparison that cannot see
	// past the minute falls back to the name and puts the wrong one first.
	earlier := time.Date(2026, 9, 4, 20, 0, 10, 0, time.Local)
	later := time.Date(2026, 9, 4, 20, 0, 40, 0, time.Local)

	days := groupByDay([]cache.SaveSyncRecord{
		record(1, "Mario", "upload", earlier),
		record(2, "Zelda", "upload", later),
	}, nil)

	if got := days[0].Events[0].RomName; got != "Zelda" {
		t.Errorf("first event is %q, want the later sync even though it sorts second by name", got)
	}
}

// Two saves recorded at the same instant have nothing to order them by but
// their names, which at least keeps the list stable between visits.
func TestGroupByDay_TiesGoByName(t *testing.T) {
	at := time.Date(2026, 9, 4, 20, 0, 0, 0, time.Local)

	days := groupByDay([]cache.SaveSyncRecord{
		record(1, "Zelda", "upload", at),
		record(2, "Mario", "upload", at),
	}, nil)

	if got := days[0].Events[0].RomName; got != "Mario" {
		t.Errorf("first event is %q, want them alphabetical when the times match", got)
	}
}

// Records arrive newest first from the cache, but the grouping must not lean
// on that: a day that appeared late would otherwise be filed in the wrong
// place.
func TestGroupByDay_DoesNotRelyOnTheInputOrder(t *testing.T) {
	older := time.Date(2026, 9, 2, 12, 0, 0, 0, time.Local)
	newer := time.Date(2026, 9, 4, 12, 0, 0, 0, time.Local)

	days := groupByDay([]cache.SaveSyncRecord{
		record(1, "Mario", "upload", older),
		record(2, "Zelda", "upload", newer),
	}, nil)

	if !days[0].Date.After(days[1].Date) {
		t.Error("oldest-first input produced oldest-first days")
	}
}

func TestGroupByDay_CarriesThePlatform(t *testing.T) {
	at := time.Date(2026, 9, 4, 20, 0, 0, 0, time.Local)

	days := groupByDay([]cache.SaveSyncRecord{record(7, "Mario", "upload", at)},
		map[int]string{7: "Super Nintendo"})

	if got := days[0].Events[0].Platform; got != "Super Nintendo" {
		t.Errorf("platform = %q, want the name the user knows", got)
	}
}

// A game the cache has never heard of still gets a row, with the column blank
// rather than the whole entry missing.
func TestGroupByDay_UnknownPlatform(t *testing.T) {
	at := time.Date(2026, 9, 4, 20, 0, 0, 0, time.Local)

	days := groupByDay([]cache.SaveSyncRecord{record(7, "Mario", "upload", at)}, nil)

	if len(days) != 1 || len(days[0].Events) != 1 {
		t.Fatal("the event was dropped along with its unknown platform")
	}
	if days[0].Events[0].Platform != "" {
		t.Errorf("platform = %q, want it left blank", days[0].Events[0].Platform)
	}
}

func TestGroupByDay_Empty(t *testing.T) {
	if days := groupByDay(nil, nil); len(days) != 0 {
		t.Errorf("got %d days for no records", len(days))
	}
}
