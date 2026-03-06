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

- **Sync Now** - Run a manual save sync. If you've synced before, the last sync time is shown (e.g., "Sync Now · 2h ago")
- **Synced Games** - Browse games that have been synced on this device, grouped by platform. From here you can view save details and manage save slots for individual games.
- **View History** - See a chronological log of all sync actions (uploads, downloads) for this device

---

## How It Works

When you run Save Sync, Grout:

1. Checks server connectivity
2. Scans your device for games and their save files
3. Matches them with corresponding ROMs in RomM
4. Compares local and remote save files
5. Resolves any conflicts (prompting you if needed)
6. Syncs saves based on which version is newer

---

## ROM Matching

For Save Sync to work, Grout must match your local ROM files with ROMs in your RomM library. Grout uses several methods
to find matches, tried in order:

### 1. Filename Match

If the local ROM filename (without extension) exactly matches a ROM's filename in RomM, it's considered a match. This is
the fastest and most common matching method.

### 2. Hash Match

If filename matching fails, Grout can compute the CRC32 or SHA1 hash of your local ROM file and compare it against hashes
stored in RomM. This is useful when:

- Your local ROM has a different filename than in RomM
- You renamed a ROM locally, but it's the same file

### 3. Fuzzy Title Match

If both filename and hash matching fail, Grout attempts to match based on title similarity. This helps when ROM names
differ slightly between your device and RomM.

**What fuzzy matching handles:**

- **Accented characters** - "Pokemon Red" matches "Pokemon Red"
- **User-added suffixes** - "Pokemon Red Nuzlocke" matches "Pokemon Red Version" (both share "Pokemon Red" as a common
  prefix)
- **Naming convention differences** - "Pokemon - Red Version" matches "Pokemon Red"

When a potential match is found with at least **80% similarity**, Grout displays a confirmation prompt:

```
+-------------------------------------+
|  Potential Match Found              |
|                                     |
|  Local: "Pokemon Red Nuzlocke"      |
|  Match: "Pokemon Red Version"       |
|  Similarity: 85%                    |
|                                     |
|  Is this the same game?             |
|                                     |
|  [B] No    [X] Yes                  |
+-------------------------------------+
```

- Press `X` to confirm the match
- Press `B` to decline

**Confirmed matches are remembered** - once you confirm a fuzzy match, Grout saves the association and won't ask again
for that ROM. This is useful for maintaining separate saves (like a Nuzlocke run alongside a regular playthrough).

**Declined matches have a cooldown** - if you decline a fuzzy match, Grout won't prompt you again for 24 hours.

!!! tip
    When you refresh the games cache, any saved matches for ROMs that no longer exist in RomM are automatically cleaned up.

---

## Sync Logic

For each mapped ROM file found on your device, Grout determines what action to take:

### When RomM has no save

Your local save, if present, is uploaded to RomM (with timestamp appended to filename).

### When you have no local save

RomM's save, if present, is downloaded to your device.

### When both saves exist

The newer save (based on last modified time) determines the action:

- **If the local save is newer:** It is uploaded to RomM with the last modified timestamp appended to the filename
- **If the RomM save is newer:**
    - The current local save is backed up to `.backup/` within the platform's save directory
    - The RomM save is downloaded to your device

### Conflicts

When Grout detects that both the local and remote saves have changed since the last sync and neither is clearly newer,
it flags the item as a conflict. You'll be shown a conflict resolution screen where you can choose **Keep Local** or
**Keep Remote** for each conflicting save.

### No matching ROM in RomM

The save file is reported as "unmatched" in the sync results.

!!! important
    The filename of the save will match the local filename of the ROM. This will allow you to have a ROM file duplicated
    locally under two names to handle two different saves (e.g. a Nuzlocke run in Pokemon alongside a vanilla
    playthrough).

---

## Save Slots

Save slots let you manage multiple save versions for a single game on RomM. By default, all saves use the "default" slot, but you can create named slots and switch between them.

### Switching Slots

You can change which slot a game syncs to from two places:

- **Game Options** - From the game details screen, press `Y` to open Game Options. The **Save Slot** setting shows all available slots plus a **New Slot...** option.
- **Synced Games** - From the Sync Menu, open **Synced Games**, navigate to a game, and press `Y` on the detail screen to change the save slot.

Use `Left/Right` to cycle through available slots. Changing to a different slot triggers a sync automatically.

### Creating a New Slot

Select **New Slot...** from the slot selector to create a new named slot. An on-screen keyboard will appear where you can type the slot name. When you confirm, Grout immediately uploads your local saves to the new slot on RomM.

### Multi-Slot Downloads

When downloading saves for the first time from a game that has multiple slots on RomM, Grout shows a slot selection screen. This lets you choose which slot to download from. The game name is prefixed with its platform slug (e.g., `[gba] Pokemon Emerald`) so you can identify games across platforms.

### Slot Preferences

Your slot preferences are stored in `save_slots.json`, separate from the main config file. Each entry maps a ROM ID to the preferred slot name. Games without a preference default to the "default" slot.

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

## Per-Game Save Directory

Some platforms have multiple emulators available (e.g., GBA on muOS). By default, Grout uses the platform-wide save
directory configured in **Save Sync Mappings**.

You can override this for individual games:

1. Open the game in Game Details view
2. Press `Y` to open Game Options
3. Change the **Save Directory** setting

When you change this setting, Grout automatically moves existing save files to the new location.

!!! important
    **Kids Mode Impact:** When Kids Mode is enabled, the Game Options screen is hidden. You won't be able to change
    per-game save directories while Kids Mode is active.

---

## Important Notes

### Save files only

Save Sync works with save files, **NOT** save states. Save states are emulator-specific snapshots that require both
sides to use the same emulator and sometimes even the same version.

### Supported save formats

Grout supports a wide range of save file extensions: `.srm`, `.sav`, `.dsv`, `.mcr`, `.mcd`, `.brm`, `.eep`, `.sra`,
`.fla`, `.mpk`, and `.nv`.

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
