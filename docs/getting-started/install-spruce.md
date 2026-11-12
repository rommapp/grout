# Installation Guide for spruce / sprigUI / twigUI

This guide will help you install Grout on devices running [spruce][spruce] v4,
[sprigUI][sprigui], or [twigUI][twigui].

> [!NOTE]
> sprigUI and twigUI are alternative operating systems developed by the spruce team.
> All three are fully supported by the same Grout package for spruce.

## Tested Devices

Grout has been tested on the following devices running spruce:

| Manufacturer | Device    |
|--------------|-----------|
| Miyoo        | A30       |
| Miyoo        | Flip      |
| Miyoo        | Mini Flip |
| TrimUI       | Brick     |
| TrimUI       | Smart Pro |
| GKD          | Pixel 2   |
| Powkiddy     | RGB30     |

## Prerequisites

- Device with spruce (v4/nightlies) installed on an SD card
- Device connected to a Wi-Fi network

## Installation Steps

### Manual Installation

1. Download the [latest Grout release](https://github.com/rommapp/grout/releases/latest/download/Grout.spruce.zip) for spruce.
2. Unzip the downloaded archive.
3. Place the `Grout` directory into `SD_ROOT/App/`.
4. Launch Grout from the `App` menu and enjoy!

## Update

### In-App update (Recommended)

Grout has a built-in update mechanism. To update Grout, launch the application and navigate to the `Settings` menu. From there,
select `Check for Updates`. If a new version is available, follow the on-screen prompts to download and install the update.

### Manual update

To update Grout, simply download the latest release and replace the existing Grout folder in your `SD_ROOT/App/` directory. If you
have made any custom configurations, ensure to back them up before replacing the folder. Be sure to keep the `grout/config.json`
file if you do not want to authenticate again, and configure platforms folder mappings again.

## Next Steps

After installation is complete, check out the [User Guide](../usage/guide.md) to learn how to use Grout.

--8<-- "docs/_includes/cfw-links.md"
