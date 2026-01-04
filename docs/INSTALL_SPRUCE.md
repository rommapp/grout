# Grout Installation Guide for Spruce

This guide will help you install Grout on TrimUI devices running Spruce v4.

## Tested Devices

Grout has been tested on the following devices running Spruce:

| Manufacturer | Device  | Compatibility |
|--------------|---------|---------------|
| Miyoo        | Flip V2 | Yes           |
| Miyoo        | A30     | No            |

## Prerequisites

- Miyoo device with Spruce (v4/nightlies) installed on an SD card
- Device connected to a Wi-Fi network

## Installation Steps

You can install Grout using one of these two methods:

### Method 1: Manual Installation

1. Download the [latest Grout release](https://github.com/rommapp/grout/releases/latest/download/Grout.spruce.zip) for Spruce.
2. Unzip the downloaded archive.
3. Place the `Grout` directory into `SD_ROOT/App/`.
4. Launch Grout from the `App` menu and enjoy!

## Update

### In-App update
Grout has a built-in update mechanism. To update Grout, launch the application and navigate to the `Settings` menu. From there,
select `Check for Updates`. If a new version is available, follow the on-screen prompts to download and install the update.

### Manual update
To update Grout, simply download the latest release and replace the existing Grout folder in your `SD_ROOT/App/` directory. If you
have made any custom configurations, ensure to back them up before replacing the folder. Be sure to keep the `grout/config.json`
file if you do not want to authenticate again, and configure platforms folder mappings again.

## Next Steps

After installation is complete, check out the [User Guide](USER_GUIDE.md) to learn how to use Grout.
