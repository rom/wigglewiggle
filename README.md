# wigglewiggle

A tiny Windows system-tray utility that keeps the screen from locking — and
runs entirely as a normal user, with **no administrator/SYSTEM privileges**.

It lives in the tray and offers two switchable ways to stay awake:

- **OS keep-awake** (default) — holds a native Windows "stay awake"
  assertion. Never touches your mouse or keyboard.
- **Input simulation** — when you've been idle for the chosen interval, it
  alternates a harmless F15 keypress and a 1px mouse wiggle. Also keeps
  Teams/Slack presence active. It is *idle-aware*, so it never fights your
  real input.

## Tray menu

- **Mode** — OS keep-awake · Input simulation
- **Interval** — 30s · 1m · 2m · 5m (used by input simulation)
- **Pause / Resume**
- **Start at login** — adds/removes a per-user `HKCU\…\Run` entry
- **Quit**

## Build

Requires Go 1.23+. Produces a single windowless `.exe` (no console window):

```sh
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
  go build -ldflags "-H=windowsgui -s -w" -o wigglewiggle.exe .
```

`GOARCH=arm64` works the same way. The build is pure Go (`CGO_ENABLED=0`), so
no C toolchain is needed.

## Run

Launch `wigglewiggle.exe` — it appears in the system tray. Right-click for the
menu; choose **Quit** to exit. Use **Start at login** if you want it to come
back automatically when you sign in.

## Develop

- Tests: `go test ./internal/ui/` (host) and `GOOS=windows go vet ./...`.
- Regenerate the tray icon: `go run ./tools/genicon`.

See [docs/design.md](docs/design.md) for the full design, or
[docs/dokumentation.md](docs/dokumentation.md) for documentation in Swedish.
