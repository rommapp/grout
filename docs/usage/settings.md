# Settings Reference

Press `X` from the main menu to access Settings.

!!! important
    **Kids Mode Impact:** When Kids Mode is enabled, the Settings screen is hidden. To access settings temporarily, press `L1` + `R1` + `Menu` during the Grout splash screen. See [Kids Mode](#kids-mode) for details.

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

For complete save sync documentation, see the [Save Sync Guide](save-sync.md).

**Save Sync Settings** - Opens a sub-menu where you can configure the default save directory for each platform. This is
useful for platforms with multiple emulators (e.g., GBA on muOS), allowing you to set which emulator's save folder
should be used for syncing. Only visible when Save Sync is enabled. Individual games can override this setting via
Game Options.

**Advanced** - Opens a sub-menu for advanced configuration options. See [Advanced Settings](#advanced-settings) below.

**Grout Info** - View version information, build details, server connection info, and the GitHub repository QR code.

**Check for Updates** - Will allow Grout to update itself.

---

## General Settings

This sub-menu contains general display and download settings.

### Box Art

When set to show, Grout displays cover art thumbnails next to game names in the game list. Artwork is
automatically cached in the background as you browse. This provides a visual preview similar to your frontend's game
library view.

### Downloaded Games

Controls how already-downloaded games appear in game lists:

- **Do Nothing** - No special treatment for downloaded games
- **Mark** - Downloaded games are marked with a download icon
- **Filter** - Downloaded games are hidden from the list entirely

### Download Art

When enabled, Grout downloads box art for games after downloading the ROMs. The art goes into your
artwork directory so your frontend can display it.

### Download Art Kind

Controls which type of artwork is downloaded when Download Art is enabled. This option is only visible when
Download Art is set to True.

- **Default** - Uses the default artwork provided by RomM
- **Box2D** - 2D box art scans
- **Box3D** - 3D box art renders
- **MixImage** - Composite mix images combining multiple artwork types

### Archived Downloads

Controls what happens when downloading archived ROM files (zip and 7z):

- **Uncompress** - Grout automatically extracts archived ROMs after downloading. The archive is deleted after extraction.
- **Do Nothing** - Keep the downloaded archive as-is without extracting.

### Language

Grout is localized! Choose from English, Deutsch, Espanol, Francais, Italiano, Portugues, Russian, or
Japanese. If you notice an issue with a translation or want to help by translating, please let us know!

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

### Rebuild Cache

Completely rebuilds the local cache from scratch. This deletes the SQLite database and re-downloads all platform
and game data from RomM. Use this if you're experiencing cache issues or want a clean slate.

!!! note
    Under normal operation, you shouldn't need to use this. Grout automatically syncs the cache in the background
    each time you launch the app, using incremental updates to only fetch data that has changed since the last sync.
    A sync icon appears in the status bar during this process.

### Download Timeout

How long Grout waits for a single ROM to download before giving up. Useful for large files or
slow connections. Options range from 15 to 120 minutes.

### API Timeout

How long Grout waits for responses from your RomM server before giving up. If you have a slow
connection or are a completionist with a heavily loaded server, increase this. Options range from 15 to 300 seconds.

### Release Channel

Controls which release channel Grout uses for updates:

- **Match RomM** - Automatically matches the release channel of your RomM server
- **Stable** - Only receive stable releases
- **Beta** - Receive beta releases for early access to new features

### Kids Mode

Hides some of the more advanced settings for a simplified experience. When enabled, Kids Mode will hide:

- The Settings screen
- The BIOS download screen
- The Game Options screen

**Temporary Override:** You can temporarily disable Kids Mode for a single session by pressing `L1` + `R1` + `Menu` during the Grout splash screen.

**Permanent Disable:** Return to this menu and turn off Kids Mode.

### Log Level

Controls the verbosity of Grout's log output:

- **Debug** - Maximum detail, useful for troubleshooting issues
- **Info** - Standard logging with sync completion summaries and key events
- **Error** - Only log errors

---

## Saving Settings

Use `Left/Right` to cycle through options. Press `Start` to save your changes, or `B` to cancel.

---

## Related

- [User Guide](guide.md) - Complete feature documentation
- [Save Sync Guide](save-sync.md) - Detailed save synchronization documentation
- [Quick Start Guide](../getting-started/index.md) - Get up and running quickly
