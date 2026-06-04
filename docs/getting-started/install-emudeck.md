# Installation Guide for Steam Deck / EmuDeck ES-DE

This guide will help you install Grout as an ES-DE port on a Steam Deck using [EmuDeck][emudeck].

## Tested Devices

Grout has been tested on the following devices running EmuDeck with ES-DE:

| Manufacturer | Device     |
|--------------|------------|
| _None yet_   | _Please report your results!_ |

_Please help verify compatibility by reporting your results!_

## Installation Steps

1. Ensure EmuDeck and ES-DE are installed on your Steam Deck.
2. Download the [latest Grout release](https://github.com/rommapp/grout/releases/latest/download/Grout-ESDE.zip) for
   EmuDeck / ES-DE.
3. Unzip the downloaded archive.
4. Copy the `Grout` folder and `Grout.sh` file to your EmuDeck Ports directory:
    - Internal storage: `~/Emulation/roms/ports/`
    - SD card: `/run/media/<sd-card-name>/Emulation/roms/ports/`
5. Restart ES-DE or refresh your game list.
6. Launch Grout from the `Ports` system and enjoy!

## Important Configuration

!!! important
    Grout expects the standard EmuDeck folder layout where `roms`, `bios`, and `saves` are inside the same
    `Emulation` directory. The launcher derives this path from the `roms/ports` folder that contains `Grout.sh`.

!!! note
    Save sync support on EmuDeck is best-effort because save locations vary by emulator. ROM downloads, BIOS downloads,
    artwork downloads, and ES-DE `gamelist.xml` updates use the standard EmuDeck directories.

## Update

### In-App update (Recommended)

Grout has a built-in update mechanism. To update Grout, launch the application and navigate to the `Settings` menu. From there,
select `Check for Updates`. If a new version is available, follow the on-screen prompts to download and install the update.

### Manual update

To update Grout manually, download the latest release and replace the existing `Grout` folder and `Grout.sh` file in your
EmuDeck Ports directory. If you have made any custom configurations, ensure to back them up before replacing the folder.
Be sure to keep the `config.json` file if you do not want to authenticate again, and configure platform folder mappings again.

## Next Steps

After installation is complete, check out the [User Guide](../usage/guide.md) to learn how to use Grout.

--8<-- "docs/_includes/cfw-links.md"
