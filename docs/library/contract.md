# Contract reference

Every method takes a JSON object of arguments and returns a JSON envelope. Hosts call methods through `grout_call` in `libgrout.so`. The `grout` CLI calls the same methods. This page is the protocol `1` contract.

## Envelope

A successful call returns:

```json
{ "ok": true, "result": { } }
```

A failed call returns:

```json
{ "ok": false, "error": { "code": "not_configured", "message": "no RomM host configured" } }
```

A call that did part of its work before failing, such as a refresh whose platforms step failed, returns `"ok": false` with both `error` and `result`.

Arguments are a JSON object. An empty string, `null`, or no arguments at all means every argument takes its default. An unknown field, trailing data or a value that is not an object fails with `bad_request`.

## Events

Long calls report progress through the event callback while they run. Each event carries the `call_id` the host passed to `grout_call`:

```json
{ "call_id": 7, "type": "progress", "stage": "games", "done": 120, "total": 450 }
```

`type` is always `progress` in protocol `1`. `stage` is one of `platforms`, `games`, `purge`, `collections` and `scan`, in that order. A refresh without a scan stops after `collections`.

## Error codes

| Code | Meaning |
|---|---|
| `not_configured` | No RomM host is set. |
| `bad_definition` | The firmware id is unknown, or the host definition failed validation. The message lists every problem. |
| `server_unreachable` | RomM could not be reached, or it answered with something that is not RomM. |
| `unauthorized` | RomM rejected the credentials. |
| `server_too_old` | The RomM server does not support save channel snapshots. |
| `busy` | A refresh or scan is already running on this handle. |
| `data_dir_locked` | Another handle or process has this data directory open. |
| `bad_request` | Unknown method, bad arguments, an unknown or closed handle, or a duplicate in-flight `call_id`. |
| `cancelled` | The call was cancelled with `grout_cancel` or by `grout_close`. |
| `storage` | The local database or a local file failed. |
| `internal` | A bug in Grout. The handle stays usable. |

## Value sets

| Field | Values |
|---|---|
| Game `kind` | `single`, `multi`, `nested` |
| Game `state` | `not_downloaded`, `partial`, `downloaded` |
| Collection `kind` | `regular`, `smart`, `virtual` |
| `downloaded_games` | `do_nothing`, `mark`, `filter` |
| `log_level` | `DEBUG`, `INFO`, `ERROR` |

## Shared shapes

**Settings**

```json
{
  "hosts": [{ "display_name": "", "root_uri": "https://romm.example", "port": 0, "username": "", "has_token": true, "insecure_skip_verify": false }],
  "directory_mappings": { "gba": { "slug": "gba", "relative_path": "" } },
  "show_regular_collections": false,
  "show_smart_collections": false,
  "show_virtual_collections": false,
  "downloaded_games": "do_nothing",
  "api_timeout": 30,
  "download_timeout": 3600,
  "log_level": "ERROR",
  "platform_order": []
}
```

Timeouts are whole seconds. The token is never returned; `has_token` says whether one is stored. `directory_mappings` is keyed by the platform's `fs_slug`. A platform is enabled when it has a mapping.

**Platform**

```json
{ "id": 4, "slug": "gba", "fs_slug": "gba", "name": "Game Boy Advance", "rom_count": 120, "enabled": true, "proposed": false, "rom_dirs": ["/mnt/SDCARD/Roms/GBA"] }
```

**GamesQuery**

```json
{ "platform_id": 4, "collection": { "kind": "regular", "remote_id": "12" }, "search": "zelda", "filters": { "genre": ["Adventure"] }, "hide_downloaded": false }
```

Every field is optional. `collection` may be `null`. Filter keys come from `library.dims`. Values within one key match any of them; separate keys must all match.

## Methods

### `status`

Arguments: none.

Result: `{ "config_source": "settings", "host": "https://romm.example", "server_version": "4.9.0", "snapshots": true }`. `config_source` is `settings`, `imported_v2` (settings were imported from Grout v2 on this open) or `new` (no settings file yet).

Errors: `not_configured`, `server_unreachable`, `unauthorized`, `server_too_old`.

### `settings.get`

Arguments: none.

Result: Settings.

Errors: none beyond `bad_request` for arguments.

### `settings.set_host`

Arguments: `{ "display_name", "root_uri", "port", "username", "token", "insecure_skip_verify" }`. `root_uri` must be an `http` or `https` URL. This replaces any host already set.

Result: Settings.

Errors: `bad_request`, `storage`.

### `settings.set_directory_mapping`

Arguments: `{ "fs_slug", "slug", "relative_path", "enabled" }`. `enabled: false` removes the mapping, which disables the platform. An empty `relative_path` means the firmware's own folder for that platform.

Result: Settings.

Errors: `bad_request` (including an empty `fs_slug`), `storage`.

### `library.refresh`

Arguments: `{ "full": false, "scan": false }`. `full` ignores the stored cursors and fetches everything again. `scan` runs `library.scan` afterwards.

Result: `{ "refresh": { "platforms", "games", "collections", "deleted", "failed_platforms": [], "collection_errors": [], "purge_error": "" }, "scan": ScanResult or null }`.

Platforms or collections that failed are listed in the result and do not fail the call.

Events: `progress`.

Errors: `not_configured`, `server_unreachable`, `unauthorized`, `server_too_old`, `busy`, `cancelled`, `storage`.

### `library.scan`

Arguments: none. Works without a RomM host.

Result: ScanResult, `{ "files", "matched", "platforms", "skipped_folders": [] }`. Folders that could not be read are listed and skipped.

Events: `progress` with stage `scan`.

Errors: `busy`, `cancelled`, `storage`.

### `library.platforms`

Arguments: `{ "all": false }`. Without `all`, only enabled platforms that have games are returned.

Result: `[Platform]`.

Errors: `cancelled`, `storage`.

### `library.games`

Arguments: GamesQuery plus `{ "offset": 0, "limit": 0 }`. `limit` 0 means all.

Result: `{ "total": 3, "games": [{ "id", "platform_id", "name", "kind", "state" }] }`. `total` counts every match before paging.

Errors: `bad_request` for an unknown filter key, an unknown collection `kind`, or a negative offset or limit; `cancelled`; `storage`.

### `library.filter_values`

Arguments: `{ "query": GamesQuery, "dim": "genre" }`.

Result: `[string]`, the values of `dim` among games that match every other part of the query.

Errors: `bad_request` for an unknown `dim` or collection `kind`; `cancelled`; `storage`.

### `library.dims`

Arguments: none.

Result: `["genre", "franchise", "company", "game_mode", "age_rating", "region", "language", "tag"]`.

Errors: none beyond `bad_request` for arguments.

### `library.collections`

Arguments: none. Only collection kinds turned on in Settings are returned, and only collections with games on enabled platforms. Protocol `1` has no method to turn collection kinds on yet, so hosts get `[]` until a later release adds one.

Result: `[{ "kind", "remote_id", "name", "games" }]`.

Errors: `cancelled`, `storage`.

### `library.collection_platforms`

Arguments: `{ "kind", "remote_id" }`.

Result: `[Platform]`, the platforms holding that collection's games, with per-platform counts in `rom_count`.

Errors: `bad_request` for an unknown collection `kind`; `cancelled`; `storage`.

### `library.game_state`

Arguments: `{ "game_id" }`.

Result: `{ "state" }`.

Errors: `bad_request` for an unknown game; `cancelled`; `storage`.
