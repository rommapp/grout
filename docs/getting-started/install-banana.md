# Installation Guide for BananaOS

This guide will help you install Grout on devices running [BananaOS][banana].

## Tested Devices

Grout has been tested on the RG DS Plus running BananaOS 0.5.0.1.

_Please help verify compatibility on other devices by reporting your results!_

## Installation Steps

1. Ensure your device is running BananaOS and connected to a Wi-Fi network.
2. Download the [latest Grout release](https://github.com/rommapp/grout/releases/latest/download/Grout-Banana.zip) for
   BananaOS.
3. Unzip the downloaded archive.
4. Copy the `Grout` folder and `Grout.sh` to `/data/roms/ports`.
5. Launch Grout from the Ports menu and enjoy!

## Update

### In-App update (Recommended)

Grout has a built-in update mechanism. To update Grout, launch the application and navigate to the `Settings` menu. From there,
select `Check for Updates`. If a new version is available, follow the on-screen prompts to download and install the update.

### Manual update

To update Grout, download the latest release and replace the existing `Grout` folder in `/data/roms/ports`. Be sure to keep
the `config.json` file if you do not want to authenticate again and configure platform folder mappings again.

## Next Steps

After installation is complete, check out the [User Guide](../usage/guide.md) to learn how to use Grout.

--8<-- "docs/_includes/cfw-links.md"
