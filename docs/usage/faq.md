# Frequently Asked Questions

---

## General

### What RomM version do I need?

Grout aggressively adopts new RomM features. The required RomM version matches the **first three components** of Grout's
version number. For example, Grout `v4.6.1.0` requires RomM `4.6.1` or newer. The fourth component is for Grout-specific
patches.

Grout may still function on older RomM versions, but support will not be provided.

### Which devices and firmware are supported?

Grout
supports [muOS](../getting-started/install-muos.md), [Knulli](../getting-started/install-knulli.md), [ROCKNIX](../getting-started/install-rocknix.md), [Spruce](../getting-started/install-spruce.md), [NextUI](../getting-started/install-nextui.md),
and [TrimUI Stock OS](../getting-started/install-trimui.md). See each installation guide for tested devices.

### Can I use Grout with OIDC / Single Sign-On?

Grout requires a username and password to authenticate. If your RomM instance uses OIDC, you can still use Grout by
setting a password for your user account. Grout will support API Keys once they are available in RomM.

For more details, see [this GitHub issue](https://github.com/rommapp/romm/issues/1767#issuecomment-2744215262){:
target="_blank"}.

---

## Connection & Login

### I can't connect to my RomM server. What should I check?

1. **Wi-Fi** -Confirm your device is connected to Wi-Fi and can reach the server.
2. **Hostname** -Make sure the hostname or IP address is correct and reachable from your device's network.
3. **Protocol** -Try switching between HTTP and HTTPS. If you get a "protocol mismatch" error, you're using the wrong
   one.
4. **Port** -If your RomM instance runs on a non-standard port, include it in the port field.
5. **Firewall** -Ensure your server's firewall allows connections from your device's network.

### I'm getting SSL / certificate errors

If you're using a self-signed certificate, set **SSL Certificates** to **Skip Verification** on the login screen.

### My connection keeps timing out

Increase the **API Timeout** and **Download Timeout** in Settings > Advanced.

The default may be too short for slow networks, remote servers, large downloads, or RomM instances with a large games
collection.

---

## Downloading Games

### Can I filter the games list?

Yes. Press `Y` from any game list to filter by genre, franchise, company, game mode, region, language, age rating, or tag. Only categories with available values for the current platform are shown. Press `B` to clear all filters.

### Can I download multiple games at once?

Yes. Press `Select` to enter multi-select mode, then use `A` to toggle individual games, `R1` to select all, or `L1` to
deselect all. Press `Start` to confirm and begin downloading.

### How do multi-disc games work?

When you download a multi-disc game, Grout automatically extracts the disc images and creates an `.m3u` playlist file.

### What does the "Archived Downloads" setting do?

When set to **Uncompress**, Grout will automatically extract downloaded `.zip` and `.7z` archives after downloading.
When set to **Do Nothing**, the archive is saved as-is.

### What's the difference between the downloaded game indicators?

In Settings, the **Downloaded Games** option controls how already-downloaded games appear in the games list:

- **Do Nothing** -No visual difference
- **Mark** -Downloaded games are shown with a download icon
- **Filter** -Downloaded games are hidden from the list entirely

---

## Box Art & Artwork

### What are the different art types?

The **Download Art Kind** setting controls which artwork Grout downloads from RomM:

- **Default** -Uses whatever artwork RomM provides as the default cover
- **Box2D** -Flat front box art
- **Box3D** -3D rendered box art with perspective
- **MixImage** -Composite image combining box art, screenshots, and system logos

### Can I download artwork for games I already have?

This is currently not supported.

The **Preload Artwork** setting only downloads artwork to be displayed within Grout's interface.

See [this GitHub issue](https://github.com/rommapp/grout/issues/130){:target="_blank"} to track this feature.

---

## Save Sync

### How does Grout match my local games to RomM?

Grout uses three methods in order:

1. **Filename match** -If the local filename (without extension) matches a ROM in RomM, it's an instant match.
2. **Hash match** -Grout computes CRC32/SHA1 hashes of the local ROM and queries RomM. Successful matches are remembered
   for future instant lookups.
3. **Fuzzy title match** -Grout normalizes both names and compares them. If similarity is 80% or higher, you'll be asked
   to confirm the match.

For more details, see the [Save Sync Guide](save-sync.md).

### What's the difference between save files and save states?

**Save files** (`.srm`, `.sav`, etc.) are created by the emulated game itself -like saving at a save point. These are
what Grout syncs.

**Save states** are snapshots of the entire emulator state at a moment in time. These are emulator-specific and are *
*not synced** by Grout.

### Will my saves be overwritten?

Grout always creates a backup of the existing local save before downloading a newer one from RomM. The newer save (by
timestamp) always wins.

### Why are some saves "unmatched"?

A save is unmatched when Grout can't find a corresponding ROM in your RomM library. This can happen if:

- The ROM was renamed locally and doesn't match any name or hash in RomM
- The ROM isn't in your RomM library at all
- A previous fuzzy match was declined (24-hour cooldown before re-prompting)

The sync summary shows unmatched saves with diagnostic info to help you resolve them.

### What's the difference between Manual and Automatic sync?

- **Manual** -Press `Y` from the main menu to trigger a sync. A summary is shown when complete.
- **Automatic** -Grout syncs in the background every time you launch the app. Progress is shown via status bar icons.

---

## Platform Mappings

### Can I change my platform mappings later?

Yes. Go to Settings > **Directory Mappings** to reconfigure which local folder maps to each RomM platform.

### What happens if I skip a platform during mapping?

Games for that platform won't be visible in Grout and can't be downloaded until a mapping is configured.

---

## BIOS Files

### How do I download BIOS files?

Navigate to a platform's game list. If the platform in RomM has BIOS files, Grout will show a prompt in the footer to press `Menu`.
From there Grout will list the BIOS files along with their status (Ready or Missing).

### Not all platforms show a BIOS option. Why?

Many platforms don't require BIOS files, and some may not have BIOS files available in your RomM library. The option
only appears when applicable.

---

## Settings & Configuration

### What is Kids Mode?

Kids Mode hides the Settings and other advanced screens, leaving only game browsing and downloading. To temporarily
access settings while Kids Mode is enabled, press `L1 + R1 + Menu`.

### Will updating Grout erase my settings?

No. The in-app updater and manual updates both preserve your `config.json` file, which contains your login credentials
and platform mappings.

### What log level should I use?

- **Error** -Only shows errors. Use this for normal operation.
- **Info** -Shows general activity. Useful for understanding what Grout is doing.
- **Debug** -Verbose logging. Use this when troubleshooting issues or filing bug reports.

---

## Troubleshooting

### Where are the log files?

Log files are stored alongside the Grout binary in a `logs` directory. The exact path depends on your firmware and
installation location.

### My cache seems wrong or outdated

Go to Settings > Advanced > **Rebuild Cache**. This clears and rebuilds the local database from your RomM server.

### Downloads are slow or failing

1. Increase the **Download Timeout** in Settings > Advanced.
2. Check your Wi-Fi signal strength -handheld devices often have limited range.
3. Verify your RomM server isn't under heavy load.

### I found a bug or have a feature request

Please [create an issue](https://github.com/rommapp/grout/issues/new/choose) on GitHub and fill out the template
completely. Include your Grout version, RomM version, device, firmware, and relevant log output.
