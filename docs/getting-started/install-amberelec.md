# Installation Guide for AmberELEC

This guide will help you install Grout on devices running [AmberELEC][amberelec].

## Tested Devices

AmberELEC support is still being verified across devices.

_Please help verify compatibility on your device by reporting your results!_

## Installation Steps

1. Ensure your device is running AmberELEC.
2. Download the [latest Grout release](https://github.com/rommapp/grout/releases/latest/download/Grout-AmberELEC.zip) for AmberELEC.
3. Unzip the downloaded archive.
4. Create the directory `/storage/roms/ports/Grout.sh/`.
5. Copy the extracted data, that includes `Grout.sh` into `/storage/roms/ports/Grout.sh/`, so you end up with `/storage/roms/ports/Grout.sh/Grout.sh` as AmberELEC's EmulationStation will launch the port directly when the folder name matches the executable inside it.
6. Refresh your ports list from the frontend if Grout does not appear immediately.
7. Launch Grout from the `Ports` menu and enjoy.

## Important Configuration

!!! important
    If artwork does not appear in the frontend, enable `Search For Local Art` in the frontend's developer options.

## Update

### In-App update (Recommended)

Grout has a built-in update mechanism. To update Grout, launch the application and navigate to the `Settings` menu. From there,
select `Check for Updates`. If a new version is available, follow the on-screen prompts to download and install the update.

### Manual update

To update Grout, download the latest release and replace the existing `Grout.sh` launcher and `Grout` folder inside
`/storage/roms/ports/Grout.sh/`.
If you have made any custom configurations, ensure to back them up before replacing the folder. Be sure to keep the
`config.json` file if you do not want to authenticate again, and configure platform folder mappings again.

## Next Steps

After installation is complete, check out the [User Guide](../usage/guide.md) to learn how to use Grout.

--8<-- "docs/_includes/cfw-links.md"