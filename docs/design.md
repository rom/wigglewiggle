# wigglewiggle — design

A tiny Windows system-tray utility that keeps the screen from locking. It
runs entirely in the current user session and needs **no administrator or
SYSTEM privileges**.

## Goals

- Prevent the screen from locking / the machine from sleeping while running.
- Offer two mechanisms the user can switch between at runtime.
- Live quietly in the system tray; no console window.
- Require no elevation, no installer, and no external runtime — a single
  `.exe`.

## Non-goals

- Cross-platform support (Windows only by design).
- A full settings window or persisted configuration file. State is chosen
  from the tray each session; only the *start-at-login* choice persists (in
  the registry).

## Decisions (from the design interview)

| Area | Decision |
| --- | --- |
| Platform | Windows only |
| Language | Go, built as a windowless GUI binary |
| Privileges | Normal user only — never requires admin/SYSTEM |
| UI | Background app with a system-tray icon + menu |
| Mechanisms | Both, switchable from the tray |
| Default mechanism | OS keep-awake (`SetThreadExecutionState`) |
| Input-simulation | Alternates an F15 keypress and a 1px mouse wiggle |
| Idle-aware | Yes — input simulation only fires after real idleness |
| Interval | 60s default; 30s / 1m / 2m / 5m from the tray |
| Auto-start | Optional, via the per-user `HKCU\…\Run` key |

## Why no elevation is needed

Every OS interaction is a per-process or per-user operation:

- **`SetThreadExecutionState`** sets an execution-state assertion for the
  *calling thread* — no special rights.
- **`SendInput`** / **`GetLastInputInfo`** operate within the interactive
  user's desktop session — no special rights.
- **Auto-start** writes to `HKEY_CURRENT_USER\Software\Microsoft\Windows\`
  `CurrentVersion\Run`, which the user owns. (The machine-wide `HKLM`
  equivalent is the one that needs admin; we deliberately avoid it.)

## Mechanisms

### OS keep-awake (default)

Calls `SetThreadExecutionState(ES_CONTINUOUS | ES_SYSTEM_REQUIRED |
ES_DISPLAY_REQUIRED)`. This tells Windows the system and display are in use,
which resets the idle timers that would otherwise sleep the machine, blank
the display, start the screensaver, or trigger the lock screen. Releasing it
(`ES_CONTINUOUS` alone) restores normal behaviour. This is the same approach
used by tools like PowerToys Awake.

The assertion never moves the cursor or sends keys, so it is invisible — but
it keeps the machine awake even while you are away, which is its intended
behaviour.

### Input simulation

When you are *idle* for the configured interval, the app injects a tiny bit
of synthetic input via `SendInput`, alternating between:

1. an **F15 keypress** — F15 is a real virtual key (`0x7E`) that essentially
   no application acts on, making it the standard "fake activity" signal; and
2. a **1px mouse wiggle** — move right one pixel and immediately back, so the
   cursor ends where it started.

Alternating covers software that watches only the keyboard or only the
mouse. Because injected input also resets the system idle timer, each nudge
naturally spaces the next one a full interval later. This mode also keeps
chat-app presence (Teams/Slack) showing as active.

### Idle-awareness

`GetLastInputInfo` reports the time of the last input event (real *or*
synthetic). The engine compares `GetTickCount() - dwTime` against the
interval and only nudges once that threshold is crossed. Result: while you
are actually using the machine, the simulator stays completely silent and
never fights your real mouse or keyboard.

Idle-awareness is specific to input-simulation mode. OS keep-awake holds a
continuous assertion and is not gated on idleness by design.

## Architecture

```
main.go                         tray UI: menu, wiring, lifecycle (//go:build windows)
internal/keepawake/engine.go    state machine + worker goroutine
internal/win/win_windows.go     thin Win32 wrappers (no cgo)
internal/autostart/autostart_windows.go   HKCU Run registry entry
internal/ui/icon.go + icon.ico  embedded tray icon
tools/genicon/                  dev tool that renders icon.ico
```

The whole application is Windows-only, so the Go files carry a
`//go:build windows` constraint and are cross-compiled with `GOOS=windows`.
The icon asset (`internal/ui`) and the generator (`tools/genicon`) are
platform-neutral.

### Threading model

`SetThreadExecutionState` assertions are tied to the **calling OS thread**
and only persist while that thread is alive. Goroutines normally migrate
across OS threads, which would drop or leak the assertion. The engine
therefore runs a single worker goroutine that calls `runtime.LockOSThread()`
for its entire lifetime and performs *all* Win32 calls (the assertion, idle
checks, and input injection) on that one pinned thread.

The worker loop:

1. applies the current state immediately on any change (mechanism, interval,
   pause), signalled over a buffered channel; and
2. wakes every `pollEvery` (5s) to re-assert the keep-awake state and, in
   input-simulation mode, to check idle time and nudge if due.

The tray menu mutates shared state through mutex-guarded setters; the worker
reads a consistent snapshot under the same mutex. On exit the worker releases
the assertion before unlocking its thread.

## Tray menu

```
<status line>            e.g. "Active — OS keep-awake"   (disabled)
────────────
Mode ▸  ● OS keep-awake
        ○ Input simulation
Interval ▸  ○ 30 seconds
            ● 1 minute
            ○ 2 minutes
            ○ 5 minutes
────────────
Pause / Resume
☑ Start at login
────────────
Quit
```

The tray library is `fyne.io/systray`, whose Windows backend is pure Go
(`golang.org/x/sys/windows`), so the binary builds with `CGO_ENABLED=0` and
needs no C toolchain. The status line and tooltip reflect the live state.

## Auto-start

Toggling *Start at login* writes or deletes a single `HKCU\…\Run` value named
`wigglewiggle`, set to the quoted path of the current executable
(`os.Executable()`). Quoting keeps paths with spaces (e.g. under
`Program Files`) valid.

## Icon

`tools/genicon` draws a friendly teal "awake" face at 16/32/48/64px and
encodes a multi-resolution 32-bit (BGRA) `.ico` with a 1-bpp AND mask derived
from the alpha channel. The result is embedded via `go:embed`. Regenerate
with `go run ./tools/genicon`.

## Build & run

```sh
# Windowless single binary (no console window):
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
  go build -ldflags "-H=windowsgui -s -w" -o wigglewiggle.exe .
```

Run `wigglewiggle.exe`; it appears in the system tray. Right-click for the
menu; choose **Quit** to exit. `arm64` builds the same way.

## Testing

- `internal/ui` has a host-runnable test validating the embedded `.ico`
  header and directory entries (`go test ./internal/ui/`).
- `internal/win` has a Windows-only test guarding the `INPUT` struct size
  that `SendInput` depends on (verified via `GOOS=windows go vet ./...`, and
  executed under Windows CI).

The Win32 behaviour itself cannot be exercised on a non-Windows host; it is
validated by cross-compilation and `go vet`, and is intended to be smoke-
tested on Windows.

## Limitations / possible future work

- 64-bit only in practice (the `INPUT` layout assumes 8-byte pointers; modern
  Windows targets are amd64/arm64).
- No persisted preferences beyond start-at-login; mechanism/interval reset to
  defaults each launch. A small config file could be added if desired.
- No tray balloon/notifications; state is conveyed via the tooltip and the
  disabled status line.
