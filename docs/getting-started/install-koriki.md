# Installation Guide for Koriki

This guide will help you install Grout on Miyoo Mini devices running [Koriki][koriki].

## Tested Devices

Grout has been tested on the following devices running Koriki:

| Manufacturer | Device          |
|--------------|-----------------|
| Miyoo        | Miyoo Mini Flip |

_Please help verify compatibility on other devices by reporting your results!_

## Prerequisites

- Miyoo Mini device with Koriki installed on an SD card
- Device connected to a Wi-Fi network

## Installation Steps

1. Download the latest Grout release for Koriki from the [releases page](https://github.com/rommapp/grout/releases/latest).
2. Unzip the downloaded archive.
3. Place the `App/Grout` directory into `/mnt/SDCARD/App/` on your SD card.
4. Place the `.simplemenu/apps/Grout.sh` file and the `.simplemenu/apps/Imgs/Grout.png` icon into their respective directories on your SD card.
5. Launch Grout from the SimpleMenu apps list and enjoy!

## Enable RTC for save sync support

1. Open the system settings on your Koriki device
2. Set the correct date and time
3. (recommended) Enable automatic time synchronization if available

## Update

### In-App update (Recommended)

Grout has a built-in update mechanism. To update Grout, launch the application and navigate to the `Settings` menu. From there,
select `Check for Updates`. If a new version is available, follow the on-screen prompts to download and install the update.

### Manual update

To update Grout, simply download the latest release and replace the existing `Grout` folder in `/mnt/SDCARD/App/`. If you
have made any custom configurations, ensure to back them up before replacing the folder. Be sure to keep the `config.json`
file if you do not want to authenticate again, and configure platforms folder mappings again.

## Next Steps

After installation is complete, check out the [User Guide](../usage/guide.md) to learn how to use Grout.

--8<-- "docs/_includes/cfw-links.md"