# Frequently Asked Questions

Can't find what you're looking for? [Open an issue](https://github.com/rommapp/grout/issues/new/choose){:target="_blank"} on GitHub.

---

## General

???+ question "What RomM version do I need?"
    Grout aggressively adopts new RomM features. The required RomM version matches the **first three components** of Grout's
    version number. 

    For example, Grout `v4.6.1.0` requires RomM `4.6.1` or newer. The fourth component is for Grout-specific patches.

    Grout may still function on older RomM versions, but support will not be provided.

??? question "Which devices and firmware are supported?"
    See the [Quick Start Guide](../getting-started/index.md) for the full list of supported platforms and installation instructions.

---

## Connection & Login

???+ question "I can't connect to my RomM server. What should I check?"
    1. **Wi-Fi** - Confirm your device is connected to Wi-Fi and can reach the server.
    2. **Hostname** - Make sure the hostname or IP address is correct and reachable from your device's network.
    3. **Protocol** - Try switching between HTTP and HTTPS. If you get a "protocol mismatch" error, you're using the wrong
       one.
    4. **Port** - If your RomM instance runs on a non-standard port, include it in the port field.
    5. **Firewall** - Ensure your server's firewall allows connections from your device's network.

??? question "I'm getting SSL / certificate errors"
    If you're using a self-signed certificate, set **SSL Certificates** to **Skip Verification** on the login screen.

??? question "My connection keeps timing out"
    Increase the **API Timeout** and **Download Timeout** in Settings > Advanced.

    The default may be too short for slow networks, remote servers, large downloads, or RomM instances with a large games
    collection.

---

## Downloading Games

???+ question "Can I filter the games list?"
    Yes. Press `Y` from any game list to filter by genre, franchise, company, game mode, region, language, age rating, or tag. Only categories with available values for the current platform are shown. Press `B` to clear all filters.

??? question "Can I download multiple games at once?"
    Yes. Press `Select` to enter multi-select mode, then use `A` to toggle individual games, `R1` to select all, or `L1` to
    deselect all. Press `Start` to confirm and begin downloading.

??? question "How do multi-disc games work?"
    When you download a multi-disc game, Grout automatically extracts and creates an `.m3u` playlist file.

??? question "What does the "Archived Downloads" setting do?"
    When set to **Uncompress**, Grout will automatically extract downloaded `.zip` and `.7z` archives after downloading.
    When set to **Do Nothing**, the archive is saved as-is.

??? question "What's the difference between the downloaded game indicators?"
    In Settings, the **Downloaded Games** option controls how already-downloaded games appear in the games list:

    - **Do Nothing** - No visual difference
    - **Mark** - Downloaded games are shown with a download icon
    - **Filter** - Downloaded games are hidden from the list entirely

---

## Box Art & Artwork

???+ question "What are the different art types?"
    The **Download Art Kind** setting controls which artwork Grout downloads from RomM:

    - **Default** - Uses whatever artwork RomM provides as the default cover
    - **Box2D** - Flat front box art
    - **Box3D** - 3D rendered box art with perspective
    - **MixImage** - Composite image combining box art, screenshots, and system logos

??? question "Can I download artwork for games I already have?"
    Yes! Use [Tools > Download Missing Art](settings.md#download-missing-art) to scan all mapped platforms and download cover art for any games missing cached artwork.

---

## Save Sync

???+ question "How does Grout match my local games to RomM?"
    Grout matches by **platform and filename** - if the local save filename (without extension) exactly matches a ROM's
    filename in RomM for the same platform, it's considered a match. For the best experience, keep your local ROM
    filenames consistent with the names in your RomM library.

    For more details, see the [Save Sync Guide](save-sync.md).

??? question "What's the difference between save files and save states?"
    **Save files** (`.srm`, `.sav`, etc.) are created by the emulated game itself - like saving at a save point. These are
    what Grout syncs.

    **Save states** are snapshots of the entire emulator state at a moment in time. These are emulator-specific and are
    **not currently synced** by Grout.

??? question "Will my saves be overwritten?"
    Grout always creates a backup of the existing local save before downloading a newer one from RomM. The newer save (by
    timestamp) always wins.

---

## Platform Mappings

???+ question "Can I change my platform mappings later?"
    Yes. Go to Settings > **Directory Mappings** to reconfigure which local folder maps to each RomM platform.

??? question "What happens if I skip a platform during mapping?"
    Games for that platform won't be visible in Grout and can't be downloaded until a mapping is configured.

---

## BIOS Files

???+ question "How do I download BIOS files?"
    Navigate to a platform's game list. If the platform in RomM has BIOS files, Grout will show a prompt in the footer to press `Menu`.
    From there Grout will list the BIOS files along with their status (Ready or Missing).

??? question "Not all platforms show a BIOS option. Why?"
    The option will only appear when the platform in RomM has BIOS files associated with it.

---

## Settings & Configuration

???+ question "What is Kids Mode?"
    Kids Mode hides Settings, Save Sync, Game Options, and BIOS downloads, leaving only game browsing and downloading.
    To temporarily access these while Kids Mode is enabled, press `L1 + R1 + Menu` during the Grout splash screen.

??? question "Will updating Grout erase my settings?"
    No. The in-app updater and manual updates both preserve your `config.json` file, which contains your credentials
    and platform mappings.

??? question "What log level should I use?"
    - **Error** - Only shows errors. Use this for normal operation.
    - **Info** - Shows general activity. Useful for understanding what Grout is doing.
    - **Debug** - Verbose logging. Use this when troubleshooting issues or filing bug reports.

---

## Troubleshooting

???+ question "Where are the log files?"
    Log files are stored alongside the Grout binary in a `logs` directory. The exact path depends on your firmware and
    installation location.

??? question "My cache seems wrong or outdated"
    Go to Settings > Advanced > **Rebuild Cache**. This clears and rebuilds the local database from your RomM server.

??? question "Downloads are slow or failing"
    1. Increase the **Download Timeout** in Settings > Advanced.
    2. Check your Wi-Fi signal strength - handheld devices often have limited range.
    3. Verify your RomM server isn't under heavy load.

??? question "I found a bug or have a feature request"
    Please [create an issue](https://github.com/rommapp/grout/issues/new/choose) on GitHub and fill out the template
    completely. Include your Grout version, RomM version, device, firmware, and relevant log output.
