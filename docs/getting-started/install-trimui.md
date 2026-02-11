# Installation Guide for TrimUI Stock OS

This guide will help you install Grout on devices running the TrimUI stock operating system.

## Tested Devices

Grout has been tested on the following devices running TrimUI Stock OS:

| Manufacturer | Device    |
|--------------|-----------|
| TrimUI       | Brick     |
| TrimUI       | Smart Pro |

_Please help verify compatibility on other devices by reporting your results!_

## Prerequisites

- TrimUI device running stock OS with an SD card
- Device connected to a Wi-Fi network

## Installation Steps

### Manual Installation

1. Download the [latest Grout release](https://github.com/rommapp/grout/releases/latest/download/Grout-Trimui.zip) for TrimUI.
2. Unzip the downloaded archive.
3. Place the `Grout` directory into `SD_ROOT/Apps/`.
4. Launch Grout from the `Apps` menu and enjoy!

## Update

### In-App update (Recommended)

Grout has a built-in update mechanism. To update Grout, launch the application and navigate to the `Settings` menu. From there,
select `Check for Updates`. If a new version is available, follow the on-screen prompts to download and install the update.

### Manual update

To update Grout, simply download the latest release and replace the existing Grout folder in your `SD_ROOT/Apps/` directory. If you
have made any custom configurations, ensure to back them up before replacing the folder. Be sure to keep the `grout/config.json`
file if you do not want to authenticate again, and configure platforms folder mappings again.

## Next Steps

After installation is complete, check out the [User Guide](../usage/guide.md) to learn how to use Grout.
