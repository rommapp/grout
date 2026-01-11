# Grout Save Sync Guide

Save Sync keeps your game saves synchronized between your RomM server and your handheld device.

---

## Table of Contents

- [Sync Modes](#sync-modes)
- [How It Works](#how-it-works)
- [Sync Logic](#sync-logic)
- [Sync Results](#sync-results)
- [Per-Game Save Directory](#per-game-save-directory)
- [Important Notes](#important-notes)

---

## Sync Modes

Grout offers two sync modes, configurable in [Settings](SETTINGS.md):

### Manual Mode

- Press `Y` from the main menu to access save sync
- You control when syncing happens
- A sync summary is displayed after completion

### Automatic Mode

- Grout automatically syncs saves in the background when you launch the app
- A cloud icon appears in the status bar showing sync progress:
    - **Cloud with up arrow** - Upload in progress
    - **Cloud with down arrow** - Download in progress
    - **Cloud with checkmark** - Sync completed successfully
    - **Cloud with an exclamation mark** - Something went wrong, check the logs

---

## How It Works

When you run Save Sync, Grout:

1. Scans your device for games and their save files
2. Matches them with corresponding ROMs in RomM
3. Compares local and remote save files
4. Syncs saves based on which version is newer

---

## Sync Logic

For each mapped ROM file found on your device, Grout determines what action to take:

### When RomM has no save

Your local save, if present, is uploaded to RomM (with timestamp appended to filename).

### When you have no local save

RomM's save, if present, is downloaded to your device.

### When both exist

The newer save (based on last modified time) determines the action:

- **If the local save is newer:** It is uploaded to RomM with the last modified timestamp appended to the filename
- **If the RomM save is newer:**
    - The current local save is backed up to `.backup/` within the platform's save directory
    - The RomM save is downloaded to your device

### When there's no matching ROM in RomM

The save file is reported as "unmatched" in the sync results.

> [!IMPORTANT]
> The filename of the save will match the local filename of the ROM. This will allow you to have a ROM file duplicated
> locally under two names to handle two different saves (e.g. a Nuzlocke run in Pokémon alongside a vanilla
> playthrough).

---

## Sync Results

![Grout preview, sync summary](../.github/resources/user_guide/sync_summary.png "Grout preview, sync summary")

After syncing, you'll see a summary showing:

- **Downloaded saves** - Saves transferred from RomM to your device
- **Uploaded saves** - Saves transferred from your device to RomM
- **Unmatched saves** - Local saves without corresponding ROMs in RomM
- **Errors** - Any problems that occurred during sync

---

## Per-Game Save Directory

Some platforms have multiple emulators available (e.g., GBA on muOS). By default, Grout uses the platform-wide save
directory configured in **Save Sync Mappings**.

You can override this for individual games:

1. Open the game in Game Details view
2. Press `Y` to open Game Options
3. Change the **Save Directory** setting

When you change this setting, Grout automatically moves existing save files to the new location.

> [!IMPORTANT]
> **Kids Mode Impact:** When Kids Mode is enabled, the Game Options screen is hidden. You won't be able to change
> per-game save directories while Kids Mode is active.

---

## Important Notes

### Save files only

Save Sync works with save files, **NOT** save states. Save states are emulator-specific snapshots that require both
sides to use the same emulator and sometimes even the same version.

### Syncs can be obscured by autoload

If you use save states with autoload enabled, the emulator will load the state instead of the save file. To use synced
saves:

- Use the in-game menu to reset the emulator, forcing the save to be used
- Disable autoload for save states
- Delete the save state after downloading a synced save

### User-specific

Saves are tied to your RomM user account. If you share your RomM account with others, your saves will also be shared.

---

## Related

- [Settings Reference](SETTINGS.md) - Configure Save Sync options
- [User Guide](USER_GUIDE.md) - Complete feature documentation
- [Quick Start Guide](QUICK_START.md) - Get up and running quickly
