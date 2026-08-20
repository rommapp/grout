# Anbernic Stock OS

The original Anbernic firmware (the "stock OS") that ships preinstalled on the Allwinner H700
handhelds — RG35XX Plus / H / SP / 2024, RG34XX, RG40XX, RG28XX and CubeXX. No custom firmware
required.

## Quirks

- **Two card slots**: Grout uses the card it was installed on. TF1 is mounted at `/mnt/mmc` and TF2
  at `/mnt/sdcard`; the launch script exports `BASE_PATH` from its own location, so a copy in
  `TF2:/Roms/APPS/` manages the ROMs on TF2.
- **Downloaded games not visible**: after downloading a game, back out of the platform list and
  re-enter it for the stock launcher to pick up the new files.
- **Saves**: the stock firmware ships a patched RetroArch that redirects saves for content under
  `/mnt/mmc` or `/mnt/sdcard` to `saves_RA/` and save states to `states_RA/`, mirroring the ROM's
  path. A save for `Roms/GBA/Game.gba` therefore lives at `saves_RA/Roms/GBA/Game.srm`. Saves
  written by the standalone emulators (PPSSPP, DraStic, OpenBOR) live under `save/` instead and are
  not synced.
- **BIOS**: BIOS files go to RetroArch's system directory on the `appfs` partition
  (`/mnt/vendor/deep/retro/system`), which already ships with most of the common BIOS set.

## Platform Mappings

This table shows the mappings of RomM Fs Slug to the Anbernic stock OS platform folders.

| Platform Name                 | RomM Fs Slug               | Folder(s)                                       |
|-------------------------------|----------------------------|-------------------------------------------------|
| 3DO Interactive Multiplayer   | 3do                        | *(none)*                                        |
| Amiga                         | amiga                      | AMIGA                                           |
| Amstrad CPC                   | acpc                       | *(none)*                                        |
| Arcade                        | arcade                     | MAME, FBNEO, HBMAME, PGM2, VARCADE              |
| Arduboy                       | arduboy                    | *(none)*                                        |
| Atari 2600                    | atari2600                  | A2600                                           |
| Atari 5200                    | atari5200                  | A5200                                           |
| Atari 7800                    | atari7800                  | A7800                                           |
| Atari 800                     | atari800                   | A800                                            |
| Atari Jaguar                  | jaguar                     | *(none)*                                        |
| Atari Lynx                    | lynx                       | LYNX                                            |
| Atari ST/STE                  | atari-st                   | ATARIST                                         |
| Capcom Play System 1          | cps1                       | CPS1                                            |
| Capcom Play System 2          | cps2                       | CPS2                                            |
| Capcom Play System 3          | cps3                       | CPS3                                            |
| Cave Story                    | cave-story                 | *(none)*                                        |
| ColecoVision                  | colecovision               | *(none)*                                        |
| Commodore 128                 | c128                       | *(none)*                                        |
| Commodore C64/128/MAX         | c64                        | C64                                             |
| Commodore PET                 | cpet                       | *(none)*                                        |
| Commodore VIC-20              | vic-20                     | VIC20                                           |
| DOS                           | dos                        | DOS                                             |
| Dreamcast                     | dc                         | DREAMCAST                                       |
| Fairchild Channel F           | fairchild-channel-f        | *(none)*                                        |
| Family Computer               | famicom                    | FC                                              |
| Family Computer Disk System   | fds                        | FDS                                             |
| Galaksija                     | galaksija                  | *(none)*                                        |
| Game & Watch                  | g-and-w                    | GW                                              |
| Game Boy                      | gb                         | GB                                              |
| Game Boy Advance              | gba                        | GBA                                             |
| Game Boy Color                | gbc                        | GBC                                             |
| Intellivision                 | intellivision              | *(none)*                                        |
| J2ME                          | j2me                       | *(none)*                                        |
| Mega Duck/Cougar Boy          | mega-duck-slash-cougar-boy | *(none)*                                        |
| MSX                           | msx                        | MSX                                             |
| Neo Geo AES                   | neogeoaes                  | NEOGEO                                          |
| Neo Geo CD                    | neo-geo-cd                 | NEOCD                                           |
| Neo Geo MVS                   | neogeomvs                  | NEOGEO                                          |
| Neo Geo Pocket                | neo-geo-pocket             | NGP                                             |
| Neo Geo Pocket Color          | neo-geo-pocket-color       | NGP                                             |
| Nintendo 3DS                  | 3ds                        | *(none)*                                        |
| Nintendo 64                   | n64                        | N64                                             |
| Nintendo DS                   | nds                        | NDS                                             |
| Nintendo Entertainment System | nes                        | FC                                              |
| Odyssey                       | odyssey                    | *(none)*                                        |
| OpenBOR                       | openbor                    | OPENBOR                                         |
| PC Engine SuperGrafx          | supergrafx                 | PCE                                             |
| PC-8000                       | pc-8000                    | *(none)*                                        |
| PC-9800 Series                | pc-9800-series             | *(none)*                                        |
| PC-FX                         | pc-fx                      | *(none)*                                        |
| Philips CD-i                  | philips-cd-i               | *(none)*                                        |
| PICO-8                        | pico                       | PICO                                            |
| PICO-8                        | pico-8                     | PICO                                            |
| PlayStation                   | psx                        | PS                                              |
| PlayStation 2                 | ps2                        | *(none)*                                        |
| PlayStation Portable          | psp                        | PSP                                             |
| Pokemon Mini                  | pokemon-mini               | POKE                                            |
| Ports                         | ports                      | PORTS                                           |
| Satellaview                   | satellaview                | *(none)*                                        |
| ScummVM                       | scummvm                    | SCUMMVM                                         |
| Sega 32X                      | sega32                     | SEGA32X                                         |
| Sega CD                       | segacd                     | MDCD                                            |
| Sega Game Gear                | gamegear                   | GG                                              |
| Sega Genesis                  | genesis                    | MD                                              |
| Sega Master System            | sms                        | SMS                                             |
| Sega Saturn                   | saturn                     | SATURN                                          |
| SG-1000                       | sg1000                     | *(none)*                                        |
| Sharp X1                      | x1                         | *(none)*                                        |
| Sharp X68000                  | sharp-x68000               | *(none)*                                        |
| Sufami Turbo                  | sufami-turbo               | *(none)*                                        |
| Super Famicom                 | sfam                       | SFC                                             |
| Super Game Boy                | sgb                        | *(none)*                                        |
| Super Nintendo                | snes                       | SFC                                             |
| Supervision                   | supervision                | *(none)*                                        |
| TIC-80                        | tic-80                     | *(none)*                                        |
| TurboGrafx-16                 | tg16                       | PCE                                             |
| TurboGrafx-CD                 | turbografx-cd              | PCECD                                           |
| Vectrex                       | vectrex                    | *(none)*                                        |
| VideoPac                      | videopac                   | *(none)*                                        |
| Virtual Boy                   | virtualboy                 | VB                                              |
| WonderSwan                    | wonderswan                 | WS                                              |
| WonderSwan Color              | wonderswan-color           | WS                                              |
| ZX Spectrum                   | zxs                        | *(none)*                                        |
| ZX81                          | zx81                       | *(none)*                                        |
