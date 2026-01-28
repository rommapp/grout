# Grout User Guide

This guide walks you through using Grout to download games from your RomM instance.

## Button Reference

Grout attempts to be consistent with its controls. Here's a quick reference:

| Button       | Description                               |
|--------------|-------------------------------------------|
| `A`          | Confirm / Select                          |
| `B`          | Back / Cancel                             |
| `X`          | Secondary action (Search, Settings)       |
| `Y`          | Tertiary action (Save Sync, Game Options) |
| `Start`      | Confirm / Save settings                   |
| `Select`     | Toggle list mode (multi-select, reorder)  |
| `L1` / `R1`  | Shoulder buttons (used in button combos)  |
| `Menu`       | Help menu (also used in button combos)    |
| `Up/Down`    | Navigate lists                            |
| `Left/Right` | Cycle options / Jump pages in list        |

---

## Status Bar Icons

The status bar displays icons to indicate background activity. Here's what each icon means:

### Cache Sync Icons

These icons appear during the [background cache sync](#background-cache-sync) process:

| Icon                                                                                    | Description                       |
|-----------------------------------------------------------------------------------------|-----------------------------------|
| ![Database Sync](../.github/resources/user_guide/statusbar_icons/Database%20Sync.png)   | Cache sync in progress            |
| ![Database Check](../.github/resources/user_guide/statusbar_icons/Database%20Check.png) | Cache sync completed successfully |
| ![Database Alert](../.github/resources/user_guide/statusbar_icons/Database%20Alert.png) | Cache sync encountered an error   |

### Save Sync Icons

These icons appear during [Save Sync](#save-sync) operations:

| Icon                                                                                    | Description                                                         |
|-----------------------------------------------------------------------------------------|---------------------------------------------------------------------|
| ![Cloud Sync](../.github/resources/user_guide/statusbar_icons/Cloud%20Sync.png)         | Save sync in progress                                               |
| ![Cloud Upload](../.github/resources/user_guide/statusbar_icons/Cloud%20Upload.png)     | Uploading saves to RomM                                             |
| ![Cloud Download](../.github/resources/user_guide/statusbar_icons/Cloud%20Download.png) | Downloading saves from RomM                                         |
| ![Cloud Check](../.github/resources/user_guide/statusbar_icons/Cloud%20Check.png)       | Save sync completed successfully                                    |
| ![Cloud Alert](../.github/resources/user_guide/statusbar_icons/Cloud%20Alert.png)       | Save sync encountered an error, go check the log file for more info |

---

## Table of Contents

- [Status Bar Icons](#status-bar-icons)
- [First Launch and Login](#first-launch-and-login)
- [Platform Directory Mapping](#platform-directory-mapping)
- [Background Cache Sync](#background-cache-sync)
- [Main Menu - Platform Selection](#main-menu--platform-selection)
- [Collections](#collections)
- [Game List](#game-list)
- [Search](#search)
- [Game Details](#game-details)
- [Downloading Games](#downloading-games)
- [BIOS Files](#bios-files)
- [Settings](#settings)
- [Save Sync](#save-sync)
- [Platform Mappings Reference](#platform-mappings-reference)
- [Advanced Configuration](#advanced-configuration)

**See Also:**

- [Quick Start Guide](QUICK_START.md) - Get up and running in 5 steps
- [Settings Reference](SETTINGS.md) - Complete settings documentation
- [Save Sync Guide](SAVE_SYNC.md) - Detailed save synchronization guide
- [Platform Mappings](#platform-mappings-reference) - CFW-specific folder names

---

## First Launch and Login

### Language Selection

![Grout preview, language selection](../.github/resources/user_guide/language_selection.png "Grout preview, language selection")

When you first launch Grout, you'll be asked to select your preferred language. Grout supports:

- English
- Deutsch (German)
- Español (Spanish)
- Français (French)
- Italiano (Italian)
- Português (Portuguese)
- Русский (Russian)
- 日本語 (Japanese)

Use `Left/Right` to cycle through the available languages. Press `A` to confirm your selection.

You can change your language later from the [Settings](SETTINGS.md#language).

---

### Login

![Grout preview, login flow](../.github/resources/user_guide/login.png "Grout preview, login flow")

1. **Protocol** – Choose between HTTP and HTTPS.
2. **Hostname** – Enter your RomM server address without the protocol.
3. **Port (optional)** – If your RomM instance runs on a non-standard port, enter it here.
4. **Username** - Your RomM username.
5. **Password** - Your RomM password.
6. **SSL Certificates** (HTTPS only) – Choose whether to verify SSL certificates:
    - **Verify** – Validate SSL certificates (recommended)
    - **Skip Verification** – Skip SSL certificate validation (useful for self-signed certificates or internal CAs)

Use the left and right buttons to cycle through options for Protocol. For the text fields (Hostname, Username,
Password), pressing `A` will open an on-screen keyboard.

Press `Start` to login. If your credentials are correct and Grout can reach your server, you'll move
to the next step. If something goes wrong, you'll get a message telling you what happened, and you can try again.

> [!TIP]
> If you're using a self-signed certificate or a certificate from an internal Certificate Authority, set
> **SSL Certificates** to **Skip Verification** to avoid connection errors.

> [!IMPORTANT]
> **OIDC Users:** If your RomM instance uses OIDC authentication, you can still use Grout by setting a password for your
> user account. Grout will support API Keys once they are available in RomM. For more details,
> see [this GitHub issue](https://github.com/rommapp/romm/issues/1767#issuecomment-2744215262).

---

## Platform Directory Mapping

On this screen, you map your RomM platforms to directories on your device.
This tells Grout where to put the games that you download.

![Grout preview, platform mapping](../.github/resources/user_guide/platform_mapping.png "Grout preview, platform mapping")

You'll see a list of all platforms from your RomM instance. For each one, you can select:

- **Skip** – Don't map this platform. Games from this platform won't be available to download.
- **Create {Directory Name}** – Create a new directory for this platform. Grout will automatically suggest directory
  names that match your custom firmware's expected structure.
- **/{Existing Directory}** – Map to an existing directory on your device.

Grout tries to be smart about this. If you already have a directory that matches the platform name, it'll be
pre-selected. If not, it'll suggest creating one with the correct name for your firmware.

Use left/right to cycle through the options for each platform. When you're happy with your mappings, press `Start` to
save. Grout will create any new directories you selected.

You can change these mappings later from [Settings](SETTINGS.md).

---

## Background Cache Sync

> [!WARNING]
> Grout currently does not gracefully handle deletions.
>
> Deleted games, platforms and collections will continue to be shown until
> the [local cache is rebuilt](SETTINGS.md#rebuild-cache).
>
> We are waiting for updated API endpoints that will allow Grout to remove deleted items from the cache without a
> rebuild. You can track the progress of this in [this issue](https://github.com/rommapp/grout/issues/83).

Grout maintains a local cache of your RomM library data (platforms, games, and collections) to provide a fast,
responsive browsing experience. This cache syncs automatically in the background each time you launch Grout.

**How it works:**

- On startup, Grout begins syncing in the background while you can immediately start browsing
- A sync icon appears in the status bar during the sync process
- Grout uses incremental updates, only fetching data that has changed since your last session
- When complete, the sync icon updates to indicate success

**First launch:**

On your very first launch (after platform mapping), Grout builds the initial cache. This may take a moment
depending on the size of your library.

> [!TIP]
> If you need to completely rebuild the cache from scratch, use **Rebuild Cache** in
> [Advanced Settings](SETTINGS.md#rebuild-cache).

---

## Main Menu – Platform Selection

![Grout preview, main menu (platforms)](../.github/resources/user_guide/platforms.png "Grout preview, main menu (platforms)")

At the top, you'll see "Collections" (if you have any collections set up in RomM). Below that, you'll see all your RomM
platforms – NES, SNES, PlayStation, whatever you've got.

**Navigation:**

- `Up/Down` to scroll through platforms
- `A` to select a platform or collection
- `X` to open Settings
- `Y` to open the Save Sync menu (when Save Sync is enabled in Manual mode, or when issues occur in Automatic mode)
- `Select` to enter reordering mode
- `B` to quit Grout

**Reordering Platforms:**

![Grout preview, reordering platforms](../.github/resources/user_guide/reordering_platforms.png "Grout preview, reordering")

Press `Select` to enter reordering mode. `↕` will appear next to the currently selected platform.

While in this mode:

- `Up/Down` to move the platform one position
- `Left/Right` to move the platform one page at a time
- `A` to place the platform into its new position

Your custom platform order is automatically saved to the config and will persist across sessions.

---

## Collections

![Grout preview, collections list](../.github/resources/user_guide/collections.png "Grout preview, collections list")

Grout has two views for collections. You can choose this view in the Settings → Collections menu.

**Platform** – After selecting a collection, you'll see a platform selection screen showing all platforms in that
collection. Select a platform to view games from only that platform.

![Grout preview, collection content - platforms](../.github/resources/user_guide/collections_platforms.png "Grout preview, collection content - platform")

**Unified** – After selecting a collection, you'll immediately see all games from all platforms with platform slugs
shown as prefixes (e.g., `[nes] Tetris`, `[snes] Tetris Battle Gaiden`)

![Grout preview, collection content - unified](../.github/resources/user_guide/collections_unified.png "Grout preview, collection content - unified")

> [!WARNING]
> If you skipped a platform in the mapping screen, you won't see games for that platform in your collections.

> [!TIP]
> Regular collections, smart collections, and virtual collections can be toggled on/off
> in [Settings](SETTINGS.md#collections-settings).

---

## Game List

![Grout preview, games list](../.github/resources/user_guide/games_list.png "Grout preview, games list")

The title bar shows you where you are – either a platform name or a collection name.

If you entered a search query, you'll see `[Search: "your search term"] | Platform / Collection Name`.

![Grout preview, search results](../.github/resources/user_guide/search_results.png "Grout preview, search results")

**Navigation and Selection:**

- `Up/Down` to scroll through games
- `Left/Right` to skip entire pages
- `A` to select a single game
- `Select` to enter multi-select mode, then use `A` to select/deselect games
- `X` to open the search keyboard
- `B` to go back

**Multi-Select Mode:**
Press `Select` once to enable multi-select. You'll see checkboxes appear next to each game. Now when you press `A` on a
game, it toggles selection instead of immediately downloading. This is perfect when you want to grab a bunch of games at
once.

Check all the ones you want, then press `Start` to confirm your selections.

![Grout preview, games multi select](../.github/resources/user_guide/multi_select.png "Grout preview, games multi select")

> [!TIP]
> Box art must be enabled in [Settings](SETTINGS.md#box-art) for it to appear.

---

## Search

Press `X` from any game list to search.

![Grout preview, search](../.github/resources/user_guide/search.png "Grout preview, search")

Type your search term using the on-screen keyboard and confirm. The game list will filter to show only matching titles.
The search is case-insensitive and matches anywhere in the game name.

To clear a search and return to the full list, press `B`.

---

## Game Details

> [!TIP]
> This screen will only show if you've enabled "Game Details" in Settings.

![Grout preview, game details](../.github/resources/user_guide/game_details.png "Grout preview, game details")

You'll see:

- **Cover art** – The game's box art (if available)
- **File Version dropdown** – If the game has multiple file versions (like different regions or revisions), use this
  dropdown to select which version to download. Already-downloaded versions are marked with a download icon.
- **Summary** – A description of the game
- **Metadata** – Release date, genres, developers/publishers, game modes, regions, languages, and file size
- **Multi-file indicator** – If the game has multiple files (like multi-disc PlayStation games)
- **QR code** – Scan this to view the game's page on your RomM web interface

From here:

- `A` to download the game (or `X` if a file version dropdown is present)
- `Y` to open Game Options
- `B` to go back without downloading

### File Version Selection

Some games in RomM may have multiple file versions – for example, different regional releases (USA, Europe, Japan) or
different revisions (Rev A, Rev B). When a game has multiple versions available:

1. A **File Version** dropdown appears on the game details screen
2. Use `Up/Down` to scroll to the dropdown, then press `A` to expand it
3. Select the version you want to download
4. Versions you've already downloaded are marked with a download icon prefix
5. Press `X` to download the selected version

### Game Options

> [!IMPORTANT]
> **Kids Mode Impact:** When Kids Mode is enabled, the Game Options screen is hidden.
> See [Settings Reference](SETTINGS.md#kids-mode) to learn how to temporarily or permanently disable Kids Mode.

- **Save Directory** – Choose which emulator's save folder this game should use. This overrides the platform-wide
  setting configured in Save Sync Mappings. When changed, Grout automatically moves existing save files to the new
  location. This is useful when you use different emulators for specific games within the same platform.

---

## Downloading Games

After you've selected games (either from the game list or game details screen), the download manager kicks in.

![Grout preview, game download](../.github/resources/user_guide/download.png "Grout preview, game download")

You'll see a progress bar and a list of games being downloaded. Grout downloads your ROMs directly from RomM to the
appropriate directory on your device. Press `Y` to cancel the download.

**What Happens During Download:**

1. **ROM files are downloaded** – The game files are saved to the correct platform directory you mapped earlier.

2. **Multi-file games are extracted automatically** – If you're downloading a multi-disc game, Grout downloads a zip
   file, extracts it, and creates an M3U playlist file so your emulator can handle disc switching.

3. **Artwork is downloaded** – If "Download Art" is enabled in Settings, Grout downloads box art for each game to your
   artwork directory after the ROMs finish.

4. **Archived files are extracted automatically** – If "Archived Downloads" is set to "Uncompress" in Settings, Grout
   will extract zip and 7z files to the configured ROM directory and then delete the archive.

If a download fails, Grout will show you which games had problems and clean up any leftover cruft.

When everything's done, you're dropped back to the game list. The games you just downloaded are now on your device and
ready to play.

---

## BIOS Files

Many emulators require BIOS files to function properly. Grout can download these files directly from your RomM server to
the correct location on your device.

> [!IMPORTANT]
> **Kids Mode Impact:** When Kids Mode is enabled, the BIOS download screen is hidden.
> See [Settings Reference](SETTINGS.md#kids-mode) to learn how to temporarily or permanently disable Kids Mode.

![Grout preview, BIOS download](../.github/resources/user_guide/bios_download.png "Grout preview, BIOS download")

### Accessing BIOS Downloads

From the main menu, press `A` on a platform that requires BIOS files. If BIOS files are available for that
platform in your RomM library, you'll see a "BIOS" option in the platform submenu.

---

## Settings

Press `X` from the main menu to access Settings.

> [!IMPORTANT]
> **Kids Mode Impact:** When Kids Mode is enabled, the Settings screen is hidden entirely. To access settings
> temporarily, press `L1` + `R1` + `Menu` during the Grout splash screen.

![Grout preview, settings](../.github/resources/user_guide/settings.png "Grout preview, settings")

Settings are organized into several categories:

- **General** - Display options (box art, game details), download behavior, artwork, language
- **Collections** - Toggle collection types, choose unified vs. platform view
- **Directory Mappings** - Remap platforms to different folders
- **Save Sync** - Off, Manual, or Automatic sync modes
- **Advanced** - Cache management, timeouts, Kids Mode, logging

Use `Left/Right` to cycle through options. Press `Start` to save your changes, or `B` to cancel.

For complete documentation on all settings, see the **[Settings Reference](SETTINGS.md)**.

---

## Save Sync

Save Sync keeps your game saves synchronized between your RomM server and your handheld device.

![Grout preview, sync summary](../.github/resources/user_guide/sync_summary.png "Grout preview, sync summary")

**Quick Overview:**

- **Manual Mode** - Press `Y` from the main menu to sync on demand
- **Automatic Mode** - Syncs in the background when Grout launches

Save Sync compares your local saves with RomM and transfers whichever version is newer. Local saves are backed up before
being overwritten.

> [!WARNING]
> Save Sync works with save files only, **NOT** save states. If you use save states with autoload, the emulator may load
> the state instead of your synced save.

For complete documentation including sync logic, per-game settings, and troubleshooting, see the *
*[Save Sync Guide](SAVE_SYNC.md)**.

---

## Platform Mappings Reference

Grout uses platform mappings to determine where to save downloaded games on your device. Each Custom Firmware (CFW) uses
different folder naming conventions. Use these references to see the exact folder names used by your CFW:

- [KNULLI](platforms/KNULLI.md) - ES-DE style folder names (e.g., `gb`, `snes`, `psx`)
- [muOS](platforms/MUOS.md) - Mixed short codes and descriptive names (e.g., `gb`, `Nintendo Game Boy`)
- [NextUI](platforms/NEXTUI.md) - Descriptive names with tags (e.g., `Game Boy (GB)`)
- [ROCKNIX](platforms/ROCKNIX.md) - ES-DE style folder names (e.g., `gb`, `snes`, `psx`)
- [Spruce](platforms/SPRUCE.md) - Uppercase short codes (e.g., `GB`, `SFC`, `PS`)

