package saves

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"grout/romm"
	"grout/settings"
)

// RomM was slow, not broken: the report says so, and says the save may not have
// arrived, instead of a bare failure.
func TestAnUploadRomMDoesNotAnswerInTimeIsReportedAsATimeout(t *testing.T) {
	root := t.TempDir()
	local := copyIn(t, root, 4679, "mGBA", "Game.srm", "progress", yesterday)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer server.Close()
	defer close(release)
	client := romm.NewClient(server.URL, romm.WithTimeout(80*time.Millisecond))

	report := ExecuteActions(client, &settings.Config{}, "device-1",
		[]SyncItem{{LocalSave: local, Action: ActionUpload, TargetSlot: "autosave"}}, nil)

	if report.Errors != 1 || report.TimedOut != 1 {
		t.Fatalf("errors=%d timedOut=%d, want 1 and 1", report.Errors, report.TimedOut)
	}
	said := DescribeSync(SyncFacts{Report: report}).Text()
	if want := "took too long"; !contains(said, want) {
		t.Errorf("did not say RomM was slow (%q):\n%s", want, said)
	}
}

func TestAFailureThatIsNotATimeoutIsNotCalledOne(t *testing.T) {
	root := t.TempDir()
	local := copyIn(t, root, 4679, "mGBA", "Game.srm", "progress", yesterday)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()

	report := ExecuteActions(romm.NewClient(server.URL), &settings.Config{}, "device-1",
		[]SyncItem{{LocalSave: local, Action: ActionUpload, TargetSlot: "autosave"}}, nil)

	if report.Errors != 1 || report.TimedOut != 0 {
		t.Errorf("errors=%d timedOut=%d, want 1 and 0", report.Errors, report.TimedOut)
	}
}

// Telling RomM a sync is over is a courtesy; a server that never answers it must
// not hold the player on a screen for the whole request timeout.
func TestTellingRomMTheSyncIsOverIsNotWaitedForLong(t *testing.T) {
	old := completeSessionTimeout
	completeSessionTimeout = 80 * time.Millisecond
	defer func() { completeSessionTimeout = old }()

	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer server.Close()
	defer close(release)
	client := romm.NewClient(server.URL, romm.WithTimeout(10*time.Second))

	start := time.Now()
	ExecuteSaveSync(client, &settings.Config{}, "device-1", nil, 130, nil)

	if took := time.Since(start); took > 2*time.Second {
		t.Errorf("waited %v for a session to be closed; the client allows 10s but this call should not", took)
	}
}

func TestWithTimeoutLeavesTheOriginalClientAlone(t *testing.T) {
	original := romm.NewClient("http://example.invalid", romm.WithTimeout(30*time.Second))
	short := original.WithTimeout(time.Second)

	if short == original {
		t.Error("returned the same client")
	}
	// A request through each reveals the limit it carries.
	hang := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer hang.Close()
	slow := romm.NewClient(hang.URL, romm.WithTimeout(30*time.Second))
	start := time.Now()
	_ = slow.WithTimeout(60 * time.Millisecond).ValidateConnection()
	if time.Since(start) > 3*time.Second {
		t.Error("the shortened client did not use its shorter limit")
	}
}

func contains(s, sub string) bool { return len(s) >= len(sub) && (indexOf(s, sub) >= 0) }

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// ---- identical copies

func TestExtraCopiesIdenticalToTheLiveOneAreTidiedAway(t *testing.T) {
	root := t.TempDir()
	gpsp := copyIn(t, root, 291, "gpSP", "Advance Wars.srm", "same save", yesterday)
	mgba := copyIn(t, root, 291, "mGBA", "Advance Wars.srm", "same save", yesterday)
	other := copyIn(t, root, 12, "gpSP", "Other.srm", "x", yesterday)

	kept, tidied := tidyIdenticalCopies([]LocalSave{gpsp, mgba, other}, func(int) string { return "gpSP" })

	if tidied != 1 {
		t.Fatalf("tidied %d copies, want 1", tidied)
	}
	if len(kept) != 2 || kept[0].FilePath != gpsp.FilePath || kept[1].FilePath != other.FilePath {
		t.Errorf("kept %+v", kept)
	}
	if _, err := os.Stat(mgba.FilePath); !os.IsNotExist(err) {
		t.Error("the extra copy is still where Grout would find it")
	}
	if data, _ := os.ReadFile(filepath.Join(root, "mGBA", ".backup", "Advance Wars [2026-11-09 20-15-30].srm")); string(data) != "same save" {
		t.Error("the extra copy was not kept in a backup")
	}
	if data, _ := os.ReadFile(gpsp.FilePath); string(data) != "same save" {
		t.Error("the live copy was touched")
	}
}

func TestCopiesThatDifferAreLeftForThePlayer(t *testing.T) {
	root := t.TempDir()
	gpsp := copyIn(t, root, 4679, "gpSP", "Game.srm", "latest from RomM", yesterday)
	mgba := copyIn(t, root, 4679, "mGBA", "Game.srm", "an older one", yesterday)

	kept, tidied := tidyIdenticalCopies([]LocalSave{gpsp, mgba}, func(int) string { return "mGBA" })

	if tidied != 0 || len(kept) != 2 {
		t.Errorf("tidied %d, kept %d: a game whose saves differ must be left alone", tidied, len(kept))
	}
	for _, path := range []string{gpsp.FilePath, mgba.FilePath} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("%s was moved", path)
		}
	}
}

func TestNothingIsTidiedWhenTheLiveFolderIsNotKnown(t *testing.T) {
	root := t.TempDir()
	a := copyIn(t, root, 1, "gpSP", "Game.srm", "same", yesterday)
	b := copyIn(t, root, 1, "mGBA", "Game.srm", "same", yesterday)

	if _, tidied := tidyIdenticalCopies([]LocalSave{a, b}, func(int) string { return "" }); tidied != 0 {
		t.Errorf("tidied %d copies without knowing which one is in use", tidied)
	}
	if _, tidied := tidyIdenticalCopies([]LocalSave{a, b}, func(int) string { return "Snes9x" }); tidied != 0 {
		t.Errorf("tidied %d copies although none is in the live folder", tidied)
	}
}

func TestThreeCopiesAreOnlyTidiedIfAllThreeAreTheSame(t *testing.T) {
	root := t.TempDir()
	a := copyIn(t, root, 373, "gpSP", "Metroid Fusion.srm", "same", yesterday)
	b := copyIn(t, root, 373, "mGBA", "Metroid Fusion.srm", "same", yesterday)
	c := copyIn(t, root, 373, "VBA", "Metroid Fusion.srm", "DIFFERENT", yesterday)

	if _, tidied := tidyIdenticalCopies([]LocalSave{a, b, c}, func(int) string { return "gpSP" }); tidied != 0 {
		t.Errorf("tidied %d copies although one of them differs", tidied)
	}
	if _, err := os.Stat(b.FilePath); err != nil {
		t.Error("an identical copy was moved although another copy differs")
	}
}
