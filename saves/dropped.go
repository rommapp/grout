package saves

import "grout/romm"

// DropReason is why Grout did not act on something RomM said to do.
type DropReason int

const (
	// DropOtherSlot: RomM offered a save in a slot this game is not using here.
	DropOtherSlot DropReason = iota
	// DropUnchangedSinceDownload: RomM asked for an upload of a save that is
	// byte-for-byte what was last downloaded, which Grout takes to be on RomM
	// already.
	DropUnchangedSinceDownload
	// DropNoLocalSave: RomM asked about a save the device does not have in that slot.
	DropNoLocalSave
	// DropNotInstalled: RomM offered a save for a game that is not on the device.
	DropNotInstalled
	// DropNoIdentity: RomM's answer did not say which save it meant.
	DropNoIdentity
)

// DroppedOp is one of RomM's instructions that Grout left alone. Doing nothing
// is sometimes right, but it must never be reported as being up to date.
type DroppedOp struct {
	Op     romm.SyncOperationSchema
	Reason DropReason
	// LocalSlot is the slot the device's save is in, for DropOtherSlot.
	LocalSlot string
}
