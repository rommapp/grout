# Grout as a library

Grout is a handheld UI, a CLI, and a library that other frontends embed to get RomM support with Grout's rules: library sync, browsing and search, and, in later releases, downloads, identification and save sync. Grout's own UI and CLI are built on the same library.

There are two ways in:

- **Go programs** import `github.com/rommapp/grout` and call `grout.Open`, which returns a typed handle.
- **Everything else** (C, C++, Rust, Kotlin through JNI) loads `libgrout.so` and calls seven C functions. Arguments and results are JSON, described in the [contract reference](contract.md).

## Getting the library

Each Grout release attaches one archive per target. Each holds `libgrout.so` and the generated `libgrout.h`:

| Asset | Target |
|---|---|
| `libgrout-linux-arm64.tar.gz` | Linux aarch64, glibc 2.31 or newer |
| `libgrout-linux-arm.tar.gz` | Linux armv7 hard float, glibc 2.31 or newer |
| `libgrout-android-arm64-v8a.tar.gz` | Android arm64-v8a, API 28 or newer, 16 KB page aligned |
| `libgrout-android-armeabi-v7a.tar.gz` | Android armeabi-v7a, API 28 or newer |

Pin one release and upgrade on purpose. The JSON contract carries a protocol number, which `grout_version` reports. Adding methods, optional arguments or result fields does not change it. Removing anything or changing its meaning does.

## Lifecycle

```c
char*     grout_version(void);
uintptr_t grout_open(const char* options_json, char** error_out);
char*     grout_call(uintptr_t handle, int64_t call_id, const char* method, const char* args_json);
void      grout_cancel(uintptr_t handle, int64_t call_id);
void      grout_set_event_callback(uintptr_t handle, grout_event_fn fn, void* userdata);
void      grout_free(char* s);
void      grout_close(uintptr_t handle);
```

1. `grout_open` takes the options and returns a handle, or 0 on failure. On failure, when `error_out` is not NULL, it points at an error envelope.
2. `grout_set_event_callback` registers a function for progress events. Optional.
3. `grout_call` runs one method and returns its envelope. Pick a `call_id` that is unique among the calls running on that handle; events and `grout_cancel` refer to it. `args_json` may be NULL.
4. `grout_cancel` stops a running call. The call returns `cancelled`. A cancel that arrives before Grout has registered the call's `call_id` is ignored. When cancelling from another thread right after starting a call, make sure that call has begun, or cancel again.
5. `grout_close` cancels every running call, waits for them, and releases the data directory.
6. Every `char*` Grout returns, including `*error_out`, belongs to the host and must be released with `grout_free`.

The options for `grout_open`:

```json
{
  "data_dir": "/data/user/0/dev.example/files/grout",
  "firmware": { "definition": "<the text of your firmware.toml>" },
  "env": { "CANNOLI_ROOT": "/storage/emulated/0/Cannoli" },
  "client": { "name": "cannoli", "version": "2.3.0" },
  "art_max_size": { "width": 320, "height": 240 }
}
```

`art_max_size` is the largest size, in pixels, Grout scales downloaded art to, keeping its shape. Leave it out to keep art at full size. Grout's own UI passes half its screen size.

`client` names your app to RomM: it is recorded on the device, so the user's device list shows the app they actually run. It defaults to Grout's own name and version.

`firmware` takes either `"id"`, naming one of Grout's built-in firmwares such as `"muos"`, or `"definition"`, holding a complete definition. `env` provides the values the definition reads. Grout never reads the process environment. The name `GROUT_ROOT` is reserved: when set, Grout treats it as the root of a mock card and resolves every firmware path under it, so hosts should not pass it.

## Threading

- `grout_call` blocks the calling thread until the method finishes. Calls from several threads run at the same time. Downloads run alongside each other, but not alongside a refresh or scan: whichever starts second returns `busy` immediately, as does a second refresh or scan, or a second download of the same game or the same platform's BIOS.
- The event callback can run on any thread. Today it runs on the thread that called `grout_call` for that call, but hosts must not rely on that. On Android the JNI shim attaches the thread when `GetEnv` reports it detached. In Rust the callback and its `userdata` must be safe to use from any thread.
- The event string is only valid during the callback. Copy it if you need it later.
- The callback may call any Grout function, including `grout_cancel` and `grout_close`, without deadlocking.
- Once `grout_set_event_callback`, `grout_cancel` (for that call) or `grout_close` returns, Grout no longer runs the old callback on any other thread and never calls it again. The host may then free its `userdata`. There is one exception: if two threads each call `grout_close` or `grout_cancel` from inside their own callback and end up waiting for each other, Grout breaks the cycle instead of deadlocking, and one of them can return while the other's callback is still running. Avoid closing from inside a callback on more than one thread at a time.

