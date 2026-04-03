# v4.8.0.0
## New Platform Support
- **Spruce Universal Distribution**: Single zip with both ARM32 and ARM64 binaries, supporting A30, Miyoo Mini Flip, Miyoo Flip, TrimUI Brick, and TrimUI Smart Pro. Per-device input mappings and A30 screen rotation. - @pawndev
- **MinUI Universal Distribution**: Single zip with both ARM32 and ARM64 binaries, supporting Miyoo Mini, Miyoo Mini Plus, Miyoo A30, Miyoo Flip, MagicX Zero28, TrimUI, and Anbernic devices. Anbernic devices auto-detected via device tree - @BrandonKowalski & @pawndev
- **Allium and Onion OS Support**: ARM32 support for Miyoo Mini devices
- **Batocera x86 and AMD64**: Added x86 (32-bit) and AMD64 (64-bit) Batocera builds alongside existing ARM64
## In-App Updater Overhaul
- **Full Distribution Updates**: The updater now replaces the entire install (binary, launch scripts, shared libraries) instead of just the binary. Uses a staging directory so the update is applied cleanly on next launch
- **SHA256 Integrity Verification**: Downloaded updates are verified against checksums published in the versions file
- **Static Versions File**: Update checks now fetch a static JSON file from GitHub Pages instead of querying the GitHub API, eliminating rate limits and pagination issues
- **Fixed Beta Version Comparison**: Prerelease identifiers are now compared numerically (beta.10 is correctly newer than beta.6)
## EmulationStation Integration
- **Artwork Downloads**: Download game marquee, logo, bezel, fanart, box back, thumbnail, video, and manual artwork for EmulationStation-based CFWs (Knulli, ROCKNIX, Batocera) - @pawndev
- **Gamelist.xml Integration**: Downloaded artwork is registered in gamelist.xml for Knulli, ROCKNIX, and Batocera - @pawndev
## PSP Save Sync
- **PSP Directory Save Sync**: Full directory-based save sync for PSP with zip upload/download
- **PARAM.SFO Parser**: Replaced static GameDB with a PARAM.SFO parser for accurate PSP save title resolution
## Authentication
- **API Token Authentication**: New pairing code authentication flow as an alternative to username/password
## Performance
- **Miyoo Memory Optimization**: Streaming architecture for cache population prevents OOM crashes on 128MB Miyoo devices. Processes one platform at a time with aggressive GC between platforms - *first contribution by @ljtilley*
- **SQLite Tuning**: Reduced SQLite cache from unbounded to 4MB, temporary tables stored on disk instead of RAM - @ljtilley
- **Batched Database Inserts**: Mega-batch junction table inserts with lookup ID caching for faster cache rebuilds - @ljtilley
- **Pointer Receivers**: Rom methods changed from value to pointer receivers, reducing memory copies during UI rendering
## Bug Fixes
- Fixed save directories for PSX and PSP on muOS - @klinkertinlegs
- Corrected muOS emulator save directory names across all platforms - @klinkertinlegs
- Fixed various save data locations across multiple platforms - @klinkertinlegs
- Updated Spruce save directories to match available emulators and directories - @ljtilley
- Updated Spruce platforms.json to reflect all supported systems - @ljtilley
- Fixed Onion and Allium save directory mappings (#170)
- Fixed image paths with special characters in EmulationStation artwork - @pawndev
- Fixed Knulli continually restarting Grout when Quick Resume is enabled
- Fixed release artifact paths for non-ARM64 distributions
- Fixed Anbernic input mappings for MinUI - @klinkertinlegs
- Fixed default GBA platform on MinUI to use gpsp
## i18n
- Translations updated. Thanks @claude! :rofl:
## Contributors
Thanks to @pawndev, @ljtilley, @klinkertinlegs, and @from-nibly for their contributions to this release!

Shout out to @ljtilley and @klinkertinlegs for their first contributions!

# v4.7.0.0

## New Features

- **Save Sync Rebuilt**: Save Sync has been completely rebuilt from the ground up with device-aware syncing powered by RomM's new save sync API (#135)
  - **Conflict Resolution**: New conflict resolution screen when both local and remote saves have changed since last sync
  - **Save Slots**: User-configurable save slots per game, with slot selection during first sync and manageable from the Synced Games screen
  - **Backup Retention**: Configurable backup retention policy for save files (keep all, last 5, 10, or 15)
  - **Sync History**: View a chronological log (stored locally) of all sync actions (uploads, downloads) for this device
  - **Synced Games Browser**: Browse all synced games grouped by platform, view save details, and manage save slots
  - **Sync Menu**: New dedicated sync menu accessible via `Y` from the main menu
  - **Device Registration**: Device registration is performed when setting up save sync, can change device name anytime in settings (#134)
- **Batocera Support**: Preliminary support for Batocera (#157) - @from-nibly
- **Server Settings Reconfiguration**: Reconfigure server host, port, protocol, and SSL verification without logging out (#146)
- **Flip Face Buttons**: New setting to swap face buttons (A <-> B, X <-> Y) (#153)
- **Cache Purging**: Local cache now purges deleted items using RomM's new identifier endpoints, keeping the cache in sync with the server more efficiently (#83)
- **muOS Jacaranda Icon Support**: Grout icons now display correctly on muOS Jacaranda and prior versions - @pawndev

## Improvements

- **muOS Splash & Preview Options**: Configuration for downloading screenshot, title, and marquee artwork (#86) - @pawndev
- **Gamelist.xml Support**: Added miyoogamelist.xml support for Spruce and Allium (#140) - @pawndev
- **TrimUI Stock Save Sync**: Added save directory mappings for TrimUI stock OS (#128) - @malkavi
- Updated translations across all supported languages (#123)

## Bug Fixes

- Fixed launch scripts and binaries not being executable in zip files (#155)
- Fixed Spruce launch script - @pawndev
- Fixed Trimui packaging - @malkavi

## Documentation

- Added Anbernic RG34XX as tested on ROCKNIX - @SethBarberee
- Added Anbernic RG34XXSP to muOS installation guide - @pawndev
- Added Anbernic RG CUBE to muOS compatibility list - @pawndev
- Added RGB30 to ROCKNIX installation guide - @pawndev
- Updated Knulli tested devices

## Contributors

Thanks to @pawndev, @from-nibly, @malkavi, @SethBarberee, and @ivan for their contributions to this release! :heart:

---

# v4.6.1.0

## New Features

- **Custom Platform Mapping Entry**: Enter custom folder names for platform mappings using the on-screen keyboard when your folder structure doesn't match Grout's suggestions (#103)

## Documentation

- Added per-CFW platform mapping reference tables for Knulli, muOS, NextUI, ROCKNIX, and Spruce
  - Non-technical users can now contribute mapping corrections by editing these simple markdown tables
- Fixed broken documentation links

## Bug Fixes

- Fixed issues with platform mapping validation (#101, #102)

## Internal

- Added pre-commit hook to remove replace directives from go.mod

---

# v4.6.0.0

> [!IMPORTANT]
> **Version Number Change:** Starting with this release, Grout's version number now mirrors the required RomM version.
> The first three components indicate RomM compatibility, and the fourth is for Grout-specific patches.
> This jump from v1.4.2 to v4.6.0.0 reflects alignment with RomM v4.6.0, not a major rewrite.

## New Features

- **ROCKNIX Support**: Preliminary support for ROCKNIX (#96) - thanks to @sucmerep for the `platforms.json`
- **7-Zip Support**: Download and extract 7z compressed ROM files (#77)
- **Version Selection**: Choose which file version to download when multiple are available (#57)
- **Beta Release Channel**: Option to receive beta updates (#62) - @pawndev
- **Match RomM Release Channel**: New release channel option to automatically match your RomM server version (#84)
- **Self-Signed SSL Certificate Support**: Option to skip SSL verification for self-signed certificates (#68)
- **Fuzzy Save Matching**: Third matching option using prefix similarity and Levenshtein distance for save sync
- **Orphan Reattachment**: Automatically reattach orphaned ROMs/saves using CRC32 lookup
- **Background Cache Sync**: Local cache now syncs in the background on startup (#81)
- **Knulli Gamelist Integration**: Grout now adds itself to Knulli's gamelist.xml and reloads on exit (#79) - @pawndev

## Improvements

- muOS app icon now displays correctly (#85) - @pawndev
- Status bar icon legend added to documentation (#82)
- Preloading artwork now considers already cached items (#73)
- Improved version comparison with prerelease support (#63) - @ZacharyKeatings
- More robust caching when options are changed
- Gabagool refactor: FSM replaced with Router for cleaner state management
- In-app updater now enabled for NextUI - @pawndev
- "Redownload" action text for already downloaded games

## Bug Fixes

- Fixed save sync timestamp comparison (#98)
- Fixed muOS Drastic save directory mappings (#89)
- Fixed save sync issues on Knulli with reverse platform mapping (#51)
- Fixed Sega Master System/Mark III custom mapping (#27)

## Documentation

- Added note about deleted items and local cache behavior (#91)
- Added status bar icon legend (#82)
- Updated user guide with file version selection
- Reorganized documentation for easier navigation
- Updated Spruce installation guide - @sargunv

## i18n

- French translation updates - @pawndev

> **Important Note:** Grout `v4.6.0.0` requires RomM `v4.6.0`

---

# v1.4.2

## Bug Fixes

- Fixed incorrect PICO-8 platform mapping (#65)

---

# v1.4.1

## Improvements

- New local SQLite-backed cache for improved performance
- Reworked caching to be more efficient

## Bug Fixes

- Fixed Dreamcast platform mappings for NextUI and Spruce (#59)
- Fixed navigation between platform mapping and main settings screen

---

# v1.4.0

## New Features

- **Spruce v4 Support**: Added support for Spruce CFW (thanks @pawndev!)
- **Kid Mode**: Parental controls with L1+R1+Menu shortcut to disable for current session
- **Select/Deselect All**: Bulk selection option for platform mappings

## Bug Fixes

- Fixed TrimUI input mappings (#44, #46)

---

# v1.3.9

## Bug Fixes

- Fixed Grout not retrieving large ROM collections due to pagination issues

---

# v1.3.8

## Bug Fixes

- Fixed font icon issues on NextUI introduced in v1.3.7

---

# v1.3.7

## Improvements

- Switched to new gabagool default font for expanded icon and glyph support

---

# v1.3.6

## Bug Fixes

- Fixed app not exiting properly after update

---

# v1.3.5

## Bug Fixes

- Fixed auto save sync from hashing files unnecessarily

---

# v1.3.4

## Bug Fixes

- Fixed regression with multi-disk game downloads

---

# v1.3.3

## New Features

- **Auto Updater**: Added in-app update functionality for muOS and Knulli

## Improvements

- Performance and organizational fixes

---

# v1.3.2

## Bug Fixes

- Various bug fixes and performance tweaks

---

# v1.3.1

## Bug Fixes

- Added missing languages to settings screen

---

# v1.3.0

## New Features

- **Knulli Support**: Support for Knulli CFW alongside muOS and NextUI
- **Save Sync**: Automatic save file synchronization with RomM server
  - Basic conflict detection (upload/download/skip logic)
  - Emulator selection for ambiguous save folders
  - Detailed sync reports
  - Local backup creation before downloads
  - Parallelized save scanning for faster sync operations
- **BIOS Downloader**: Download BIOS files directly through Grout
- **Box Art in Games List**: Display box art thumbnails next to game names
- **Download Indicator**: Visual indicator on games list showing which games are already downloaded
- **Platform Reordering**: Ability to reorder platforms on the main menu
- **Smart Collections & Virtual Collections**: Enhanced collection browsing with search
- **Collection View Modes**: View collections unified (all games together) or by platform
- **Collection Search**: Search functionality added to collection selection screen
- **Info Screen**: New info screen for version and build details

## UX Improvements

- **Language Selection on First Boot**: Choose your language during initial setup
- **Enhanced Login Flow**: Better feedback during login process
- **Logout with Confirmation**: Added logout option with confirmation dialog
- **Custom Keyboard Layouts**: URL and numeric keyboard types for easier login input

## i18n

- **Spanish Translation**
- **French Translation** (contributed by @einarliszt )
- **German Translation**
- **Italian Translation**
- **Portuguese Translation**
- **Japanese Translation**
- **Russian Translation**

> **Note: **Claude was used to help localize Grout.
> Any and all help with these translations will be greatly appreciated.

## Bug Fixes

- Fixed incorrect platform slug names in constants
- Added Neo Geo to arcade slug mapping (#28)
- Fixed download indicator display (#26)
- Fixed BIOS download location for muOS

## Internal Improvements

- Resources are now bundled in the binary
- Code cleanup and removal of magic numbers
- muOS input mapping automatically detected

## Compatibility

- Added RG35XX-H to tested devices list

> **Important Note:** Grout `v1.3.0` requires RomM `v4.5.0` as it has API endpoints that facilitate save syncing.