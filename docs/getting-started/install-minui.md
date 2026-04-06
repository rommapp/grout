# Installation Guide for MinUI

This guide will help you install Grout on devices running [MinUI][minui].

## Tested Devices

Grout has been tested on the following devices running MinUI:

| Manufacturer | Device      |
|--------------|-------------|
| Miyoo        | A30         |
| Miyoo        | Mini Plus   |
| Miyoo        | Flip V2     |
| Magicx       | Mini Zero28 |
| Trimui       | Smart Pro   |
| Trimui       | Brick       |

_Please help verify compatibility on other devices by reporting your results!_

## Prerequisites

- Device with MinUI installed on an SD card
- Device connected to a Wi-Fi network

## Installation Steps

1. Download the latest Grout release for MinUI from the [releases page](https://github.com/rommapp/grout/releases/latest).
2. Unzip the downloaded archive.
3. Place the `Grout` directory into `SD_ROOT/Tools/` on your SD card.
4. Launch Grout from the Tools menu and enjoy!

!!! note
    The MinUI distribution includes both ARM32 and ARM64 binaries. The correct one is selected automatically based on your device.

## Update

### In-App update (Recommended)

Grout has a built-in update mechanism. To update Grout, launch the application and navigate to the `Settings` menu. From there,
select `Check for Updates`. If a new version is available, follow the on-screen prompts to download and install the update.

### Manual update

To update Grout, simply download the latest release and replace the existing `Grout` folder in `SD_ROOT/Tools/`. If you
have made any custom configurations, ensure to back them up before replacing the folder. Be sure to keep the `config.json`
file if you do not want to authenticate again, and configure platforms folder mappings again.

## Next Steps

After installation is complete, check out the [User Guide](../usage/guide.md) to learn how to use Grout.

--8<-- "docs/_includes/cfw-links.md"
