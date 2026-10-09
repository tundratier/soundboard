# Soundboard - Development plan

Private project (possibly open source, never commercial). Goal: a Windows soundboard as a single `.exe`.

## Requirements

- Windows 10/11 as the target
- Freely selectable microphone input
- **Main output** = microphone + effects → shows up in Windows as a recording device that can be chosen as the
  *default communications device* (Discord, Teams, …)
- **Monitor output** = effects only, on a freely selectable output device (headphones); Windows mixes it with
  everything else playing there
- Sounds are stored locally in a folder
- Boards with a variable x/y tile grid (one board in v1, several since 2026-10-08); per tile: sound, icon, balance
  (+ gain, mode)
- Local admin UI for channels and boards
- A local web server serves the board view; anyone on the network may trigger sounds (no PIN)
- Frontend: TypeScript/React. Backend: Go, single exe, nothing to install at runtime

## Feasibility - result

Feasible. One prerequisite remains:

**Virtual audio device.** For the mix to show up as a microphone in Windows, an audio driver is needed. Writing our
own would require kernel signing → not sensible.
→ Install **VB-Audio Virtual Cable** (donationware) once. The app writes to `CABLE Input`, Windows sees
`CABLE Output` as a microphone.
- Bundling it in the exe is only permitted by agreement under the VB-Audio license → the app detects the cable and,
  if it is missing, shows a hint + download link.
- Alternative that was checked: `VirtualDrivers/Virtual-Audio-Driver` (MIT) is beta, needs test signing and has no
  loopback → unsuitable for now.
- The app can set `CABLE Output` as the default communications device at the push of a button (via the
  undocumented but stable `IPolicyConfig` COM interface, which EarTrumpet/SoundSwitch use as well).

## Technology

| Area | Choice |
|---|---|
| Audio I/O | `github.com/gen2brain/malgo` (miniaudio, WASAPI) - needs cgo |
| Decoding/resampling | `github.com/gopxl/beep/v2` (wav, mp3, flac, vorbis) - decoders only, no output |
| HTTP/WS | `net/http` + `github.com/coder/websocket` |
| Folder watching | `github.com/fsnotify/fsnotify` |
| Tray | `fyne.io/systray` |
| Windows COM (set default device) | `github.com/go-ole/go-ole` |
| QR code | `github.com/skip2/go-qrcode` |
| TS types from Go | `github.com/gzuidhof/tygo` (planned; types are mirrored by hand instead) |
| Exe icon/manifest | `github.com/tc-hib/go-winres` |
| Frontend | Vite, React, TypeScript, TanStack Query, dnd-kit, lucide-react (planned; implemented with React, lucide-react and nosleep.js only) |

**Build:** cgo cross-compile in a Docker container (`golang` + `gcc-mingw-w64`) → no C compiler needed on the host.
Consequence: the cgo package `internal/audio/miniaudio` is only built in the container; mixer, store, server etc. are
plain Go and testable locally with `go test`.

## Architecture

```
┌──────────────────────── soundboard.exe ───────────────────────────┐
│ Mic (capture) ─► SPSC ring buffer ─► drift compensation ─┐        │
│                                                          ▼        │
│ Trigger ─► Voice ───────────────────────────────► "Main" bus ─► limiter ─► CABLE Input
│        └─► Voice ───────────────────────────────► "Monitor" bus ─► limiter ─► headphones
│                                                                   │
│ Library: sounds/ → decoded → float32 stereo 48 kHz in RAM         │
│ Store:   config.json                                              │
│ HTTP:    /admin (127.0.0.1 only), /boards, REST, WebSocket        │
│ Tray:    open admin · board URL/QR · quit                         │
└───────────────────────────────────────────────────────────────────┘
```

- Every output device has its own clock. That is why each bus keeps its own voice cursors → sounds never drift.
  Only the mic path (capture clock ≠ main clock) needs drift compensation.
- Internally uniform: 48 kHz, float32, stereo. miniaudio converts device formats.
- Audio callbacks are allocation-free; control commands (trigger/stop) reach the callback through a lock-free queue.
- Balance: planned as constant-power panning, implemented as a true balance control (see OPEN_QUESTIONS.md). An output
  limiter guards against clipping when several sounds play at once.
- Feedback protection: the input must never be `CABLE Output`.
- Windows ducking: while a communications device is active, Windows lowers other audio streams (including the
  monitor). Opt out per session via `SetDuckingPreference`, otherwise point to
  mmsys.cpl → Communications → "Do nothing".

## Data model (v2)

