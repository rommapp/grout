# Installation Guide for RetroDECK

This guide will help you install Grout on devices running [RetroDECK][retrodeck].

## Tested Devices

Grout has been tested on the following devices having RetroDECK installed:

| Manufacturer | Device   | OS      |
|--------------|----------|---------|
| Asus         | ROG Ally | Bazzite |

_Please help verify compatibility on other devices by reporting your results!_

## Installation Steps

### Automatic (Recommended)

Here is an all-in-one install script that will install Grout and add it as a non-Steam game.

```bash
curl -o- https://raw.githubusercontent.com/rommapp/grout/refs/heads/main/scripts/RetroDECK/install.sh | bash
```

```bash
wget -qO- https://raw.githubusercontent.com/rommapp/grout/refs/heads/main/scripts/RetroDECK/install.sh | bash
```

### Manual

1. Ensure your device has RetroDECK installed.
2. Download the [latest Grout release](https://github.com/rommapp/grout/releases/latest/download/Grout-RetroDECK.zip) for
   RetroDECK.
3. Unzip the downloaded archive.
4. Copy the `Grout` folder to your home directory (`/home/deck/grout/`)
5. Copy the `Grout.sh` file to the same Ports directory (`/home/deck/grout/Grout.sh`)
6. Open Steam and add Grout as a non-Steam game:
   - Target: `env`
   - Start In: `/home/deck/grout/`
   - Launch options: `/home/deck/grout/Grout.sh`
   - You'll find game media in `/home/deck/grout/Grout/media/`
7. Launch Grout from Steam and enjoy!

## Update

### In-App update (Recommended)

Grout has a built-in update mechanism. To update Grout, launch the application and navigate to the `Settings` menu. From there,
select `Check for Updates`. If a new version is available, follow the on-screen prompts to download and install the update.

### Manual update

To update Grout, simply download the latest release and replace the existing Grout folder in your home directory (`/home/deck/grout/`). If you
have made any custom configurations, ensure to back them up before replacing the folder. Be sure to keep the `config.json`
file if you do not want to authenticate again, and configure platforms folder mappings again.

## Additional Notes

- Grout doesn't currently support custom RetroDECK install location: you must have selected either the internal or the SD card install.
- It seems like Grout doesn't currently play well with Steam Input, ensure it is disabled or you're using an external controller.
- Given the above and if you're running Bazzite, you may need to configure your handheld so it's seen as an Xbox controller.

## Next Steps

After installation is complete, check out the [User Guide](../usage/guide.md) to learn how to use Grout.

--8<-- "docs/_includes/cfw-links.md"
