# Installation Guide for Leaf

This guide will help you install Grout on the Miniloong Pocket 1 running [Leaf][leaf].

## Tested Devices

| Manufacturer | Device             |
|--------------|--------------------|
| Miniloong    | Pocket 1 (MLP1)    |

## Prerequisites

- A Miniloong Pocket 1 running Leaf
- Device connected to a Wi-Fi network (**Settings → Network**)

## Installation Steps

1. Download the latest [Grout release for Leaf](https://github.com/rommapp/grout/releases/latest/download/Grout-Leaf.zip).
2. Unzip the downloaded archive.
3. Copy the `Grout.pak` folder into `Apps/mlp1/` on your SD card, so that Grout ends up in
   `Apps/mlp1/Grout.pak/`.
4. Put the card back in the console, run **Rescan Library** from the launcher's **MENU** (or
   reboot), and launch Grout from the **Apps** tab.

!!! warning "Install location"

    Leaf only lists apps under `Apps/mlp1/`. A `Grout.pak` copied straight into `Apps/` will not
    show up.

!!! note "Second SD card"

    For now Grout only manages the primary card (TF1, the one Leaf boots from). Games and saves on a
    second card in TF2 are not handled yet.

## Enable RTC for save sync support

1. Open **Settings** on your device
2. Set the correct date and time
3. (recommended) Stay connected to Wi-Fi so the clock stays in sync

## Update

### In-App update (Recommended)

Grout has a built-in update mechanism. To update Grout, launch the application and navigate to the `Settings` menu. From there,
select `Check for Updates`. If a new version is available, follow the on-screen prompts to download and install the update.

### Manual update

To update Grout, download the latest release and copy its `Grout.pak` folder into `Apps/mlp1/` again. Be sure to keep
the `config.json` file in `Apps/mlp1/Grout.pak/` if you do not want to authenticate again and configure platform
folder mappings again.

## Next Steps

After installation is complete, check out the [User Guide](../usage/guide.md) to learn how to use Grout.

--8<-- "docs/_includes/cfw-links.md"
