# Unverified Platform Mappings

This page lists platform data in the Grout source that is known to be
inconsistent, but where resolving it correctly needs someone with the device in
hand. Each entry says what disagrees, what to check, and what to change.

These are **not** open questions about how Grout should work. They are questions
about what a given firmware actually names a folder, which cannot be answered by
reading the code.

If you own one of these devices and can check, please open a PR or drop the
answer in an issue.

!!! note
    Platform tables under `cfw/*/data/` are the source of truth and are embedded
    in the binary. The pages under **CFW Specific Info** are generated from them
    by `task gen-platform-docs`. Fix the JSON, not the markdown.

---

## `pico`: PICO-8 or Sega Pico?

**Firmware:** muOS

Grout's own tables disagree about what the RomM slug `pico` means.

| Where | Value |
|---|---|
| `cfw/muos/data/platforms.json` | `pico` → `["pico8", "PICO-8"]` |
| `cfw/muos/data/art_directories.json:58` | `pico` → `"Sega Pico"` |

So a `pico` game's rom is written to the **PICO-8** folder while its box art is
written to the **Sega Pico** catalogue. One of them is wrong.

All twelve platform docs map `pico` to PICO-8, which suggests the art table is
the outlier — but if RomM's `pico` slug really does mean Sega Pico, then all
twelve are wrong and the fix is much larger.

**To verify:** check what RomM reports as the `fs_slug` for a Sega Pico library
and for a PICO-8 library. If they are `pico` and `pico-8` respectively, then
`pico` is Sega Pico and every firmware's mapping needs correcting. If RomM has
no separate Sega Pico slug, then `art_directories.json` should be `"PICO-8"`.

**Related:** `pico-8` is mapped to PICO-8 by every firmware and is not in doubt.

---

## Koriki: `snes` and `sfam` point at different emulators

**Firmware:** Koriki

`cfw/koriki/data/save_directories.json` is internally inconsistent for what is
physically the same console:

| Slug | Line | Emulator folder |
|---|---|---|
| `sfam` | 128 | `["Supafaust", "Snes9x"]` |
| `snes` | 141 | `["Beetle Supafaust", "Snes9x"]` |

The neighbouring Miyoo firmwares are each self-consistent, and disagree with
each other:

| Firmware | `snes` | `sfam` |
|---|---|---|
| Onion | `Supafaust` | `Supafaust` |
| Allium | `Beetle Supafaust` | `Beetle Supafaust` |
| Koriki | `Beetle Supafaust` | `Supafaust` |

`cfw.GetSaveDirectory` uses `emulatorDirs[0]` (`cfw/saves.go:104`, and
`GetSaveDirectoryForRomPath` at `:134`), so only the first entry is ever used.
Whichever of the two is wrong silently never syncs saves.

**To verify:** on a Koriki device, look under the saves root for the SNES
emulator's directory and note its exact name.

**To fix:** make both slugs list the same folder first. If Koriki genuinely
ships both directories, list them in preference order rather than picking one —
the list is already ordered.

---

## PCSX ReARMed: hyphen or space?

**Firmware:** Allium, Koriki, Onion

The same emulator is spelled two ways, all at line 116 of each firmware's
`save_directories.json`:

| Firmware | `psx` |
|---|---|
| Allium | `["PCSX ReARMed"]` (space) |
| Koriki | `["PCSX-ReARMed"]` (hyphen) |
| Onion | `["PCSX-ReARMed"]` (hyphen) |

These are three separate firmwares, so this may not be a bug at all — each is
free to name the folder differently. It is listed here because the difference
looks like a typo and should be confirmed rather than assumed either way.

**To verify:** on each device, check the exact directory name under the saves
root after running a PlayStation game.

---

## Adding to this page

Keep entries in this shape: what disagrees (with file and line), what the
neighbouring firmwares do, what to physically check, and what to change once
the answer is known. Remove an entry when it is resolved — the fix itself is
the record.
