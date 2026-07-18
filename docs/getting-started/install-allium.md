# Installation Guide for Allium

This guide will help you install Grout on Miyoo Mini devices running [Allium][allium].

## Tested Devices

Grout has been tested on the following devices running Allium:

| Manufacturer | Device          |
|--------------|-----------------|
| Miyoo        | Miyoo Mini Flip |

_Please help verify compatibility on other devices by reporting your results!_

## Prerequisites

- Miyoo Mini device with Allium installed on an SD card
- Device connected to a Wi-Fi network

## Installation Steps

1. Download the latest [Grout release for Allium](https://github.com/rommapp/grout/releases/latest/download/Grout-Allium.zip).
2. Unzip the downloaded archive.
3. Place the `Grout.pak` directory into `/mnt/SDCARD/Apps/` on your SD card.
4. Launch Grout from the Apps menu and enjoy!

## Update

### In-App update (Recommended)

Grout has a built-in update mechanism. To update Grout, launch the application and navigate to the `Settings` menu. From there,
select `Check for Updates`. If a new version is available, follow the on-screen prompts to download and install the update.

### Manual update

To update Grout, simply download the latest release and replace the existing `Grout.pak` folder in `/mnt/SDCARD/Apps/`. If you
have made any custom configurations, ensure to back them up before replacing the folder. Be sure to keep the `config.json`
file if you do not want to authenticate again, and configure platforms folder mappings again.

## Next Steps

After installation is complete, check out the [User Guide](../usage/guide.md) to learn how to use Grout.

--8<-- "docs/_includes/cfw-links.md"
