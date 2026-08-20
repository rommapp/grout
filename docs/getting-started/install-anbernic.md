# Installation Guide for the Anbernic Stock OS

This guide will help you install Grout on Anbernic handhelds running the **original Anbernic
firmware** — the "stock OS" that ships preinstalled. No custom firmware is required.

## Tested Devices

Grout has been tested on the following devices running the stock firmware:

| Manufacturer | Device        |
|--------------|---------------|
| Anbernic     | RG35XX Plus   |

_The stock OS is shared across the Allwinner H700 family (RG35XX H/SP/2024, RG34XX, RG40XX,
RG28XX, CubeXX). Please help verify compatibility on other devices by reporting your results!_

## Prerequisites

- An Anbernic H700 handheld running the stock firmware
- Device connected to a Wi-Fi network

## Installation Steps

1. Download the latest [Grout release for the Anbernic stock OS](https://github.com/rommapp/grout/releases/latest/download/Grout-Anbernic.zip).
2. Unzip the downloaded archive.
3. Copy the `Roms` folder to the root of your ROM card, merging it with the existing one. This
   places `Grout.sh` and the `grout` payload folder in `Roms/APPS/`, and the icon in
   `Roms/APPS/Imgs/`.
4. Put the card back in the console and launch Grout from the **Apps** list.

!!! tip "Two card slots"

    Install Grout on the card whose ROMs you want to manage. A copy in TF1's `Roms/APPS/` manages
    `/mnt/mmc`; a copy in TF2's `Roms/APPS/` manages `/mnt/sdcard`. The launch script works this
    out from its own location.

## Enable RTC for save sync support

1. Open the system settings on your device
2. Set the correct date and time
3. (recommended) Connect to Wi-Fi so NTP can keep the clock in sync

## Update

### In-App update (Recommended)

Grout has a built-in update mechanism. To update Grout, launch the application and navigate to the `Settings` menu. From there,
select `Check for Updates`. If a new version is available, follow the on-screen prompts to download and install the update.

### Manual update

To update Grout, download the latest release and copy its `Roms` folder over the card again. If you
have made any custom configurations, ensure to back them up before replacing the folder. Be sure to
keep the `config.json` file in `Roms/APPS/grout/` if you do not want to authenticate again, and
configure platforms folder mappings again.

## Next Steps

After installation is complete, check out the [User Guide](../usage/guide.md) to learn how to use Grout.

--8<-- "docs/_includes/cfw-links.md"
