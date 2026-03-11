# Installation Guide for Batocera

This guide will help you install Grout on devices running [Batocera](https://batocera.org).

## Tested Devices

Grout has been tested on the following devices running Batocera:

| Manufacturer | Device |
|--------------|--------|
| _None yet_   | _Please report your results!_ |

_Please help verify compatibility on other devices by reporting your results!_

## Installation Steps

1. Ensure your device is running Batocera.
2. Download the [latest Grout release](https://github.com/rommapp/grout/releases/latest/download/Grout-Batocera.zip) for
   Batocera.
3. Unzip the downloaded archive.
4. Copy the `Grout` folder to your Ports directory (`/userdata/roms/ports/`)
5. Copy the `Grout.sh` file to the same Ports directory (`/userdata/roms/ports/Grout.sh`)
6. Run `batocera-es-swissknife --updategamelists` or restart EmulationStation.
7. Launch Grout from the `Ports` menu and enjoy!

## Update

### In-App update (Recommended)

Grout has a built-in update mechanism. To update Grout, launch the application and navigate to the `Settings` menu. From there,
select `Check for Updates`. If a new version is available, follow the on-screen prompts to download and install the update.

### Manual update

To update Grout, simply download the latest release and replace the existing Grout folder in your Ports directory (`/userdata/roms/ports`). If you
have made any custom configurations, ensure to back them up before replacing the folder. Be sure to keep the `config.json`
file if you do not want to authenticate again, and configure platforms folder mappings again.

## Next Steps

After installation is complete, check out the [User Guide](../usage/guide.md) to learn how to use Grout.
