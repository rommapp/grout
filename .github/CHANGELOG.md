# v4.6.0.0

> [!IMPORTANT]
> **Version Number Change:** Starting with this release, Grout's version number now mirrors the required RomM version.
> The first three components indicate RomM compatibility, and the fourth is for Grout-specific patches.
> This jump from v1.4.2 to v4.6.0.0 reflects alignment with RomM v4.6.0, not a major rewrite.

## New Features

- **ROCKNIX Support**: Preliminary support for ROCKNIX (#96)
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