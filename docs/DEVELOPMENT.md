# Grout Development Guide

> [!NOTE]
> These instructions were written with macOS in mind. It should work elsewhere, but we have
> not personally verified this.

We hang out over in
the [Grout Development Channel](https://discord.com/channels/1138838206532554853/1456747141518069906) on
the [RomM Discord](https://discord.gg/P5HtHnhUDH). Come join us!

---

## Prerequisites

### Local Development

- [Go](https://go.dev) v1.24+
- [Task](https://taskfile.dev) (for running the handy build scripts)
- SDL2 Shared Libraries (can be installed via [Homebrew](https://brew.sh))

```shell
brew install sdl2 sdl2_image sdl2_ttf sdl2_gfx
```

### Local Builds / Packaging

- [Docker Desktop](https://www.docker.com/products/docker-desktop/) or equivalent
    - We love [OrbStack](https://orbstack.dev) (not a sponsor)

---

## Getting Started

1. Clone the [Grout](https://github.com/rommapp/grout) repository.
2. Make a copy of `.env.dev` and save it as `.env` in the root of the cloned repository.
3. Fill out the `.env` file. Here are descriptions of the various values you can set.
    - `ENVIRONMENT=DEV` (mandatory), this will disable some Gabagool features behind the scenes
    - `WINDOW_WIDTH` (optional)
    - `WINDOW_HEIGHT` (optional)
    - `NITRATES` [true | false] (optional) This is used for Gabagool development debugging
    - `CFW` [MUOS | KNULLI | SPRUCE | NEXTUI] (mandatory), this controls how Grout interacts with and places files
    - `BASE_PATH` (mandatory), this acts as the root path like you would have on a handheld (e.g. `/mmc/sdcard` on
      muOS). Have the subdirectory structure of this path match the CFW you are working on.
4. Run / Debug `app/grout.go`, making sure to reference the `.env` file in your run configuration.

---

## Project Structure

The codebase is laid out fairly well. It attempts to keep everything grouped by feature / function / domain.

- `app` contains the glue, the main function, finite state machine for screen transitions, and the setup / cleanup code.
- `bios` handles BIOS file operations
- `cache` contains the logic for the SQLite database that powers the local cache
- `cfw` contains all the ~~magic~~ logic for adapting Grout to the various CFWs that are supported
- `docs` for the user guide and other repo housekeeping, including this document!
- `internal` the college educated utils package. App-wide / stateless utilities live here
- `resources` the splash screen image and localization files live here, along with the go file that embeds them
- `romm` a client library for the RomM API.
    - Why wasn't this generated with the OpenAPI spec? We tried a number of the codegen tools for OpenAPI and they
      weren't compatible with version 3 of the spec and hacking around this limitation produced frustrating to use code.
- `scripts` contains the scripts (and metadata) associated with creating a package for each CFW
- `sync` contains the save sync functionality
- `ui` contains the screens that the FSM references in `app/states.go`
- `update` handles the in-app updater functionality, excluding the UI
- `version` exposes the version information that is injected at build time. Having it as its own package made the script
  cleaner.

---

## Packaging

The `Taskfile.yml` defines scripts for building and packaging Grout for various CFWs. All builds target ARM64 Linux
and use Docker for cross-compilation.

### Quick Start

```shell
# Build and package for all platforms
task all

# Or build with a local gabagool workspace (for gabagool development)
task all-local
```

### Build Process

The build happens in two stages:

1. **Docker Build** (`task build`) - Cross-compiles the Go binary for ARM64 Linux inside a Docker container. This
   ensures consistent builds regardless of your host OS and handles SDL2 dependencies.

2. **Extract** (`task extract`) - Copies the compiled binary and required shared libraries (like `libSDL2_gfx`) from
   the Docker container to the local `build/` directory.

### Platform-Specific Packaging

After building, you can package for individual platforms:

| Task                  | Platform        | Output Location                           |
|-----------------------|-----------------|-------------------------------------------|
| `task package-next`   | NextUI (TrimUI) | `build/Grout.pak/`                        |
| `task package-muos`   | muOS            | `build/muOS/Grout/`, `build/Grout.muxapp` |
| `task package-knulli` | Knulli          | `build/Knulli/Grout/`                     |
| `task package-spruce` | Spruce          | `build/Spruce/Grout/`                     |

Each packaging task copies the binary, launch scripts from `scripts/<platform>/`, shared libraries, and documentation
into the appropriate directory structure for that CFW.

### Deployment via ADB

For rapid testing, you can deploy directly to a connected device:

```shell
# NextUI (TrimUI devices)
task adb-next

# muOS (SD card 1 or 2)
task adb-muos-sd1
task adb-muos-sd2

# Knulli
task adb-knulli
```

These tasks will remove any existing installation and push the freshly built package to the device.

### Local Gabagool Development

When developing gabagool alongside Grout, use the `-local` variants:

```shell
task build-local   # Build using local gabagool via go.work
task all-local     # Build and package all platforms with local gabagool
```

This requires a `go.work` file in the parent directory that references both projects.

### Output Structure

After running `task all`, the `build/` directory will contain:

```
build/
├── grout              # ARM64 Linux binary
├── lib/               # Shared libraries
│   └── libSDL2_gfx-1.0.so.0
├── Grout.pak/         # NextUI package
├── Grout.muxapp       # muOS archive (ready to install)
├── muOS/Grout/        # muOS package (unpacked)
├── Knulli/Grout/      # Knulli package
└── Spruce/Grout/      # Spruce package
```

---

## Helper Tools

The `Taskfile.yml` includes several utility tasks for common development workflows.

### Internationalization (i18n)

Grout uses [go-i18n](https://github.com/nicksnyder/go-i18n) for localization. The workflow for updating translations:

```shell
# Extract new messages and find missing translations (recommended)
task i18n

# Or run steps individually:
task i18n:extract  # Extract messages from source to active.en.toml
task i18n:merge    # Compare against other locales, output missing to translations_todo/
```

The `i18n` task will:

1. Scan the codebase for translatable strings and update `resources/locales/active.en.toml`
2. Compare against each locale (es, fr, de, it, pt, ja, ru) to find missing translations
3. Output any missing translations to `translations_todo/<lang>.toml`
4. Simplify the locale files to a clean `key = "value"` format

### Code Quality

```shell
# Run all linters (fmt, vet, staticcheck)
task lint
```

This runs `go fmt`, `go vet`, and `staticcheck` across the codebase. Requires
[staticcheck](https://staticcheck.dev/) to be installed (`go install honnef.co/go/tools/cmd/staticcheck@latest`).

### Media Conversion

```shell
# Convert MP4 video to animated WebP (interactive, prompts for paths)
task mp4-to-webp

# Resize all user guide screenshots to 1024px width
task resize-user-guide-images
```

The `mp4-to-webp` task is useful for creating animated preview images for documentation.

The `resize-user-guide-images` makes sure all the user guide screenshots are the same size.

---

Happy Developing ❤️
