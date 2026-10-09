# Soundboard

A local soundboard for Windows, shipped as a single `soundboard.exe` (Go backend, React/TypeScript frontend).

- **Main output** = microphone + effects → shows up as a virtual microphone in Discord, Teams, OBS …
- **Monitor** = effects only, on your headphones
- **Boards** in the browser (phones/tablets on the same network too); the admin UI is only reachable from the PC itself

The UI is in German; code, logs and documentation are in English.

## Prerequisite: a virtual cable (one-time)

Windows has no built-in virtual audio device, and writing our own driver would mean a signed kernel driver.
So install [VB-Audio Virtual Cable](https://vb-audio.com/Cable/) once (free, needs a reboot). Then:

1. **Main output** = `CABLE Input`, **input** = your microphone, **monitor** = your headphones.
2. In Discord & co. pick `CABLE Output` as the microphone - or click "Als Standard-Kommunikationsgerät setzen"
   (set as default communications device) under *Windows-Integration* in the admin UI.

Without the cable everything still works, but the mixed output then goes to a real device.

## Usage

Start `dist\soundboard.exe` (tray icon; a click opens the admin UI). On first start the admin UI opens and an
example beep ("Testton") is created.

| Address | Who | What |
|---|---|---|
| `http://localhost:8723/admin` | this PC only | Devices, boards (as tabs), sounds, icons |
| `http://<IP>:8723/boards` | everyone on the network | Overview of all boards (QR code in the admin UI; `/` redirects here) |
| `http://<IP>:8723/boards/<path>` | everyone on the network | One board with its tiles |

Create boards with "+ Board" in the tab strip of the admin UI. The path (`<path>`) is generated from the name
(`Zum Lachen` → `/boards/zum-lachen`) and can be changed per board; the order of the tabs is also the order in the
overview. An older `config.json` with a single board is migrated on first start (the original is kept as
`config.json.v1.bak`).

Data lives in `%APPDATA%\Soundboard` (`config.json`, `sounds\`, `icons\`, `soundboard.log`).
If a folder named `data` sits next to the exe, it is used instead (portable mode).
You can also copy sounds straight into `sounds\` - the folder is watched.

Command-line options: `-data <folder>`, `-port <n>`, `-no-browser`, `-version`.
`soundboard-console.exe` is the same app with a console window (logs visible).

## Building

Needs Node.js and Docker - **no Go and no C compiler on the host**. The audio library (miniaudio) needs cgo, so the
exe is cross-compiled for Windows inside a container (`build/Dockerfile`: Go + MinGW-w64).

```powershell
scripts\build.ps1             # frontend + exe → dist\soundboard.exe, dist\soundboard-console.exe
scripts\build-backend.ps1     # backend only (uses the existing frontend build)
```

Development with live reload: start the backend (`dist\soundboard-console.exe -no-browser`), then
`cd frontend; npm run dev` → <http://localhost:5173/admin> (proxies to port 8723).

Tests (plain Go, no cgo; run on the host or in the container):

```powershell
cd backend
go test ./internal/audio ./internal/library ./internal/store ./internal/server
```

`dist\spike.exe` is a diagnostic tool: without arguments it lists the audio devices; with
`spike run <mic> <output> [monitor]` it runs a long-duration test with statistics (arguments are substrings of the
device names).

## Versions and contributing

The version is derived automatically from git tags and [Conventional Commits](https://www.conventionalcommits.org)
(Semantic Versioning); `soundboard.exe -version` and the admin UI show it. Branching follows git-flow
(`master` = releases, `develop` = integration, `feature/*` for changes). Run `scripts\setup-git.ps1` once after
cloning. Commit format, branching, versioning and the release process are described in
[CONTRIBUTING.md](CONTRIBUTING.md); releases are listed in [CHANGELOG.md](CHANGELOG.md).

## Layout

```
backend/cmd/soundboard      main, tray, single instance (Windows)
backend/cmd/spike           diagnostics / long-duration test
backend/internal/audio      engine, buses, voices, ring buffer, drift compensation (plain Go, tested)
backend/internal/audio/miniaudio   WASAPI binding via malgo (cgo)
backend/internal/winaudio   set the default device (IPolicyConfig)
backend/internal/library    sound folder: decode (wav/mp3/flac/ogg), watch, upload
backend/internal/store      config.json, validation
backend/internal/server     REST + WebSocket, admin protection, static files
backend/web                 embedded frontend build (go:embed)
frontend/                   Vite + React + TypeScript (board and admin views)
```

Audio path: microphone → ring buffer → (drift compensation) → main bus (+ effects) → limiter → `CABLE Input`;
effects → monitor bus → limiter → headphones. Every device has its own clock, so each bus keeps its own voice
cursors; only the microphone path (capture clock ≠ main clock) needs the drift compensation.

## Security

The boards are deliberately not password-protected. The admin UI is only reachable from `localhost` (loopback client,
`Host` header check against DNS rebinding, a custom header against CSRF from foreign web pages). On first start the
Windows firewall asks for permission - allow "private networks".

## License

[MIT](LICENSE). The libraries used keep their own licenses (all permissive: malgo/miniaudio, beep, fsnotify,
coder/websocket, systray, go-ole, go-qrcode, React, lucide-react, nosleep.js). The VB-Audio Virtual Cable is a separate
product with its own license and is not part of this repository.

Open points and decisions taken: [OPEN_QUESTIONS.md](OPEN_QUESTIONS.md). Original plan: [PLAN.md](PLAN.md).
