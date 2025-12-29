# Grout State Machine

This document shows the navigation flow between screens in Grout.

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

## Settings Flow

```mermaid
flowchart TD
    PS[Platform Selection]
    SET[Settings]
    CSET[Collections Settings]
    SSSET[Save Sync Settings]
    ASET[Advanced Settings]

    PS -->|"Settings"| SET
    SET -->|"Save/Back"| PS
    SET --> CSET
    SET --> SSSET
    SET --> ASET

    CSET --> SET
    SSSET --> SET
    ASET --> SET
```

## Advanced Settings Flow

```mermaid
flowchart TD
    SET[Settings]
    ASET[Advanced Settings]
    PM[Platform Mapping]
    CC[Clear Cache]
    ART[Artwork Sync]
    INFO[Info]
    LOGOUT[Logout Confirm]
    PS[Platform Selection]

    SET --> ASET
    ASET -->|"Back"| SET
    ASET --> PM
    ASET --> CC
    ASET --> ART
    ASET --> INFO

    PM --> ASET
    CC --> ASET
    ART --> ASET

    INFO -->|"Back"| SET
    INFO -.->|"Back (from adv)"| ASET
    INFO --> LOGOUT

    LOGOUT -->|"Cancel"| INFO
    LOGOUT -->|"Confirm"| PS
```

## State Descriptions

| State | Description |
|-------|-------------|
| Platform Selection | Main menu showing platforms and collections |
| Game List | List of games for selected platform/collection |
| Game Details | Detailed view with metadata and download |
| Game Options | Per-game settings (save directory) |
| Search | On-screen keyboard for game search |
| Collection List | List of available collections |
| Collection Platform Selection | Platform filter within a collection |
| Collection Search | On-screen keyboard for collection search |
| Settings | Main settings menu |
| Collections Settings | Collection display options |
| Save Sync Settings | Per-platform save directory config |
| Advanced Settings | Timeouts, cache, mappings |
| Platform Mapping | Configure ROM directory mappings |
| Clear Cache | Confirm cache clearing |
| Artwork Sync | Pre-cache artwork for all games |
| Info | App info and logout option |
| Logout Confirmation | Confirm logout action |
| Save Sync | Manual save synchronization |
| BIOS Download | Download BIOS files |

## Navigation State

The FSM maintains state in a single `NavState` struct:

```go
type NavState struct {
    // Game browsing
    CurrentGames, FullGames []romm.Rom
    SearchFilter            string
    HasBIOS                 bool
    GameListPos             ListPosition

    // Collections
    CollectionSearchFilter  string
    CollectionGames         []romm.Rom
    CollectionListPos       ListPosition
    CollectionPlatformPos   ListPosition

    // List positions
    PlatformListPos         ListPosition
    SettingsPos             ListPosition
    CollectionsSettingsPos  ListPosition
    AdvancedSettingsPos     ListPosition

    // Flags
    QuitOnBack, ShowCollections bool
    InfoPreviousState           gaba.StateName
}
```