## Process rules

- One handle per data directory. A second `grout_open` on the same directory, from the same process or another one, fails with `data_dir_locked` until the first handle is closed. A `grout_close` made from inside an event callback releases the directory only when that callback's call returns.
- Load one Go shared library per process. The library cannot be unloaded.
- Grout's own code never exits the process, never installs signal handlers, and never writes to stdout or stderr. It logs to `logs/grout.log` in the data directory.
- The Go runtime inside `libgrout.so` does install signal handlers (for example SIGSEGV, SIGBUS and SIGPIPE) when the library loads. It chains to any handler already installed, so load `libgrout.so` after the host's own crash reporter or runtime has set up its handlers. A handler a host installs after loading Grout must use `SA_ONSTACK` and chain to the previous handler, or a fault inside Grout becomes a crash instead of an `internal` error. On Android, load it from Java or Kotlin with `System.loadLibrary` after the app has started, as the example below does.

## Signing in

Set the server first with `settings.set_host` (`root_uri`, and `port` or `insecure_skip_verify` if needed). Calling it again with the same server keeps the user signed in; pointing it at a different server signs them out. Then sign in one of two ways:

- **Pairing on the device (recommended).** Call `auth.pair_start` with a name for the device, then show the returned `user_code` and a QR code of `verification_url`. Call `auth.pair_wait`, which returns when the user approves or denies the device in RomM's web UI or the code expires. If the user backs out, cancel the `auth.pair_wait` call with `grout_cancel`. Pairing also registers the device for save sync.
- **A pairing code from RomM.** The user makes a code in RomM's web UI and types it on the device; call `auth.exchange_code` with it. Codes expire after a minute. If the result has an empty `device_id`, call `device.register` before using save sync.

`auth.logout` forgets the token on this device. `settings.get` shows who is signed in (`username`), whether a token is stored and when it expires.

## Downloading

Grout downloads one item per call, and the host owns the queue: call `fetch.game` once per game, from as many threads as you want downloads running at once, and cancel any of them with `grout_cancel`. Keep your own list, ordering and notification; Grout keeps no queue.

- `fetch.game` downloads a game, unpacks it if needed, places it where the firmware expects it, fetches its art and records it as downloaded, so `library.games` reports it at once without a scan. Show progress from the `download` events' `bytes_done` and `bytes_total`.
- For a game with several versions, list them with `library.game_files` and pass the chosen `file_id`.
- `fetch.art` fills in missing art for a game already on the card.
- `library.bios` shows which of the BIOS files the user uploaded to RomM are on the card; `fetch.bios` downloads them all for a platform.
- A refresh or scan cannot run while any download is running, and the reverse. Wait for downloads to finish, or cancel them, before refreshing.
- A failed or cancelled download leaves nothing half-written. A download cancelled after the game reached the card keeps it and skips the art.

## Choosing the data directory

Grout keeps its settings (including the RomM token), its database and its log in the data directory. Give it a folder that only your app can read. On Android that is `context.filesDir`, which every Android version sandboxes per app.

## Writing a firmware definition

A host describes its device with a definition in the same format as Grout's built-in firmwares in `core/firmware/<name>/firmware.toml`. Start from the built-in definition closest to your device and copy it whole. Host definitions do not extend built-ins.

- Paths the host only knows at runtime use `{env.NAME}` variables, for example `card = { value = "{env.CANNOLI_ROOT}" }`. The value comes from `env` in the open options. A variable missing from `env`, or set to an empty string, fails the open with `bad_definition`, naming the variable.
- Host definitions are checked with the host profile: the same rules as built-in definitions, except for Grout's own packaging fields and its input mapping files.
- Hooks are named in the definition. A C host can use the hooks Grout registers, but cannot add its own.
- Check your definition in your own CI against the Grout release you pin:

```sh
grout firmware validate path/to/firmware.toml --host
```

The definition format has no version field. The parser rejects unknown keys, so a definition that no longer fits a newer Grout fails loudly at open instead of meaning something else. Every Grout release notes each change to the definition format in its release notes.

## Rust

