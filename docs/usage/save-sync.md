# Save Sync Guide

Save Sync keeps your game saves synchronized between your RomM server and your handheld device.

---

## Getting Started

Save Sync requires a registered device. The first time you open **Save Sync** from Settings, you'll be prompted to
enter a device name. This registers your handheld with your RomM server so it can track which saves belong to which
device.

Once registered, the **Sync** button (`Y`) appears on the main menu, giving you quick access to the Sync Menu.

---

## Sync Menu

Press `Y` from the main menu to open the Sync Menu. It contains:

- **Sync Now** - Run a manual save sync
- **Synced Games** - Browse games that have been synced on this device, grouped by platform. From here you can view save details and manage save slots for individual games.
- **View History** - See a chronological log of all sync actions (uploads, downloads) for this device

---

## How It Works

When you run Save Sync, Grout:

1. Checks server connectivity
2. Scans your device for games and their save files
3. Matches them with corresponding ROMs in RomM
4. Hashes each save and sends the list to the server's sync orchestrator
5. Executes the plan the server returns (uploads, downloads, conflicts)
6. Prompts you to resolve any conflicts

> [!NOTE]
> Save downloads only apply to games present on your device. If a ROM exists in your RomM library but is not on
> the device, its saves will not be downloaded. Local saves are still uploaded even if the ROM file itself has
> been removed from the device.

---

## ROM Matching

For Save Sync to work, Grout must match your local save files with ROMs in your RomM library.

Grout matches by **platform and filename** - the local save filename (without extension) is matched against the
filenames of the ROMs in your RomM library for the same platform. Any of a multi-file game's alternative versions
counts as a match, and when no exact match is found Grout falls back to a forgiving case-insensitive comparison.
PSP saves, which live in directories, are matched by their Game ID instead of filename.

> [!TIP]
> For the best experience, keep your local ROM filenames consistent with the names in your RomM library.

---

## Sync Logic

Sync decisions are made by your RomM server (RomM 4.9+), not on the device. Grout
hashes each local save and sends the list to the server's sync orchestrator, which
compares it against the server's record of this device and returns a plan: for each
save an **upload**, a **download**, a **conflict**, or nothing to do.

### Uploads

Local saves the server doesn't have yet — or whose content changed since the last
sync — are uploaded. If a local save is byte-identical to what was previously
downloaded from the server, the upload is skipped: the content is already there.
The server timestamps stored saves; your local filenames are left untouched.

### Downloads

When the server has a newer save for this device:

- The current local save is backed up to `.backup/` within the platform's save directory
- If the backup fails, the download is aborted to protect your local save
- The RomM save is downloaded to your device

Downloads only apply to games actually present on the device. On a fresh install
(or after reflashing your SD card), Grout additionally asks the server for saves of
games that have no local save yet, so your saves come back automatically.

### Conflicts

When both the local and the server save have changed since the last sync, the item
is flagged as a conflict. You'll be shown a conflict resolution screen where each
item defaults to **Skip** — nothing is overwritten unless you actively choose
**Keep Local** or **Keep Remote**.

Use `Left/Right` to choose a resolution for each game, then press `Start` to apply.
Pressing `B` cancels — unresolved conflicts are offered again on the next sync.

### No matching ROM in RomM

Save files that can't be matched to a ROM in your RomM library are skipped.

> [!IMPORTANT]
> The filename of the save will match the local filename of the ROM. This will allow you to have a ROM file duplicated
> locally under two names to handle two different saves (e.g. a Nuzlocke run in Pokemon alongside a vanilla
> playthrough).

---

## Save Slots

Save slots let you manage multiple save versions for a single game on RomM. By default, all saves use the "autosave" slot, but you can create named slots and switch between them.

### Switching Slots

You can change which slot a game syncs to from two places:

- **Game Options** - From the game details screen, press `Y` to open Game Options. The **Save Slot** setting shows all available slots plus a **New Slot...** option.
- **Synced Games** - From the Sync Menu, open **Synced Games**, navigate to a game, and press `Y` on the detail screen to change the save slot.

Use `Left/Right` to cycle through available slots. Changing to a different slot triggers a sync automatically.

### Creating a New Slot

Select **New Slot...** from the slot selector to create a new named slot. An on-screen keyboard will appear where you can type the slot name. When you confirm, Grout immediately uploads your local saves to the new slot on RomM.

### Multi-Slot Downloads

When downloading saves for the first time from a game that has multiple slots on RomM, Grout shows a slot selection screen. This lets you choose which slot to download from. The game name is prefixed with its platform slug (e.g., `[gba] Pokemon Emerald`) so you can identify games across platforms.

Use `Left/Right` to pick a slot for each game, then press `Start` to confirm. Pressing `B` skips those downloads.

### Slot Preferences

Your slot preferences are stored in `save_slots.json`, separate from the main config file. Each entry maps a ROM ID to the preferred slot name. Games without a preference default to the "autosave" slot.

---

## Backup Retention

When Grout downloads a newer save from RomM, it backs up your current local save to a `.backup/` directory. You can
control how many backups are kept per game in **Settings > Save Sync > Save Backups**:

- **5** / **10** / **15** - Keep the N most recent backups per game, automatically deleting older ones
- **No Limit** - Keep all backups (default)

---

## Sync Results

After syncing, you'll see a summary showing:

- **Downloaded saves** - Saves transferred from RomM to your device
- **Uploaded saves** - Saves transferred from your device to RomM
- **Conflicts** - Saves that required manual conflict resolution
- **Errors** - Any problems that occurred during sync

---

## Save Directory Mapping

Some platforms have multiple emulators available (e.g., GBA on muOS), each with its
own save directory. Grout syncs one save directory per platform.

To choose which emulator's save directory a platform syncs with:

1. Open **Settings > Save Sync > Save Mapping**
2. Use `Left/Right` to pick the save directory for each platform
3. Press `Start` to save your selection

Only platforms with more than one emulator save directory are listed. Selecting the
default entry clears the override.

> [!IMPORTANT]
> **Kid Mode Impact:** When Kid Mode is enabled, the Settings and Game Options
> screens are hidden, so save mappings and save slots can't be changed while it
> is active.

---

## Important Notes

### Save files only

Save Sync works with save files, **NOT** save states. Save states are emulator-specific snapshots that require both
sides to use the same emulator and sometimes even the same version.

### Supported save formats

Grout supports a wide range of save file extensions: `.srm`, `.sav`, `.dsv`, `.mcr`, `.mcd`, `.brm`, `.eep`, `.sra`,
`.fla`, `.mpk`, and `.nv`.

We have this filter in place as some CFWs place the save files alongside the ROM files.

If you notice that a save that is not being synced has an extension not in this list, please [create an issue on GitHub](https://github.com/rommapp/grout/issues/new?template=bug-report.md).

### Syncs can be obscured by autoload { data-toc-label="Autoload Warning" }

If you use save states with autoload enabled, the emulator will load the state instead of the save file. To use synced
saves:

- Use the in-game menu to reset the emulator, forcing the save to be used
- Disable autoload for save states
- Delete the save state after downloading a synced save

### User-specific

Saves are tied to your RomM user account. If you share your RomM account with others, your saves will also be shared.

---

## Related

- [Settings Reference](settings.md) - Configure Save Sync options
- [User Guide](guide.md) - Complete feature documentation
- [Quick Start Guide](../getting-started/index.md) - Get up and running quickly
