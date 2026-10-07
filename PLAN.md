# Soundboard – Entwicklungsplan

Privates Projekt (ggf. Open Source, nie kommerziell). Ziel: Windows-Soundboard als einzelne `.exe`.

## Anforderungen

- Windows 10/11 als Ziel
- Mikrofon-Eingang frei wählbar
- **Hauptausgang** = Mikrofon + Effekte → erscheint in Windows als Aufnahmegerät, das als
  *Standardkommunikationsgerät* gewählt werden kann (Discord, Teams, …)
- **Monitor-Ausgang** = nur Effekte auf ein frei wählbares Ausgabegerät (Kopfhörer); Windows mischt
  das mit allem anderen, was dort läuft
- Sounds liegen lokal in einem Ordner
- Ein Board (v1) mit variablem x/y-Kachel-Grid; pro Kachel: Sound, Icon, Balance (+ Gain, Modus)
- Lokales Admin-UI für Kanäle und Board
- Lokaler Webserver liefert die Board-Ansicht; jeder im Netz darf Sounds auslösen (keine PIN)
- Frontend: TypeScript/React. Backend: Go, single exe, keine Installation zur Laufzeit

## Machbarkeit – Ergebnis

Machbar. Eine Voraussetzung bleibt:

**Virtuelles Audiogerät.** Damit der Mix in Windows als Mikrofon auftaucht, braucht es einen
Audio-Treiber. Eigene Treiber erfordern Kernel-Signierung → nicht sinnvoll.
→ **VB-Audio Virtual Cable** (Donationware) einmalig installieren. App schreibt auf `CABLE Input`,
Windows sieht `CABLE Output` als Mikrofon.
- Mitliefern in der exe ist laut VB-Audio-Lizenz nur mit Vereinbarung erlaubt → App erkennt das
  Kabel, und falls es fehlt, zeigt sie einen Hinweis + Download-Link.
- Geprüfte Alternative: `VirtualDrivers/Virtual-Audio-Driver` (MIT) ist Beta, braucht Testsigning
  und hat keinen Loopback → aktuell ungeeignet.
- Die App kann `CABLE Output` per Knopfdruck als Standardkommunikationsgerät setzen
  (undokumentiertes, aber stabiles `IPolicyConfig`-COM-Interface, das auch EarTrumpet/SoundSwitch nutzen).

## Technologie

| Bereich | Wahl |
|---|---|
| Audio-I/O | `github.com/gen2brain/malgo` (miniaudio, WASAPI) – benötigt cgo |
| Dekodierung/Resampling | `github.com/gopxl/beep/v2` (wav, mp3, flac, vorbis) – nur Dekoder, keine Ausgabe |
| HTTP/WS | `net/http` + `github.com/coder/websocket` |
| Ordner-Überwachung | `github.com/fsnotify/fsnotify` |
| Tray | `fyne.io/systray` |
| Windows-COM (Default-Gerät setzen) | `github.com/go-ole/go-ole` |
| QR-Code | `github.com/skip2/go-qrcode` |
| TS-Typen aus Go | `github.com/gzuidhof/tygo` |
| Exe-Icon/Manifest | `github.com/tc-hib/go-winres` |
| Frontend | Vite, React, TypeScript, TanStack Query, dnd-kit, lucide-react |

**Build:** cgo-Cross-Compile im Docker-Container (`golang` + `gcc-mingw-w64`) → kein C-Compiler auf dem
Host nötig. Konsequenz: Paket `internal/audio/device` (cgo) wird nur im Container gebaut; Mixer,
Store, Server usw. sind reines Go und lokal mit `go test` testbar.

## Architektur

```
┌──────────────────────── soundboard.exe ───────────────────────────┐
│ Mic (Capture) ─► SPSC-Ringpuffer ─► Drift-Ausgleich ─┐            │
│                                                      ▼            │
│ Trigger ─► Voice ──────────────────────────────► Bus „Main“ ─► Limiter ─► CABLE Input
│        └─► Voice ──────────────────────────────► Bus „Monitor“ ─► Limiter ─► Kopfhörer
│                                                                   │
│ Library: sounds/ → dekodiert → float32 Stereo 48 kHz im RAM       │
│ Store:   config.json                                              │
│ HTTP:    /admin (nur 127.0.0.1), /board, REST, WebSocket          │
│ Tray:    Admin öffnen · Board-URL/QR · Beenden                    │
└───────────────────────────────────────────────────────────────────┘
```

- Jedes Ausgabegerät hat eine eigene Uhr. Deshalb führt jeder Bus eigene Voice-Cursor → Sounds
  driften nie. Nur der Mic-Pfad (Capture-Uhr ≠ Main-Uhr) braucht Drift-Ausgleich.
- Intern einheitlich: 48 kHz, float32, Stereo. miniaudio konvertiert Gerätformate.
- Audio-Callbacks allokationsfrei; Steuerbefehle (trigger/stop) per lock-freier Queue in den Callback.
- Balance: Constant-Power-Panning. Ausgangs-Limiter gegen Clipping bei mehreren Sounds.
- Feedback-Schutz: Eingang darf nie `CABLE Output` sein.
- Windows-Ducking: Ist ein Kommunikationsgerät aktiv, senkt Windows andere Audiostreams ab (auch den
  Monitor). In der Session per `SetDuckingPreference` abmelden, sonst Hinweis auf
  mmsys.cpl → Kommunikation → „Keine Aktion“.

## Datenmodell (v1)

