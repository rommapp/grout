# State Machine Reference

This document shows the navigation flow between screens in Grout.

---

## Overview

```mermaid
flowchart LR
    PS[Platform Selection] -->|"Select Platform"| GL[Game List] -->|"Select Game"| GD[Game Details]
```

## Platform Selection

```mermaid
flowchart LR
    PS[Platform Selection]
    PS -->|"Select Platform"| GL[Game List]
    PS -->|"Collections"| COLL[["Collections Flow"]]
    PS -->|"Settings"| SETT[["Settings Flow"]]
    PS -->|"Save Sync"| SS[Save Sync]
    PS -->|"Quit"| EXIT((Exit))
    SS --> PS
```

## Game List

```mermaid
flowchart LR
    GL[Game List]
    GL -->|"Select Game"| GD[Game Details]
    GL -->|"Search"| S[Search] --> GL
    GL -->|"Filters"| GF[Game Filters]
    GF -->|"Apply"| GL
    GF -->|"Cancel/Clear"| GL
    GL -->|"BIOS"| BIOS[BIOS Download] --> GL
    GL -->|"Back"| PS[Platform Selection]
```

## Game Details

```mermaid
flowchart LR
    GD[Game Details]
    GD -->|"Download"| GL[Game List]
    GD -->|"Options"| GO[Game Options] --> GD
    GD -->|"Back"| GL
```

---

## Collections Flow

```mermaid
flowchart TD
    PS[Platform Selection]
    CL[Collection List]
    CPS[Collection Platform Selection]
    GL[Game List]
    CS[Collection Search]

    PS -->|"Collections"| CL
    CL -->|"Select"| CPS
    CL -->|"Search"| CS
    CL -->|"Back"| PS

    CPS -->|"Select Platform"| GL
    CPS -->|"Back"| CL

    CS --> CL

    GL -->|"Back"| CPS
    GL -.->|"Back (unified)"| CL
```

---

## Settings Flow

```mermaid
flowchart TD
    PS[Platform Selection]
    SET[Settings]
    GSET[General Settings]
    CSET[Collections Settings]
    SSSET[Save Sync Settings]
    ASET[Advanced Settings]
    PM[Platform Mapping]
    INFO[Info]
    UPD[Update Check]
    LOGOUT[Logout Confirm]

    PS -->|"Settings"| SET
    SET -->|"Save/Back"| PS
    SET --> GSET
    SET --> CSET
    SET --> SSSET
    SET --> ASET
    SET --> PM
    SET --> INFO
    SET --> UPD

    GSET --> SET
    CSET --> SET
    SSSET --> SET
    ASET --> SET
    PM --> SET
    UPD --> SET

    INFO -->|"Back"| SET
    INFO --> LOGOUT

    LOGOUT -->|"Cancel"| INFO
    LOGOUT -->|"Confirm"| PS
```

---

## Advanced Settings Flow

```mermaid
flowchart TD
    SET[Settings]
    ASET[Advanced Settings]
    RC[Refresh Cache]
    ART[Artwork Sync]

    SET --> ASET
    ASET -->|"Back"| SET
    ASET --> RC
    ASET --> ART

    RC --> ASET
    ART --> ASET
```

---

## State Descriptions

| State                         | Description                                                                                                                                                          |
|-------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Platform Selection            | Main menu showing platforms and collections                                                                                                                          |
| Game List                     | List of games for selected platform/collection                                                                                                                       |
| Game Details                  | Detailed view with metadata and download                                                                                                                             |
| Game Options                  | Per-game settings (save directory)                                                                                                                                   |
| Game Filters                  | Filter games by genre, franchise, platform, etc. Changing a filter dynamically updates available options for other filters and clears selections that become invalid |
| Search                        | On-screen keyboard for game search                                                                                                                                   |
| Collection List               | List of available collections                                                                                                                                        |
| Collection Platform Selection | Platform filter within a collection                                                                                                                                  |
| Collection Search             | On-screen keyboard for collection search                                                                                                                             |
| Settings                      | Main settings menu                                                                                                                                                   |
| General Settings              | Box art, download behavior, language                                                                                                                                 |
| Collections Settings          | Collection display options                                                                                                                                           |
| Save Sync Settings            | Save sync mode and per-platform config                                                                                                                               |
| Advanced Settings             | Timeouts and cache management                                                                                                                                        |
| Platform Mapping              | Configure ROM directory mappings                                                                                                                                     |
| Refresh Cache                 | Select and refresh cache types                                                                                                                                       |
| Artwork Sync                  | Pre-cache artwork for all games                                                                                                                                      |
| Info                          | App info (version, CFW, RomM version) and logout option                                                                                                              |
| Update Check                  | Check for and install updates                                                                                                                                        |
| Logout Confirmation           | Confirm logout action                                                                                                                                                |
| Save Sync                     | Manual save synchronization                                                                                                                                          |
| BIOS Download                 | Download BIOS files                                                                                                                                                  |
