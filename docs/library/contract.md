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
| `busy` | A refresh or scan, or an `auth.pair_wait`, is already running on this handle. |
| `data_dir_locked` | Another handle or process has this data directory open. |
| `bad_request` | Unknown method, bad arguments, an unknown or closed handle, or a duplicate in-flight `call_id`. |
| `cancelled` | The call was cancelled with `grout_cancel` or by `grout_close`. |
| `storage` | The local database or a local file failed. |
| `internal` | A bug in Grout. The handle stays usable. |
| `invalid_code` | The pairing code is unknown, already used or expired. |
| `rate_limited` | RomM is limiting requests from this device. Wait a minute and try again. |

## Value sets

| Field | Values |
|---|---|
| Game `kind` | `single`, `multi`, `nested` |
| Game `state` | `not_downloaded`, `partial`, `downloaded` |
| Collection `kind` | `regular`, `smart`, `virtual` |
| `downloaded_games` | `do_nothing`, `mark`, `filter` |
| `log_level` | `DEBUG`, `INFO`, `ERROR` |
| Pairing `outcome` | `approved`, `denied`, `expired` (a pairing that fails returns the `server_unreachable` error instead) |

## Shared shapes

**Settings**

```json
{
  "hosts": [{ "display_name": "", "root_uri": "https://romm.example", "port": 0, "username": "ana", "has_token": true, "token_name": "Living room RG35XX", "token_expires_at": "2027-11-20T18:00:00Z", "device_id": "c1f0...", "device_name": "Living room RG35XX", "insecure_skip_verify": false }],
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

Timeouts are whole seconds. The token is never returned; `has_token` says whether one is stored. `token_expires_at` is RFC 3339 in UTC, for example `2027-11-20T18:00:00Z`, and empty for a token that never expires. `device_id` is set once the device is registered with RomM, by pairing or by `device.register`. `directory_mappings` is keyed by the platform's `fs_slug`. A platform is enabled when it has a mapping.

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

Errors: `not_configured`, `server_unreachable`, `unauthorized`, `server_too_old`, `rate_limited`.

### `settings.get`

Arguments: none.

Result: Settings.

Errors: none beyond `bad_request` for arguments.

### `settings.set_host`

Arguments: `{ "display_name", "root_uri", "port", "username", "token", "insecure_skip_verify" }`. `root_uri` must be an `http` or `https` URL. This replaces the server settings. When `root_uri` and `port` name the same server as before and no `token` is passed, the sign-in (token, username and device) is kept, so a host can call this on every launch. Pointing at a different server forgets the sign-in unless a new `token` is passed. The device identifier used for pairing is always kept.

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

Errors: `not_configured`, `server_unreachable`, `unauthorized`, `server_too_old`, `rate_limited`, `busy`, `cancelled`, `storage`.

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

Arguments: none. Only collection kinds turned on in Settings are returned, and only collections with games on enabled platforms. Turn kinds on with `settings.update`.

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

### `auth.pair_start`

Arguments: `{ "device_name" }`, the name RomM shows for this device. Required.

Result: `{ "user_code": "ABCD2345", "verification_url": "https://romm.example/pair/device?user_code=ABCD2345", "expires_at": "2026-11-20T18:15:00Z" }`. Show the code and a QR code of `verification_url`; the user approves the device in RomM.

Starting a new pairing replaces any earlier one on the handle. Changing the server with `settings.set_host` drops a pending pairing.

Starting a pairing while an `auth.pair_wait` for an earlier one is still running leaves that wait running; cancel it first.

Errors: `not_configured` (no host set), `bad_request` (empty name), `server_unreachable`, `server_too_old`, `rate_limited`, `cancelled`, `storage`.

### `auth.pair_wait`

Arguments: none.

Waits until the user approves or denies the pairing, or it expires, then returns. Cancel it with `grout_cancel` when the user backs out; the pending pairing is kept, so calling `auth.pair_wait` again resumes it.

Result: `{ "outcome": "approved", "username": "ana", "device_id": "c1f0...", "token_expires_at": "" }`. `denied` and `expired` are results too, with the other fields empty, and end the pairing. On `approved` Grout stores the token and device; nothing else is needed before calling library methods.

Errors: `bad_request` (no pending pairing, or the server changed since `auth.pair_start`), `busy` (another `auth.pair_wait` is running), `cancelled`, `server_unreachable` (RomM could not be reached three times in a row, or it approved without a token), `storage`.

### `auth.exchange_code`

Arguments: `{ "code" }`, a pairing code made in RomM's web UI. Case, spaces and dashes do not matter.

Result: `{ "username", "device_id", "token_expires_at", "scopes": [string] }`. `device_id` is empty unless the code belongs to a token already bound to a device; any device stored from an earlier sign-in is forgotten in that case.

The token is stored as soon as RomM hands it out, even if the call is then cancelled, because RomM accepts each code only once. Grout then checks the token; only a token RomM rejects is removed again.

Errors: `not_configured`, `bad_request` (empty code, or the server changed during the call), `invalid_code`, `rate_limited`, `unauthorized` (RomM issued a token that it then rejected), `server_unreachable`, `cancelled`, `storage`. After `server_unreachable`, `rate_limited` or `cancelled` from the check, the token stays stored.

### `auth.logout`

Arguments: none. Forgets the token, username and device on this device. The server address and the device identifier are kept, so pairing again reuses the same RomM device.

Result: Settings.

Errors: `not_configured` (no host set), `storage`.

### `device.register`

Arguments: `{ "name" }`. Registers this device with RomM for save sync, or updates its name and client version if it is already registered. Pairing registers the device already; this is only needed after `auth.exchange_code` returned an empty `device_id`.

Result: `{ "device_id", "name" }`.

Errors: `not_configured` (no host or no token), `bad_request` (empty name, or the server changed during the call), `unauthorized`, `server_unreachable`, `rate_limited`, `cancelled`, `storage`.

### `settings.update`

Arguments: any of `{ "show_regular_collections", "show_smart_collections", "show_virtual_collections", "downloaded_games", "api_timeout", "download_timeout", "log_level", "platform_order" }`. A field that is absent stays as it is. A field set to `null` is `bad_request`.

Limits: `api_timeout` 1 to 300 seconds; `download_timeout` 1 second to 24 hours; `platform_order` has no duplicates; `downloaded_games` and `log_level` take the values listed under Value sets. A `log_level` change applies to the log file immediately.

Result: Settings.

Errors: `bad_request`, `storage`.