```ts
type Config = {
  version: 2;                         // 1: single "board"
  audio: {
    inputDeviceId: string;            // physical microphone
    mainDeviceId: string;             // e.g. CABLE Input
    monitorDeviceId?: string;         // headphones
    micGain: number; fxGainMain: number; fxGainMonitor: number;
    periodMs: number;                 // default 10
  };
  server: { port: number; bindLan: boolean };
  boards: Board[];
};
type Board = { id: string; name: string; cols: number; rows: number; tiles: Tile[] };  // id = URL path
type Tile = {
  id: string; x: number; y: number;   // tile IDs are unique across all boards
  label?: string; sound: string;      // file name relative to sounds/
  icon?: { kind: "file" | "emoji"; value: string };
  pan: number;                        // -1 … +1
  gain: number;                       // 0 … 2
  mode: "oneshot" | "restart" | "loop" | "toggle";
  color?: string;
};
```

Data lives in `%APPDATA%\Soundboard\` (`config.json`, `sounds/`, `icons/`); portable mode if a folder `data/` sits
next to the exe.

## Project structure

```
soundboard/
├─ backend/
│  ├─ cmd/soundboard/       main, tray, single instance
│  ├─ cmd/spike/            phase 0 prototype / diagnostics
│  ├─ internal/audio/       engine, buses, voices, ring buffer, drift (plain Go)
│  ├─ internal/audio/miniaudio  malgo binding (cgo)
│  ├─ internal/winaudio/    IPolicyConfig (Windows COM)
│  ├─ internal/library/     sound folder, decoding
│  ├─ internal/store/       config.json
│  ├─ internal/server/      REST, WS
│  └─ web/dist/             frontend build (go:embed)
├─ frontend/                Vite + React + TS (admin/, board/, shared/)
├─ build/Dockerfile         cross-compile image
└─ scripts/                 build.ps1, build-backend.ps1
```

## Phases

> **Status (as of 2026-10-09):** Phases 0-5 are implemented, see [README.md](README.md). Added later: multiple boards
> (`/boards`, tabs in the admin UI) and keeping the phone screen awake. What stayed untested (listening/latency with a
> real virtual cable, soak test, setting the default device) and the decisions I took are in
> [OPEN_QUESTIONS.md](OPEN_QUESTIONS.md). Deviations from the plan: true balance instead of constant-power panning, no
> tygo/dnd-kit/TanStack Query, the microphone buffer's target fill level is 2 periods.

### Phase 0 - Spike (2-3 days) ✅ (hardware measurements still open)
- [x] Docker build of a Windows exe with malgo
- [x] List devices (capture/playback, default flag)
- [x] Mic → ring buffer → CABLE Input; mix a sound into main + monitor (`spike run …`)
- [x] Statistics: ring buffer fill level, underruns/overruns, drift corrections
- [ ] Manual test: latency (< 30-40 ms), crackles, 1 h soak run, CPU
- [x] Decision: approach confirmed (Go + miniaudio)

### Phase 1 - Audio engine (~1 week)
- Engine API: device settings, `Trigger(tileID)`, `Stop`, `StopAll`, levels, events (playing/stopped)
- Playback modes, gain/pan, limiter, drift control (PI-controlled resampling)
- Device hot-plug, reconnect, default-device changes
- Unit tests (mixer, pan, ring buffer, drift)

### Phase 2 - Library & store (2-3 days)
- Read/watch the sound folder, decode, resample, error handling
- `config.json` with a version and atomic writes

### Phase 3 - Server/API (3-4 days)
- REST: devices, config, boards, tiles, upload of sounds/icons
- WebSocket: `trigger`, `stop`, `stopAll`, status/level events
- Admin only via localhost; boards open on the LAN
- tygo types for the frontend (dropped, see above)

### Phase 4 - Frontend (~1-1.5 weeks)
- Admin: devices (incl. CABLE detection, "set as default communications device"), levels, grid size, tile editor
  (drag & drop, icon, pan, gain, mode), audition on the monitor, upload
- Board: responsive touch grid, live status, fullscreen, PWA

### Phase 5 - Packaging (2-3 days)
- go:embed, `-H windowsgui`, icon/manifest, tray, single instance, browser start, autostart (optional)
- Ducking opt-out, IPolicyConfig

### Phase 6 - Later
- Global hotkeys, mic ducking while effects play, fades, ~~multiple boards~~ (done), import/export, Wails window

## Risks

| Risk | Mitigation |
|---|---|
| VB-Cable is missing | Detection + instructions in the admin UI |
| Drift mic ↔ main | Ring buffer + drift control, soak test in phase 0 |
| Windows ducking lowers the monitor | Ducking opt-out / hint |
| Microphone privacy settings block desktop apps | Detect the error, point to the settings |
| Firewall blocks the LAN | Hint, configurable port |
| cgo build | Docker image, reproducible |
