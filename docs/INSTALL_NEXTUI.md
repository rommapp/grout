# Grout Installation Guide for NextUI

This guide will help you install Grout on TrimUI devices running NextUI.

## Tested Devices

Grout has been tested on the following devices running NextUI:

| Manufacturer | Device    |
|--------------|-----------|
| TrimUI       | Brick     |
| TrimUI       | Smart Pro |

## Prerequisites

- TrimUI device with NextUI installed on an SD card
- Device connected to a Wi-Fi network

## Installation Steps

You can install Grout using one of these two methods:

### Method 1: Pak Store (Recommended)

1. Launch the NextUI Pak Store from the `Tools` menu.
2. Navigate to the `ROM Management` category.
3. Select Grout and install.
4. Launch Grout from the `Tools` menu and enjoy!

### Method 2: Manual Installation

1. Download the [latest Grout release](https://github.com/rommapp/grout/releases/latest/download/Grout.pak.zip) for NextUI.
2. Unzip the downloaded archive.
3. Place the `Grout.pak` directory into `SD_ROOT/Tools/tg5040`.
4. Launch Grout from the `Tools` menu and enjoy!

## Update

### Pak Store update (Recommended)
1. Launch the NextUI Pak Store from the `Tools` menu.
2. If there are updates available, you will see an entry in the menu name `Available Pak Updates`. Navigate to it
3. Select Grout and update.
4. Launch Grout from the `Tools` menu and enjoy the latest version!

### Manual update
To update Grout, simply download the latest release and replace the existing Grout folder in your `SD_ROOT/Tools/tg5040` directory. If you
have made any custom configurations, ensure to back them up before replacing the folder. Be sure to keep the `config.json`
file if you do not want to authenticate again, and configure platforms folder mappings again.

## Next Steps

After installation is complete, check out the [User Guide](USER_GUIDE.md) to learn how to use Grout.
