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
- **Override Files**: Advanced users can override embedded JSON configuration files for custom platform mappings and BIOS requirements

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