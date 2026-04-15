# vendored/pspdb

This directory contains a vendored copy of the PSP game title database.

## Source

| Field   | Value                                                                                         |
|---------|-----------------------------------------------------------------------------------------------|
| Project | [GameDB-PSP](https://github.com/niemasd/GameDB-PSP) by [@niemasd](https://github.com/niemasd) |
| File    | `PSP.titles.json`                                                                             |
| URL     | https://github.com/niemasd/GameDB-PSP/releases/latest/download/PSP.titles.json                |
| License | [GNU General Public License v3.0](https://github.com/niemasd/GameDB-PSP/blob/main/LICENSE)    |

## Why vendored?

The file is kept here to ensure the code generator (`tools/gen-psp-rom-list`) can always run
offline and is not affected by upstream repository deletions, renames, or release changes.

## Updating

To refresh the vendored file with the latest upstream release:

```bash
curl -sL https://github.com/niemasd/GameDB-PSP/releases/latest/download/PSP.titles.json \
     -o vendored/pspdb/PSP.titles.json
```

Then regenerate the Go package:

```bash
go generate ./internal/pspdb/
```