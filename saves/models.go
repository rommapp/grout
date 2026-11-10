package saves

import "grout/romm"

type LocalSave struct {
	RomID           int
	RomName         string
	FSSlug          string
	FileName        string
	FilePath        string // Primary save path; for PSP, the first DATA directory
	EmulatorDir     string
	RomFileName     string
	RomPath         string   // Where the ROM is on the device, when known
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
	TargetSlot     string // Slot to upload to (from slot preference); used by upload()
	TimedOut       bool   // Set when RomM did not answer in time
	// UploadOnly means this item may send the save but must never replace it. A
	// server that turns an upload down because its own save is ahead would
	// otherwise have the save on the device downloaded over.
	UploadOnly bool
	// AfterWrite, when set, runs right after a downloaded save has been written
	// to the card and before anything else is asked of the server, so whatever
	// has to follow the new save follows it even if the sync is stopped there.
	AfterWrite     func()
	AvailableSlots []string    // Distinct slot names when multiple slots exist (first-time downloads)
	AllRemoteSaves []romm.Save // All remote saves for re-selection after slot pick
}

func (item *SyncItem) Resolve(action SyncAction) {
	item.Action = action
}

type SyncResult struct {
	Items     []SyncItem
	SessionID int
	// Duplicates are the games left out because their save is in more than one
	// emulator's folder.
	Duplicates []DuplicateSaves
	// Operations is everything RomM said, and Dropped the part of it that Grout
	// did not act on. Together they are what a sync can honestly report.
	Operations []romm.SyncOperationSchema
	Dropped    []DroppedOp
}

type SyncReport struct {
	Uploaded   int
	Downloaded int
	Conflicts  int
	Skipped    int
	Errors     int
	// TimedOut counts the errors that were RomM not answering in time.
	TimedOut int
	Items    []SyncItem
}
