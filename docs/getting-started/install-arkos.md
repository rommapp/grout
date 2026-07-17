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

1. Download the latest [Grout release for ArkOS](https://github.com/rommapp/grout/releases/latest/download/Grout-ArkOS.zip).
2. Unzip the downloaded archive.
3. Copy the `Grout` folder and `Grout.sh` into `/roms/ports/` on your device.
4. From the EmulationStation main menu, press `Start`, navigate to `Game Settings`, and select `Update Gamelist`.
5. Launch Grout from the `Ports` menu and enjoy!

## Update

> [!NOTE]
> Grout's built-in updater does not support ArkOS yet, so updates must be done manually.

To update Grout, simply download the latest release and replace the existing `Grout` folder and `Grout.sh` in `/roms/ports/`. If you
have made any custom configurations, ensure to back them up before replacing the folder. Be sure to keep the `config.json`
file if you do not want to authenticate again, and configure platforms folder mappings again.

## Next Steps

After installation is complete, check out the [User Guide](../usage/guide.md) to learn how to use Grout.

--8<-- "docs/_includes/cfw-links.md"
