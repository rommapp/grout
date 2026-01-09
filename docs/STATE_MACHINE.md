# Grout State Machine

This document shows the navigation flow between screens in Grout.

---

## Main Flow

```mermaid
flowchart TD
    PS[Platform Selection]
    GL[Game List]
    GD[Game Details]

    PS -->|"Select Platform"| GL
    PS -->|"Collections"| COLL[["Collections Flow"]]
    PS -->|"Settings"| SETT[["Settings Flow"]]
    PS -->|"Save Sync"| SS[Save Sync]
    PS -->|"Quit"| EXIT((Exit))

    GL -->|"Select Game"| GD
    GL -->|"Search"| S[Search]
    GL -->|"BIOS"| BIOS[BIOS Download]
    GL -->|"Back"| PS

    GD -->|"Download"| GL
    GD -->|"Options"| GO[Game Options]
    GD -->|"Back"| GL

    GO --> GD
    S --> GL
    SS --> PS
    BIOS --> GL
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

| State                         | Description                                    |
|-------------------------------|------------------------------------------------|
| Platform Selection            | Main menu showing platforms and collections    |
| Game List                     | List of games for selected platform/collection |
| Game Details                  | Detailed view with metadata and download       |
| Game Options                  | Per-game settings (save directory)             |
| Search                        | On-screen keyboard for game search             |
| Collection List               | List of available collections                  |
| Collection Platform Selection | Platform filter within a collection            |
| Collection Search             | On-screen keyboard for collection search       |
| Settings                      | Main settings menu                             |
| General Settings              | Box art, download behavior, language           |
| Collections Settings          | Collection display options                     |
| Save Sync Settings            | Save sync mode and per-platform config         |
| Advanced Settings             | Timeouts and cache management                  |
| Platform Mapping              | Configure ROM directory mappings               |
| Refresh Cache                 | Select and refresh cache types                 |
| Artwork Sync                  | Pre-cache artwork for all games                |
| Info                          | App info and logout option                     |
| Update Check                  | Check for and install updates                  |
| Logout Confirmation           | Confirm logout action                          |
| Save Sync                     | Manual save synchronization                    |
| BIOS Download                 | Download BIOS files                            |

---

## Navigation State

The FSM maintains state in a `NavState` struct:

```go
type NavState struct {
    CurrentGames []romm.Rom
    FullGames    []romm.Rom
    SearchFilter string
    HasBIOS      bool
    GameListPos  ListPosition

    CollectionSearchFilter string
    CollectionGames        []romm.Rom
    CollectionListPos      ListPosition
    CollectionPlatformPos  ListPosition

    PlatformListPos ListPosition

    SettingsPos            ListPosition
    CollectionsSettingsPos ListPosition
    AdvancedSettingsPos    ListPosition

    QuitOnBack      bool
    ShowCollections bool
}
```
