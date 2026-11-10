# BananaOS

[BananaOS][banana] is a custom firmware for the RG DS Plus. Everything lives on the `/data` partition.

## Quirks

- **Paths**: ROMs are in `/data/roms/<folder>`, saves in `/data/saves/<core>`, BIOS files in `/data/bios`.
  The `BASE_PATH` environment variable overrides the `/data` root.
- **Box art**: art goes to `/data/roms/<folder>/media/box/`, next to the ROMs.
- **Saves**: saves are sorted into one folder per core, so a Game Boy Advance save lives in `gpSP/`. Nintendo DS
  (DraStic) saves are not synced yet.

## Platform Mappings

This table shows the mappings of RomM Fs Slug to BananaOS's platform folders.

| Platform Name                 | RomM Fs Slug | Folder(s) |
|-------------------------------|--------------|-----------|
| Game Boy                      | gb           | gb        |
| Game Boy Advance              | gba          | gba       |
| Game Boy Color                | gbc          | gbc       |
| Nintendo 64                   | n64          | n64       |
| Nintendo DS                   | nds          | nds       |
| Nintendo Entertainment System | nes          | nes       |
| PICO-8                        | pico-8       | pico-8    |
| PlayStation                   | psx          | psx       |
| Sega Genesis                  | genesis      | megadrive |
| Super Nintendo                | snes         | snes      |

--8<-- "docs/_includes/cfw-links.md"