```rust
use std::ffi::{c_char, c_void, CStr, CString};

#[link(name = "grout")]
extern "C" {
    fn grout_open(options_json: *const c_char, error_out: *mut *mut c_char) -> usize;
    fn grout_call(handle: usize, call_id: i64, method: *const c_char, args_json: *const c_char) -> *mut c_char;
    fn grout_cancel(handle: usize, call_id: i64);
    fn grout_set_event_callback(handle: usize, f: Option<extern "C" fn(*mut c_void, *const c_char)>, userdata: *mut c_void);
    fn grout_free(s: *mut c_char);
    fn grout_close(handle: usize);
}

fn take(p: *mut c_char) -> String {
    let s = unsafe { CStr::from_ptr(p) }.to_string_lossy().into_owned();
    unsafe { grout_free(p) };
    s
}

pub fn call(handle: usize, id: i64, method: &str, args: &str) -> String {
    let m = CString::new(method).unwrap();
    let a = CString::new(args).unwrap();
    take(unsafe { grout_call(handle, id, m.as_ptr(), a.as_ptr()) })
}
```

## Android

A small C file in the app's NDK build bridges JNI to `libgrout.so`. Pass strings in both directions as UTF-8 bytes, never through `GetStringUTFChars` or `NewStringUTF`. Those use Java's modified UTF-8, which corrupts characters outside the Basic Multilingual Plane, such as emoji in a search or a game name.

```c
#include <jni.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include "libgrout.h"

static char* text(JNIEnv* env, jbyteArray in) {
    jsize n = (*env)->GetArrayLength(env, in);
    char* out = malloc((size_t)n + 1);
    (*env)->GetByteArrayRegion(env, in, 0, n, (jbyte*)out);
    out[n] = 0;
    return out;
}

static jbyteArray bytes(JNIEnv* env, char* s) {
    jsize n = (jsize)strlen(s);
    jbyteArray out = (*env)->NewByteArray(env, n);
    (*env)->SetByteArrayRegion(env, out, 0, n, (const jbyte*)s);
    grout_free(s);
    return out;
}

JNIEXPORT jbyteArray JNICALL Java_dev_example_Grout_call(JNIEnv* env, jclass cls, jlong h, jlong id, jbyteArray method, jbyteArray args) {
    char* m = text(env, method);
    char* a = text(env, args);
    char* out = grout_call((uintptr_t)h, id, m, a);
    free(m);
    free(a);
    return bytes(env, out);
}
```

```kotlin
object Grout {
    init {
        System.loadLibrary("grout")
        System.loadLibrary("grout_jni")
    }

    @JvmStatic external fun call(handle: Long, callId: Long, method: ByteArray, args: ByteArray): ByteArray

    fun callJson(handle: Long, callId: Long, method: String, args: String = "{}"): String =
        String(
            call(handle, callId, method.toByteArray(Charsets.UTF_8), args.toByteArray(Charsets.UTF_8)),
            Charsets.UTF_8,
        )
}
```

The event callback works the same way: keep the `JavaVM` from `JNI_OnLoad`, call `AttachCurrentThread` when `GetEnv` reports the thread is detached, and pass a global reference to your listener as `userdata`.

## Errors

Every failure is an envelope with one of these codes:

| Code | Meaning |
|---|---|
| `not_configured` | No RomM host is set. |
| `bad_definition` | The firmware id is unknown, or the host definition failed validation. |
| `server_unreachable` | RomM could not be reached, or answered with something that is not RomM. |
| `unauthorized` | RomM rejected the credentials. |
| `server_too_old` | The RomM server does not support save channel snapshots. |
| `busy` | A refresh or scan, or an `auth.pair_wait`, is already running on this handle. |
| `data_dir_locked` | Another handle or process has this data directory open. |
| `bad_request` | Unknown method, bad arguments, an unknown or closed handle, or a duplicate in-flight `call_id`. |
| `cancelled` | The call was cancelled. |
| `storage` | The local database or a local file failed. |
| `internal` | A bug in Grout. The handle stays usable. |
| `invalid_code` | The pairing code is unknown, already used or expired. |
| `rate_limited` | RomM is limiting requests from this device; wait a minute. |

The [contract reference](contract.md) lists which codes each method returns. A host should handle `busy` (try later), `rate_limited` (wait a minute), `invalid_code` (ask for a new code), `cancelled`, `unauthorized` (ask the user to log in again), `server_too_old`, `not_configured` (set a host first) and `data_dir_locked`. Everything else can be shown to the user as-is, using `message`.
