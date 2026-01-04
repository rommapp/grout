# Grout Installation Guide for Knulli

This guide will help you install Grout on devices running Knulli.

## Tested Devices

Grout has been tested on the following devices running Knulli Gladiator II:

| Manufacturer | Device |
|--------------|--------|
| Anbernic     | RG34XX |

_Please help verify compatibility on other devices by reporting your results!_

## Installation Steps

1. Ensure your device is running Knulli.
2. Download the [latest Grout release](https://github.com/rommapp/grout/releases/latest/download/Grout-Knulli.zip) for
   Knulli.
3. Unzip the downloaded archive.
4. Copy the Grout folder to your Tools directory (`/userdata/roms/tools`)
5. On the main Knulli menu, press `start`, navigate to `Game Settings`, and select `Update Gameslist`.
6. Launch Grout from the `Tools` menu and enjoy!

## Important Configuration

> [!IMPORTANT]
> Grout requires a setting to be toggled in Knulli to enable art downloading.
>
> On the main Knulli menu, press `start`, 'System Settings', `Frontend Developer Options` (at the very bottom), and turn
`Search For Local Art` on.

## Update

### In-App update
Grout has a built-in update mechanism. To update Grout, launch the application and navigate to the `Settings` menu. From there,
select `Check for Updates`. If a new version is available, follow the on-screen prompts to download and install the update.

### Manual update
To update Grout, simply download the latest release and replace the existing Grout folder in your Tools directory (`/userdata/roms/tools`) directory. If you
have made any custom configurations, ensure to back them up before replacing the folder. Be sure to keep the `config.json`
file if you do not want to authenticate again, and configure platforms folder mappings again.

## Next Steps

After installation is complete, check out the [User Guide](USER_GUIDE.md) to learn how to use Grout.
