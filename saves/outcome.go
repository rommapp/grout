package saves

import (
	"fmt"
	"os"
	"strings"
	"time"

	"grout/romm"
)

// OutcomeKind is what a sync of one game came to.
type OutcomeKind int

const (
	// OutcomeInSync: RomM and the device hold the same save, which RomM's own
	// content hash confirms. This is the only outcome that means "up to date".
	OutcomeInSync OutcomeKind = iota
	// OutcomeNothingYet: neither the device nor RomM has a save for the game.
	OutcomeNothingYet
	// OutcomeNoAnswer: RomM gave no instructions for the game.
	OutcomeNoAnswer
	// OutcomeDownloaded: RomM's save was taken.
	OutcomeDownloaded
	// OutcomeUploaded: the device's save was sent to RomM.
	OutcomeUploaded
	// OutcomeOtherSlot: RomM's newest save is in a slot the game is not using.
	OutcomeOtherSlot
	// OutcomeLeftAlone: RomM's instruction was set aside for another reason.
	OutcomeLeftAlone
	// OutcomeNeedsYou: a conflict or a choice only the player can settle.
	OutcomeNeedsYou
	// OutcomeFailed: something went wrong.
	OutcomeFailed
)

// Outcome says what a sync of one game did, in lines short enough for a small
// screen. Doing nothing is never reported as being up to date unless it is.
type Outcome struct {
	Kind  OutcomeKind
	Lines []string
}

// Text is the outcome as one message.
func (o Outcome) Text() string { return strings.Join(o.Lines, "\n") }

// SyncFacts is what is known about one game's sync.
type SyncFacts struct {
	// Ops is everything RomM said about the game.
	Ops []romm.SyncOperationSchema
	// Dropped is the part of that which Grout did not act on.
	Dropped []DroppedOp
	// Held counts what needs the player: conflicts and slot choices.
	Held int
	// Report is what was done.
	Report SyncReport
	// Local is the game's saves on the device before the sync, and LocalWrote
	// when the newest was written, taken before the sync because a download or
	// an upload sets the file's time to RomM's.
	Local      []LocalSave
	LocalWrote time.Time
	// StateSetAside is whether the game's old resume state was put away.
	StateSetAside bool
}

// DescribeSync works out what a sync of one game amounted to.
func DescribeSync(f SyncFacts) Outcome {
	switch {
	case f.Report.Errors > 0 && f.Report.TimedOut == f.Report.Errors:
		return Outcome{OutcomeFailed, []string{
			"RomM took too long to",
			"answer.",
			"The save may not have",
			"arrived. Try again soon.",
		}}

	case f.Report.Errors > 0:
		return Outcome{OutcomeFailed, []string{
			"Sync failed.",
			fmt.Sprintf("%d step(s) went wrong.", f.Report.Errors),
			"See the Grout logs.",
		}}

	case f.Report.Downloaded > 0:
		lines := []string{"Took RomM's save."}
		for _, item := range f.Report.Items {
			if item.Action == ActionDownload && item.Success && item.RemoteSave != nil {
				lines = append(lines, "RomM's save: "+when(item.RemoteSave.UpdatedAt)+slotNote(item.RemoteSave.Slot))
				break
			}
		}
		if !f.LocalWrote.IsZero() {
			lines = append(lines, "Replaced this device's save", "from "+when(f.LocalWrote)+" (backed up).")
		} else {
			lines = append(lines, "This device had no save yet.")
		}
		if f.StateSetAside {
			lines = append(lines, "Old resume state set aside.")
		}
		return Outcome{OutcomeDownloaded, lines}

	case f.Report.Uploaded > 0:
		lines := []string{"Sent this device's save", "to RomM."}
		if !f.LocalWrote.IsZero() {
			lines = append(lines, "This device: "+when(f.LocalWrote))
		}
		for _, op := range f.Ops {
			if op.Action == "upload" && op.ServerUpdatedAt != nil {
				lines = append(lines, "RomM had: "+when(*op.ServerUpdatedAt))
				break
			}
		}
		return Outcome{OutcomeUploaded, lines}

	case f.Held > 0:
		return Outcome{OutcomeNeedsYou, []string{
			"This needs you.",
			"The save changed on both",
			"sides, or has several slots.",
			"Open Grout > Sync.",
		}}
	}

	if len(f.Dropped) > 0 {
		return describeDropped(f.Dropped[0])
	}

	if len(f.Ops) == 0 {
		if len(f.Local) == 0 {
			return Outcome{OutcomeNothingYet, []string{"No save for this game yet,", "on this device or on RomM."}}
		}
		return Outcome{OutcomeNoAnswer, []string{"RomM gave no instructions", "for this game."}}
	}

	// Every instruction was "nothing to do". Believe it only if the saves match.
	for _, op := range f.Ops {
		if op.Action != "no_op" {
			return Outcome{OutcomeLeftAlone, []string{"Nothing was done.", "RomM said: " + op.Action + ".", "See the Grout logs."}}
		}
	}
	for _, op := range f.Ops {
		if op.ServerContentHash == nil || len(f.Local) == 0 {
			continue
		}
		if hash, err := saveContentHash(f.Local[0]); err == nil && hash != *op.ServerContentHash {
			return Outcome{OutcomeLeftAlone, []string{
				"RomM says nothing to do,",
				"but the saves differ.",
				"See the Grout logs.",
			}}
		}
	}
	lines := []string{"Already in sync.", "RomM and this device hold", "the same save."}
	for _, op := range f.Ops {
		if op.ServerUpdatedAt != nil {
			lines = append(lines, "Saved "+when(*op.ServerUpdatedAt)+slotNote(op.Slot))
			break
		}
	}
	return Outcome{OutcomeInSync, lines}
}

func describeDropped(dropped DroppedOp) Outcome {
	switch dropped.Reason {
	case DropOtherSlot:
		return Outcome{OutcomeOtherSlot, []string{
			"Nothing changed.",
			"RomM's newest save is in",
			"slot '" + SlotName(dropped.Op.Slot) + "'" + whenIn(dropped.Op) + ".",
			"This device uses '" + dropped.LocalSlot + "'.",
			"Game Options: Save Slot,",
			"or Use RomM's Save.",
		}}
	case DropUnchangedSinceDownload:
		return Outcome{OutcomeLeftAlone, []string{
			"Nothing changed.",
			"RomM asked for an upload,",
			"but this save is what was",
			"last downloaded.",
			"Game Options: Upload Local",
			"Save sends it anyway.",
		}}
	case DropNotInstalled:
		return Outcome{OutcomeLeftAlone, []string{"Nothing changed.", "This game is not on", "this device."}}
	}
	return Outcome{OutcomeLeftAlone, []string{
		"Nothing changed.",
		"One of RomM's instructions",
		"was skipped. See the logs.",
	}}
}

// when is a time in a short form the device's screen can hold.
func when(t time.Time) string { return t.Local().Format("Jan 2 15:04") }

func whenIn(op romm.SyncOperationSchema) string {
	if op.ServerUpdatedAt == nil {
		return ""
	}
	return " (" + when(*op.ServerUpdatedAt) + ")"
}

func slotNote(slot *string) string {
	name := SlotName(slot)
	if name == "" || name == "autosave" {
		return ""
	}
	return ", slot '" + name + "'"
}

// NewestWrite is when the most recently written of the saves was written.
func NewestWrite(local []LocalSave) time.Time {
	var newest time.Time
	for _, save := range local {
		if info, err := os.Stat(save.FilePath); err == nil && info.ModTime().After(newest) {
			newest = info.ModTime()
		}
	}
	return newest
}
