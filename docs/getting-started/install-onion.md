# Installation Guide for Onion

This guide will help you install Grout on Miyoo Mini devices running [Onion][onion].

## Tested Devices

Grout has been tested on the following devices running Onion:

| Manufacturer | Device         |
|--------------|----------------|
| Miyoo        | Miyoo Mini Plus |

_Please help verify compatibility on other devices by reporting your results!_

## Prerequisites

- Miyoo Mini device with Onion installed on an SD card
- Device connected to a Wi-Fi network

## Installation Steps

1. Download the latest Grout release for Onion from the [releases page](https://github.com/rommapp/grout/releases/latest).
2. Unzip the downloaded archive.
3. Place the `Grout` directory into `/mnt/SDCARD/App/` on your SD card.
4. Launch Grout from the Apps menu and enjoy!

## Enable RTC for save sync support

1. Install the Clock app from OnionOS Package Manager
2. Open Clock app and set the time
3. (recommended) Open the Tweaks app
4. (recommended) Go to System, then Date and Time section and enable "set time from internet"
5. (optional) Enable "Wait for sync on startup"

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