```ts
type Config = {
  version: 1;
  audio: {
    inputDeviceId: string;            // physisches Mikrofon
    mainDeviceId: string;             // z. B. CABLE Input
    monitorDeviceId?: string;         // Kopfhörer
    micGain: number; fxGainMain: number; fxGainMonitor: number;
    periodMs: number;                 // Standard 10
  };
  server: { port: number; bindLan: boolean };
  board: Board;
};
type Board = { name: string; cols: number; rows: number; tiles: Tile[] };
type Tile = {
  id: string; x: number; y: number;
  label?: string; sound: string;      // Dateiname relativ zu sounds/
  icon?: { kind: "file" | "emoji" | "lucide"; value: string };
  pan: number;                        // -1 … +1
  gain: number;                       // 0 … 2
  mode: "oneshot" | "restart" | "loop" | "toggle";
  color?: string;
};
```

Daten in `%APPDATA%\Soundboard\` (`config.json`, `sounds/`, `icons/`); portabler Modus, wenn
neben der exe ein Ordner `data/` liegt.

## Projektstruktur

```
soundboard/
├─ backend/
│  ├─ cmd/soundboard/       main, Tray, Single-Instance
│  ├─ cmd/spike/            Phase-0-Prototyp
│  ├─ internal/audio/       Engine, Busse, Voices, Ringpuffer, Drift (reines Go)
│  ├─ internal/audio/device malgo-Anbindung (cgo)
│  ├─ internal/winaudio/    IPolicyConfig, Ducking (Windows-COM)
│  ├─ internal/library/     Sound-Ordner, Dekodierung, Cache
│  ├─ internal/store/       config.json
│  ├─ internal/server/      REST, WS, go:embed
│  └─ web/dist/             Frontend-Build
├─ frontend/                Vite + React + TS (admin/, board/, shared/)
├─ build/Dockerfile         Cross-Compile-Image
└─ Taskfile.yml / build.ps1
```

## Phasen

> **Status (Stand 2026-10-06):** Phase 0–5 sind umgesetzt, siehe [README.md](README.md). Was ungetestet blieb
> (Hörtest/Latenz mit echtem virtuellem Kabel, Dauertest, Standardgerät setzen) und welche Entscheidungen ich
> getroffen habe, steht in [OPEN_QUESTIONS.md](OPEN_QUESTIONS.md). Abweichungen vom Plan: echte Balance statt
> Constant-Power-Panning, kein tygo/dnd-kit/TanStack Query, Ziel-Füllstand des Mikrofon-Puffers = 2 Perioden.

### Phase 0 – Spike (2–3 Tage) ✅ (Hardware-Messungen offen)
- [x] Docker-Build für eine Windows-exe mit malgo
- [x] Geräte auflisten (Capture/Playback, Default-Kennzeichnung)
- [x] Mic → Ringpuffer → CABLE Input; Sound in Main + Monitor einmischen (`spike run …`)
- [x] Statistik: Ringpuffer-Füllstand, Unter-/Überläufe, Drift-Korrekturen
- [ ] Manueller Test: Latenz (< 30–40 ms), Knackser, 1 h Dauerlauf, CPU
- [x] Entscheidung: Ansatz bestätigt (Go + miniaudio)

### Phase 1 – Audio-Engine (~1 Woche)
- Engine-API: `SetDevices`, `Trigger(tileID)`, `Stop`, `StopAll`, Pegel, Events (playing/stopped)
- Wiedergabemodi, Gain/Pan, Limiter, Drift-Regelung (PI-geregeltes Resampling)
- Geräte-Hotplug, Reconnect, Default-Wechsel
- Unit-Tests (Mixer, Pan, Ringpuffer, Drift)

### Phase 2 – Library & Store (2–3 Tage)
- Sound-Ordner einlesen/überwachen, dekodieren, resamplen, Fehlerbehandlung
- `config.json` mit Version und atomarem Schreiben

### Phase 3 – Server/API (3–4 Tage)
- REST: Geräte, Config, Board, Kacheln, Upload Sounds/Icons
- WebSocket: `trigger`, `stop`, `stopAll`, Status-/Pegel-Events
- Admin nur über localhost; Board im LAN offen
- tygo-Typen für das Frontend

### Phase 4 – Frontend (~1–1,5 Wochen)
- Admin: Geräte (inkl. CABLE-Erkennung, „als Standardkommunikationsgerät setzen“), Pegel,
  Grid-Größe, Kachel-Editor (Drag & Drop, Icon, Pan, Gain, Modus), Vorhören auf Monitor, Upload
- Board: responsives Touch-Grid, Live-Status, Vollbild, PWA

### Phase 5 – Packaging (2–3 Tage)
- go:embed, `-H windowsgui`, Icon/Manifest, Tray, Single-Instance, Browser-Start, Autostart (optional)
- Ducking-Opt-out, IPolicyConfig

### Phase 6 – Später
- Globale Hotkeys, Mic-Ducking bei Effekten, Fades, mehrere Boards, Import/Export, Wails-Fenster

## Risiken

| Risiko | Gegenmaßnahme |
|---|---|
| VB-Cable fehlt | Erkennung + Anleitung im Admin |
| Drift Mic ↔ Main | Ringpuffer + Drift-Regelung, Dauertest in Phase 0 |
| Windows-Ducking senkt Monitor ab | Ducking-Opt-out / Hinweis |
| Mikrofon-Datenschutz blockiert Desktop-Apps | Fehler erkennen, Hinweis auf Einstellungen |
| Firewall blockiert LAN | Hinweis, Port konfigurierbar |
| cgo-Build | Docker-Image, reproduzierbar |
