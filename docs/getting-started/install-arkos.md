# Installation Guide for ArkOS / dArkOS

This guide will help you install Grout on devices running [ArkOS][arkos] or [dArkOS][darkos].

## Tested Devices

Grout has been tested on the following devices:

| Manufacturer | Device | CFW |
|--------------|--------|-----|
|              |        |     |

_Please help verify compatibility on other devices by reporting your results!_

## Prerequisites

- Device with ArkOS or dArkOS installed
- Device connected to a Wi-Fi network

## Installation Steps

1. Download the latest Grout release for ArkOS from the [releases page](https://github.com/rommapp/grout/releases/latest).
2. Unzip the downloaded archive.
3. Copy the `Grout` folder and `Grout.sh` into `/storage/roms/ports/` on your device.
4. From the EmulationStation main menu, press `Start`, navigate to `Game Settings`, and select `Update Gamelist`.
5. Launch Grout from the `Ports` menu and enjoy!

## Update

### In-App update (Recommended)

Grout has a built-in update mechanism. To update Grout, launch the application and navigate to the `Settings` menu. From there,
select `Check for Updates`. If a new version is available, follow the on-screen prompts to download and install the update.

### Manual update

To update Grout, simply download the latest release and replace the existing `Grout` folder and `Grout.sh` in `/storage/roms/ports/`. If you
have made any custom configurations, ensure to back them up before replacing the folder. Be sure to keep the `config.json`
file if you do not want to authenticate again, and configure platforms folder mappings again.

## Next Steps

After installation is complete, check out the [User Guide](../usage/guide.md) to learn how to use Grout.

--8<-- "docs/_includes/cfw-links.md"
