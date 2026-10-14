# Leaf

[Leaf](https://leaf.game) is a custom firmware for the Miniloong Pocket 1 (MLP1). It runs on top of the stock
LoongOS rather than replacing it, and keeps one ROM folder per system.

## Quirks

- **Install location**: Leaf only lists paks under `Apps/mlp1/`. A `Grout.pak` placed directly in
  `Apps/` is not discovered.
- **Downloaded games**: Leaf lists games from its own library index. Grout asks the launcher to
  rescan after every download; if a game still does not show up, run **Rescan Library** from the
  launcher's **MENU**.
- **Folder names**: Leaf accepts several aliases for a system folder (`NES`, `FC` and `FAMICOM`
  all work) and ignores case. Grout downloads to the recommended folder, listed first below.
- **Box art**: art goes to `Images/<FOLDER>/`, named like the ROM folder and after the game file
  (`Images/SNES/Chrono Trigger.png`). Leaf also looks there for a ROM folder that uses an alias, so
  games in `Roms/FC` get their art from `Images/FC`.
- **Saves**: Leaf's RetroArch sorts saves into one folder per core, so a mGBA save for
  `Roms/GBA/Game.gba` lives at `Saves/mGBA/Game.srm`. Saves written by the standalone emulators
  (DraStic, PPSSPP, standalone Flycast and Mupen64Plus) live elsewhere and are not synced.
- **BIOS**: BIOS files go to `BIOS/`, RetroArch's system directory, keeping the sub-folders the
  cores expect (`BIOS/dc/`, `BIOS/np2kai/`). Amiga kickstarts go to `BIOS/puae/` and Saturn BIOS
  files to `BIOS/SATURN/`, as Leaf's packaged emulators expect.
- **Two card slots**: for now Grout only manages the primary card (TF1, the one Leaf boots from).
  ROMs, saves, BIOS and art on a second card in TF2 are not seen or synced, even if Grout is
  installed there.
- **HTTPS**: the launch script points Grout at Leaf's certificate bundle, since stock LoongOS has
  no default one.

## Platform Mappings

This table shows the mappings of RomM Fs Slug to Leaf platform folders.

| Platform Name                 | RomM Fs Slug               | Folder(s)                                       |
|-------------------------------|----------------------------|-------------------------------------------------|
| 3DO Interactive Multiplayer   | 3do                        | *(none)*                                        |
| Amiga                         | amiga                      | AMIGA                                           |
| Amstrad CPC                   | acpc                       | *(none)*                                        |
| Arcade                        | arcade                     | ARCADE, MAME, MAME2003, MAME2010                |
| Arduboy                       | arduboy                    | *(none)*                                        |
| Atari 800                     | atari800                   | *(none)*                                        |
| Atari 2600                    | atari2600                  | ATARI2600, A2600                                |
| Atari 5200                    | atari5200                  | *(none)*                                        |
| Atari 7800                    | atari7800                  | A7800, ATARI7800                                |
| Atari Jaguar                  | jaguar                     | *(none)*                                        |
| Atari Lynx                    | lynx                       | LYNX                                            |
| Atari ST/STE                  | atari-st                   | *(none)*                                        |
| Capcom Play System 1          | cps1                       | ARCADE                                          |
| Capcom Play System 2          | cps2                       | ARCADE                                          |
| Capcom Play System 3          | cps3                       | ARCADE                                          |
| Cave Story                    | cave-story                 | *(none)*                                        |
| ColecoVision                  | colecovision               | COLECO, COLECOVISION                            |
| Commodore 128                 | c128                       | *(none)*                                        |
| Commodore C64/128/MAX         | c64                        | *(none)*                                        |
| Commodore PET                 | cpet                       | *(none)*                                        |
| Commodore VIC-20              | vic-20                     | *(none)*                                        |
| DOS                           | dos                        | DOS                                             |
| Dreamcast                     | dc                         | DC, DREAMCAST                                   |
| Fairchild Channel F           | fairchild-channel-f        | *(none)*                                        |
| Family Computer               | famicom                    | NES, FC, FAMICOM                                |
| Family Computer Disk System   | fds                        | FDS                                             |
| Galaksija                     | galaksija                  | *(none)*                                        |
| Game & Watch                  | g-and-w                    | GW                                              |
| Game Boy                      | gb                         | GB                                              |
| Game Boy Advance              | gba                        | GBA                                             |
| Game Boy Color                | gbc                        | GBC                                             |
| Intellivision                 | intellivision              | *(none)*                                        |
| J2ME                          | j2me                       | *(none)*                                        |
| Mega Duck/Cougar Boy          | mega-duck-slash-cougar-boy | *(none)*                                        |
| MSX                           | msx                        | *(none)*                                        |
| Neo Geo AES                   | neogeoaes                  | NEOGEO                                          |
| Neo Geo CD                    | neo-geo-cd                 | *(none)*                                        |
| Neo Geo MVS                   | neogeomvs                  | NEOGEO                                          |
| Neo Geo Pocket                | neo-geo-pocket             | NGP                                             |
| Neo Geo Pocket Color          | neo-geo-pocket-color       | NGPC                                            |
| Nintendo 3DS                  | 3ds                        | *(none)*                                        |
| Nintendo 64                   | n64                        | N64                                             |
| Nintendo DS                   | nds                        | NDS                                             |
| Nintendo Entertainment System | nes                        | NES, FC, FAMICOM                                |
| Odyssey                       | odyssey                    | *(none)*                                        |
| OpenBOR                       | openbor                    | *(none)*                                        |
| PC Engine SuperGrafx          | supergrafx                 | *(none)*                                        |
| PC-8000                       | pc-8000                    | *(none)*                                        |
| PC-9800 Series                | pc-9800-series             | PC98                                            |
| PC-FX                         | pc-fx                      | *(none)*                                        |
| Philips CD-i                  | philips-cd-i               | *(none)*                                        |
| Philips Videopac G7000        | videopac                   | *(none)*                                        |
| PICO-8                        | pico                       | *(none)*                                        |
| PICO-8                        | pico-8                     | PICO8                                           |
| PlayStation                   | psx                        | PSX, PS, PS1                                    |
| PlayStation 2                 | ps2                        | *(none)*                                        |
| PlayStation Portable          | psp                        | PSP                                             |
| Pokemon Mini                  | pokemon-mini               | *(none)*                                        |
| Ports                         | ports                      | PORTS                                           |
| Satellaview                   | satellaview                | *(none)*                                        |
| ScummVM                       | scummvm                    | *(none)*                                        |
| Sega 32X                      | sega32                     | 32X, SEGA32X, MD32X                             |
| Sega CD                       | segacd                     | SEGACD, MEGACD                                  |
| Sega Game Gear                | gamegear                   | GG                                              |
| Sega Genesis                  | genesis                    | GENESIS, MD, MEGADRIVE                          |
| Sega Master System            | sms                        | MS, SMS                                         |
| Sega Saturn                   | saturn                     | SATURN                                          |
| SG-1000                       | sg1000                     | *(none)*                                        |
| Sharp X1                      | x1                         | *(none)*                                        |
| Sharp X68000                  | sharp-x68000               | *(none)*                                        |
| Sufami Turbo                  | sufami-turbo               | *(none)*                                        |
| Super Famicom                 | sfam                       | SNES, SFC                                       |
| Super Game Boy                | sgb                        | GB                                              |
| Super Nintendo                | snes                       | SNES, SFC                                       |
| Supervision                   | supervision                | *(none)*                                        |
| TIC-80                        | tic-80                     | *(none)*                                        |
| TurboGrafx-16                 | tg16                       | PCE, TG16                                       |
| TurboGrafx-CD                 | turbografx-cd              | PCECD                                           |
| Vectrex                       | vectrex                    | VECTREX                                         |
| Virtual Boy                   | virtualboy                 | VB                                              |
| WonderSwan                    | wonderswan                 | WS                                              |
| WonderSwan Color              | wonderswan-color           | WSC                                             |
| ZX Spectrum                   | zxs                        | *(none)*                                        |
| ZX81                          | zx81                       | *(none)*                                        |
