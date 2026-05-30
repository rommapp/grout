# Settings Reference

Press `X` from the main menu to access Settings.

!!! important
    **Kid Mode Impact:** When Kid Mode is enabled, the Settings screen is hidden. To access settings temporarily, press `L1` + `R1` + `Menu` during the Grout splash screen. See [Kid Mode](#kid-mode) for details.

---

## Main Settings

**Switch to API Token** - Shown only when using password authentication. Initiates the [pairing code flow](guide.md#pairing-code) to switch your connection to token-based authentication.

**General** - Opens a sub-menu for general display and download options.
See [General Settings](#general-settings) below.

**Collections** - Opens a sub-menu for configuring collection display options.
See [Collections Settings](#collections-settings) below.

**Directory Mappings** - Change which device directories are mapped to which RomM platforms.
See [Directory Mappings](#directory-mappings) below.

**Save Sync** - Opens a sub-menu for configuring save sync. See [Save Sync Settings](#save-sync-settings) below.

**Tools** - Opens a sub-menu for artwork management and parental controls. See [Tools](#tools) below.

**Advanced** - Opens a sub-menu for advanced configuration options. See [Advanced Settings](#advanced-settings) below.

**Grout Info** - View version information, build details, server connection info (including API token name and expiry
when using token authentication), and the GitHub repository QR code.

**Check for Updates** - Check for and install Grout updates.

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

## Directory Mappings

Opens the platform directory mapping screen, which is the same screen shown during initial setup. This lets you change
which device directories are mapped to which RomM platforms.

For each platform, you can select:

- **Skip** - Don't map this platform. Games from this platform won't be available to download.
- **Create {Directory Name}** - Create a new directory for this platform. Grout suggests directory names that match your
  custom firmware's expected structure.
- **/{Existing Directory}** - Map to an existing directory on your device.
- **Custom...** - Enter a custom folder name using the on-screen keyboard.

For detailed documentation on platform mapping, see the [User Guide](guide.md#platform-directory-mapping).

### Mappings Reference

Each CFW uses different folder naming conventions:

- [KNULLI](../platforms/knulli.md) - ES-DE style folder names (e.g., `gb`, `snes`, `psx`)
- [EmuDeck / ES-DE](../platforms/esde.md) - ES-DE style folder names (e.g., `gb`, `snes`, `psx`)
- [muOS](../platforms/muos.md) - Mixed short codes and descriptive names (e.g., `gb`, `Nintendo Game Boy`)
- [NextUI](../platforms/nextui.md) - Descriptive names with tags (e.g., `Game Boy (GB)`)
- [ROCKNIX](../platforms/rocknix.md) - ES-DE style folder names (e.g., `gb`, `snes`, `psx`)
- [Spruce](../platforms/spruce.md) - Uppercase short codes (e.g., `GB`, `SFC`, `PS`)
- [Batocera](../platforms/BATOCERA.md) - ES-DE style folder names (e.g., `gb`, `megadrive`, `psx`)

---

## Save Sync Settings

This sub-menu configures save synchronization. For complete save sync documentation, see the [Save Sync Guide](save-sync.md).

### Device Name

Register or rename this device with your RomM server. Each device needs a unique name so RomM can track which saves
belong to which device. Selecting this opens a keyboard to enter or change the device name.

### Save Sync Mappings

Configure which emulator save directory is used for each platform. This tells Grout where to find and place save files
on your device. You can override per-game mappings from the [Game Options](guide.md#game-options) screen.

### Save Backups

Controls how many backup copies of local saves are retained when a newer save is downloaded from the server:

- **5** / **10** / **15** - Keep the N most recent backups per game
- **No Limit** - Keep all backups (default)

Backups are stored in a `.backup/` directory within each platform's save directory.

---

## Tools

This sub-menu contains artwork management and parental controls.

### Download Missing Art

Scans all mapped platforms and downloads cover art for any games that don't already have cached artwork. Useful after
adding new games to your library.

Note that this artwork is only displayed within Grout's interface - it does not affect the artwork shown in your CFW's
game list.

### Kid Mode

Hides some of the more advanced features for a simplified experience. When enabled, Kid Mode will hide:

- The Settings screen
- The Save Sync screen
- The Game Options screen
- The BIOS download screen

**Temporary Override:** You can temporarily disable Kid Mode for a single session by pressing `L1` + `R1` + `Menu` during the Grout splash screen.

**Permanent Disable:** Return to this menu and turn off Kid Mode.

---

## Advanced Settings

This sub-menu contains advanced configuration and system settings.

### Preload Artwork

Pre-cache artwork for all games across all mapped platforms. Grout scans your platforms, identifies
games without cached artwork, and downloads cover art from RomM. Useful for pre-caching after adding new games.

Note that this artwork is only displayed within Grout's interface - it does not affect the artwork shown in your CFW's game list.

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

### Server Address

Change the protocol, hostname, or port of your RomM server without logging out. Useful if your server's address
changes or you need to switch between HTTP and HTTPS.

### Release Channel

Controls which release channel Grout uses for updates:

- **Match RomM** - Automatically matches the release channel of your RomM server
- **Stable** - Only receive stable releases
- **Beta** - Receive beta releases for early access to new features

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
