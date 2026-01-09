# Grout Installation Guide for muOS

This guide will help you install Grout on devices running muOS.

## Tested Devices

Grout has been tested on the following devices running muOS 2508.4 Loose Goose:

| Manufacturer | Device    |
|--------------|-----------|
| Anbernic     | RG34XX    |
| Anbernic     | RG35XX-H  |
| Anbernic     | RG35XXSP  |
| Anbernic     | RG40XXV   |
| TrimUI       | Brick     |
| TrimUI       | Smart Pro |

_Please help verify compatibility on other devices by reporting your results!_

## Installation Steps

1. Ensure your device is running muOS.
2. Download the [latest Grout release](https://github.com/rommapp/grout/releases/latest/download/Grout.muxapp) for muOS.
3. Transfer the `Grout.muxapp` file to an `ARCHIVE` folder on your device:
    - `/mnt/mmc/ARCHIVE` or `/mnt/sdcard/ARCHIVE`
4. Open Applications and launch Archive Manager.
5. Select `[SDX-APP] Grout` from the list and let it extract to your applications directory.
6. Exit Archive Manager.
7. Select `Apps` on the main menu, launch Grout, and enjoy!

## Update

### In-App update (Recommended)
Grout has a built-in update mechanism. To update Grout, launch the application and navigate to the `Settings` menu. From there,
select `Check for Updates`. If a new version is available, follow the on-screen prompts to download and install the update.

### Manual update
To update Grout, simply download the latest release and replace the existing `Grout.muxapp` file in your `ARCHIVE` folder on your device:
- `/mnt/mmc/ARCHIVE` or `/mnt/sdcard/ARCHIVE`. If you have made any custom configurations, ensure to back them up before replacing the file. Be
sure to keep the `config.json` file if you do not want to authenticate again, and configure platforms folder mappings again.


## Next Steps

After installation is complete, check out the [User Guide](USER_GUIDE.md) to learn how to use Grout.
