package sync

import "grout/romm"

type LocalSave struct {
	RomID           int
	RomName         string
	FSSlug          string
	FileName        string
	FilePath        string // Primary save path; for PSP, the first DATA directory
	EmulatorDir     string
	RomFileName     string
	IsDirectorySave bool     // True for platforms like PSP where saves are directories
	GameID          string   // PSP: game ID prefix (e.g. "UCUS98751")
	RelatedDirs     []string // PSP: full paths of all save directories for this game
}

type SyncAction int

const (
	ActionUpload SyncAction = iota
	ActionDownload
	ActionConflict
	ActionSkip
)

func (a SyncAction) String() string {
	switch a {
	case ActionUpload:
		return "upload"
	case ActionDownload:
		return "download"
	case ActionConflict:
		return "conflict"
	case ActionSkip:
		return "skip"
	default:
		return "unknown"
	}
}

type SyncItem struct {
	LocalSave      LocalSave
	RemoteSave     *romm.Save
	Action         SyncAction
	Success        bool
	ForceOverwrite bool
	TargetSlot     string      // Slot to upload to (from slot preference); used by upload()
	AvailableSlots []string    // Distinct slot names when multiple slots exist (first-time downloads)
	AllRemoteSaves []romm.Save // All remote saves for re-selection after slot pick
}

func (item *SyncItem) Resolve(action SyncAction) {
	item.Action = action
}

type SyncReport struct {
	Uploaded   int
	Downloaded int
	Conflicts  int
	Skipped    int
	Errors     int
	Items      []SyncItem
}
