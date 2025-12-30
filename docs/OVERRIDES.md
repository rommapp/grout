Act# Override Files

Grout allows you to override embedded JSON configuration files with local copies. This is useful for testing, customization, or adding support for new platforms/devices without recompiling the application.

## How It Works

When Grout loads configuration files at startup, it checks for override files in the `overrides/` directory relative to the current working directory. If an override file exists, it will be used instead of the embedded version. If no override exists, Grout falls back to the embedded file.

## Directory Structure

Override files must match the exact path structure of the embedded files:

```
overrides/
├── bios/
│   ├── core_requirements.json
│   ├── core_subdirectories.json
│   └── platform_cores.json
└── cfw/
    ├── muos/
    │   ├── platforms.json
    │   ├── save_directories.json
    │   ├── art_directories.json
    │   └── input_mappings/
    │       ├── anbernic.json
    │       ├── trimui_brick.json
    │       └── trimui_smart_pro.json
    ├── nextui/
    │   ├── platforms.json
    │   └── save_directories.json
    └── knulli/
        └── platforms.json
```

## Use Cases

### Testing New Platform Mappings

To test adding a new platform without recompiling:

1. Copy the relevant embedded file to the override directory:
   ```bash
   mkdir -p overrides/cfw/muos
   # Copy current embedded content or start fresh
   ```

2. Edit `overrides/cfw/muos/platforms.json` to add your new platform mapping

3. Run Grout from the directory containing the `overrides/` folder

### Custom BIOS Requirements

To add or modify BIOS file requirements for a specific core:

1. Create `overrides/bios/core_requirements.json`

2. Add or modify core entries with their BIOS file requirements

3. Grout will use your custom BIOS configuration

### Device-Specific Input Mappings

To test custom input mappings for muOS devices:

1. Create `overrides/cfw/muos/input_mappings/anbernic.json` (or your device)

2. Modify the button/axis mappings as needed

3. Test with your device

## Important Notes

### File Format

Override files must:
- Be valid JSON matching the structure of the embedded files
- Use the exact same schema and field names
- Maintain proper JSON syntax (trailing commas will cause errors)

### Case Sensitivity

When working with BIOS files:
- Filenames in JSON should match the exact case expected by emulators
- Grout performs case-insensitive matching when looking up files
- Files are saved using the exact case specified in the JSON

Example:
```json
{
  "FileName": "ATARIBAS.ROM",
  "RelativePath": "ATARIBAS.ROM"
}
```
Will be saved as `ATARIBAS.ROM` (uppercase) regardless of how RomM returns the filename.

### Working Directory

Overrides are loaded relative to the **current working directory** where Grout is executed, not the binary location.

If Grout is installed at `/mnt/mmc/MUOS/application/grout/grout`, you should place overrides at:
```
/mnt/mmc/MUOS/application/grout/overrides/
```

And run Grout from `/mnt/mmc/MUOS/application/grout/`.

### No Runtime Reloading

Override files are loaded once at application startup. To apply changes:
1. Exit Grout completely
2. Modify the override file
3. Restart Grout

### Validation

Grout does not perform extensive validation on override files. Invalid JSON or incorrect data structures may cause:
- Application crashes at startup
- Runtime errors
- Unexpected behavior

Always validate your JSON before using it as an override.

### Debugging

To verify which files are being loaded, check the application logs. When an override is found and used, Grout will load it silently, falling back to embedded files only when overrides don't exist.

## Examples

### Example: Adding a New Platform to muOS

Create `overrides/cfw/muos/platforms.json`:

```json
{
  "3do": ["3DO"],
  "amiga": ["AMIGA"],
  "customplatform": ["CUSTOM", "CUSTOM-ALT"],
  "gba": ["GBA", "GB"]
}
```

This adds support for "customplatform" which maps to the `CUSTOM` or `CUSTOM-ALT` directories on your device.

### Example: Custom BIOS Entry

Create `overrides/bios/core_requirements.json`:

```json
{
  "mgba": {
    "CoreName": "mgba_libretro",
    "DisplayName": "Nintendo - Game Boy Advance (mGBA)",
    "Files": [
      {
        "FileName": "gba_bios.bin",
        "RelativePath": "gba_bios.bin",
        "MD5Hash": "a860e8c0b6d573d191e4ec7db1b1e4f6",
        "Optional": false
      },
      {
        "FileName": "custom_bios.bin",
        "RelativePath": "custom/custom_bios.bin",
        "MD5Hash": "",
        "Optional": true
      }
    ]
  }
}
```

### Example: Custom Input Mapping

Create `overrides/cfw/muos/input_mappings/anbernic.json`:

```json
{
  "keyboard_map": {},
  "controller_button_map": {},
  "controller_hat_map": {},
  "joystick_axis_map": {},
  "joystick_button_map": {
    "0": 4,
    "3": 5,
    "4": 6,
    "5": 8,
    "6": 7,
    "7": 9,
    "12": 10,
    "8": 11,
    "13": 12,
    "10": 13,
    "9": 14,
    "14": 15
  },
  "joystick_hat_map": {
    "1": 1,
    "4": 2,
    "8": 3,
    "2": 4
  }
}
```

## Contributing Overrides Back

If you create useful overrides that add support for new platforms, devices, or fix issues:

1. Test thoroughly to ensure they work correctly
2. Submit a pull request with your changes to the embedded JSON files
3. Include details about what you're adding and why

This helps the entire Grout community benefit from your work!
