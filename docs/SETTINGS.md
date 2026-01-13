# Grout Settings Reference

Press `X` from the main menu to access Settings.

![Grout preview, settings](../.github/resources/user_guide/settings.png "Grout preview, settings")

> [!IMPORTANT]
> **Kids Mode Impact:** When Kids Mode is enabled, the Settings screen is hidden. To access settings temporarily, press `L1` + `R1` + `Menu` during the Grout splash screen. See [Kids Mode](#kids-mode) for details.

---

## Table of Contents

- [Main Settings](#main-settings)
- [General Settings](#general-settings)
- [Collections Settings](#collections-settings)
- [Advanced Settings](#advanced-settings)

---

## Main Settings

**General** - Opens a sub-menu for general display and download options.
See [General Settings](#general-settings) below.

**Collections** - Opens a sub-menu for configuring collection display options.
See [Collections Settings](#collections-settings) below.

**Directory Mappings** - Change which device directories are mapped to which RomM platforms. This takes you back to
the platform mapping screen that appeared during setup.

**Save Sync** - Controls save synchronization behavior:

- **Off** - Save sync is completely disabled
- **Manual** - Save sync is available via the `Y` button from the main menu
- **Automatic** - Grout automatically syncs saves in the background when you launch the app. A cloud icon in the status
  bar shows sync progress. If issues are detected, a `Y` button appears to access manual sync.

For complete save sync documentation, see the [Save Sync Guide](SAVE_SYNC.md).

**Save Sync Mappings** - Opens a sub-menu where you can configure the default save directory for each platform. This is
useful for platforms with multiple emulators (e.g., GBA on muOS), allowing you to set which emulator's save folder
should be used for syncing. Only visible when Save Sync is enabled. Individual games can override this setting via
Game Options.

![Grout preview, save sync mapping](../.github/resources/user_guide/sync_mappings.png "Grout preview, save sync mapping")

**Advanced** - Opens a sub-menu for advanced configuration options. See [Advanced Settings](#advanced-settings) below.

**Grout Info** - View version information, build details, server connection info, and the GitHub repository QR code.

**Check for Updates** - Will allow Grout to update itself. This feature is only present on muOS and Knulli as NextUI has
the Pak Store.

---

## General Settings

This sub-menu contains general display and download settings.

### Box Art

When set to show, Grout displays cover art thumbnails next to game names in the game list. Artwork is
automatically cached in the background as you browse. This provides a visual preview similar to your frontend's game
library view.

### Game Details

When enabled, selecting a game shows a detailed information screen with cover art, summary,
metadata, and game options before downloading. When disabled, selecting a game immediately starts the download.

> [!IMPORTANT]
> **Kids Mode Impact:** When Kids Mode is enabled, the Game Details screen is still accessible, but the Game Options button (`Y`) is hidden.

### Downloaded Games

Controls how already-downloaded games appear in game lists:

- **Do Nothing** - No special treatment for downloaded games
- **Mark** - Downloaded games are marked with a download icon
- **Filter** - Downloaded games are hidden from the list entirely

### Download Art

When enabled, Grout downloads box art for games after downloading the ROMs. The art goes into your
artwork directory so your frontend can display it.

### Archived Downloads

Controls what happens when downloading archived ROM files (zip and 7z):

- **Uncompress** - Grout automatically extracts archived ROMs after downloading. The archive is deleted after extraction.
- **Do Nothing** - Keep the downloaded archive as-is without extracting.

### Language

Grout is localized! Choose from English, Deutsch, Español, Français, Italiano, Português, Русский, or
日本語. If you notice an issue with a translation or want to help by translating, please let us know!

---

## Collections Settings

This sub-menu contains all collection-related configuration.

### Collections

When set to show, Grout displays regular collections in the main menu.

### Smart Collections

When set to show, Grout displays smart collections in the main menu.

### Virtual Collections

When set to show, Grout displays virtual collections in the main menu.

### Collection View

Controls how collections display their games:

- **Platform** - After selecting a collection, you'll see a platform selection screen showing all platforms in that
  collection. Select a platform to view games from only that platform.
- **Unified** - After selecting a collection, you'll immediately see all games from all platforms with platform slugs
  shown as prefixes (e.g., `[nes] Tecmo Bowl`, `[snes] Super Mario World`)

---

## Advanced Settings

This sub-menu contains advanced configuration and system settings.

### Preload Artwork

Pre-cache artwork for all games across all mapped platforms. Grout scans your platforms, identifies
games without cached artwork, and downloads cover art from RomM. Useful for pre-caching after adding new games.

### Refresh Cache

Re-sync cached data from RomM. Select which caches to refresh: Games Cache (platform and ROM data)
or Collections Cache. Shows when each cache was last refreshed.

### Download Timeout

How long Grout waits for a single ROM to download before giving up. Useful for large files or
slow connections. Options range from 15 to 120 minutes.

### API Timeout

How long Grout waits for responses from your RomM server before giving up. If you have a slow
connection or are a completionist with a heavily loaded server, increase this. Options range from 15 to 300 seconds.

### Kids Mode

Hides some of the more advanced settings for a simplified experience. When enabled, Kids Mode will hide:

- The Settings screen
- The BIOS download screen
- The Game Options screen

**Temporary Override:** You can temporarily disable Kids Mode for a single session by pressing `L1` + `R1` + `Menu` during the Grout splash screen.

**Permanent Disable:** Return to this menu and turn off Kids Mode.

### Log Level

Set to Debug if you're troubleshooting issues and want detailed logs. Otherwise, Error is fine.

---

## Saving Settings

Use `Left/Right` to cycle through options. Press `Start` to save your changes, or `B` to cancel.

---

## Related

- [User Guide](USER_GUIDE.md) - Complete feature documentation
- [Save Sync Guide](SAVE_SYNC.md) - Detailed save synchronization documentation
- [Quick Start Guide](QUICK_START.md) - Get up and running quickly
