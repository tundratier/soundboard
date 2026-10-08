# Soundboard

Lokales Soundboard für Windows als einzelne `soundboard.exe` (Go-Backend, React/TypeScript-Frontend).

- **Hauptausgang** = Mikrofon + Effekte → als virtuelles Mikrofon in Discord, Teams, OBS …
- **Monitor** = nur die Effekte auf deinen Kopfhörern
- **Board** im Browser (auch Handy/Tablet im selben Netz), Verwaltung nur auf dem eigenen Rechner

## Voraussetzung: virtuelles Kabel (einmalig)

Windows hat kein virtuelles Audiogerät eingebaut, und ein eigener Treiber wäre ein signierter Kernel-Treiber.
Installiere deshalb einmalig [VB-Audio Virtual Cable](https://vb-audio.com/Cable/) (kostenlos, Neustart).
Dann:

1. **Hauptausgang** = `CABLE Input`, **Eingang** = dein Mikrofon, **Monitor** = Kopfhörer.
2. In Discord & Co. das Mikrofon `CABLE Output` wählen – oder in der Verwaltung unter *Windows-Integration*
   „Als Standard-Kommunikationsgerät setzen“ klicken.

Ohne Kabel läuft alles, nur der gemischte Ausgang landet dann auf einem echten Gerät.

## Benutzen

`dist\soundboard.exe` starten (Tray-Symbol, Klick öffnet die Verwaltung). Beim ersten Start wird die Verwaltung
geöffnet und ein Testton angelegt.

| Adresse | Wer | Was |
|---|---|---|
| `http://localhost:8723/admin` | nur dieser Rechner | Geräte, Boards (als Tabs), Sounds, Icons |
| `http://<IP>:8723/boards` | alle im Netz | Übersicht aller Boards (QR-Code in der Verwaltung; `/` leitet hierher um) |
| `http://<IP>:8723/boards/<pfad>` | alle im Netz | ein Board mit seinen Kacheln |

Boards legst du in der Verwaltung über das „+ Board“ im Tab-Streifen an. Der Pfad (`<pfad>`) wird aus dem Namen
erzeugt (`Zum Lachen` → `/boards/zum-lachen`) und lässt sich pro Board ändern; die Reihenfolge der Tabs ist auch
die Reihenfolge in der Übersicht. Eine ältere `config.json` mit nur einem Board wird beim ersten Start migriert
(das Original bleibt als `config.json.v1.bak` liegen).

Daten liegen in `%APPDATA%\Soundboard` (`config.json`, `sounds\`, `icons\`, `soundboard.log`).
Liegt ein Ordner `data` neben der exe, wird stattdessen er benutzt (portabel).
Sounds kann man auch direkt in `sounds\` kopieren – der Ordner wird überwacht.

Startoptionen: `-data <Ordner>`, `-port <n>`, `-no-browser`, `-version`.
`soundboard-console.exe` ist dieselbe App mit Konsole (Logs sichtbar).

## Bauen

Benötigt Node.js und Docker – **kein Go, kein C-Compiler auf dem Host**. Die Audio-Bibliothek (miniaudio) braucht
cgo; deshalb wird im Container (`build/Dockerfile`: Go + MinGW-w64) nach Windows cross-kompiliert.

```powershell
scripts\build.ps1             # Frontend + exe → dist\soundboard.exe, dist\soundboard-console.exe
scripts\build-backend.ps1     # nur das Backend (nimmt den vorhandenen Frontend-Build)
```

Entwickeln mit Live-Reload: Backend starten (`dist\soundboard-console.exe -no-browser`), dann
`cd frontend; npm run dev` → <http://localhost:5173/admin> (Proxy auf Port 8723).

Tests (reines Go, ohne cgo; auf dem Host oder im Container):

```powershell
cd backend
go test ./internal/audio ./internal/library ./internal/store ./internal/server
```

`dist\spike.exe` ist ein Diagnose-Werkzeug: ohne Argumente listet es die Audiogeräte, mit
`spike run <mikro> <ausgang> [monitor]` fährt es einen Dauertest mit Statistik (Teilstrings der Gerätenamen).

## Aufbau

```
backend/cmd/soundboard      main, Tray, Single-Instance (Windows)
backend/cmd/spike           Diagnose / Dauertest
backend/internal/audio      Engine, Busse, Voices, Ringpuffer, Drift-Ausgleich (reines Go, getestet)
backend/internal/audio/miniaudio   WASAPI-Anbindung via malgo (cgo)
backend/internal/winaudio   Standardgerät setzen (IPolicyConfig)
backend/internal/library    Sound-Ordner: dekodieren (wav/mp3/flac/ogg), überwachen, Upload
backend/internal/store      config.json, Validierung
backend/internal/server     REST + WebSocket, Admin-Schutz, statische Dateien
backend/web                 eingebetteter Frontend-Build (go:embed)
frontend/                   Vite + React + TypeScript (Board- und Admin-Ansicht)
```

Audio-Pfad: Mikrofon → Ringpuffer → (Drift-Ausgleich) → Main-Bus (+ Effekte) → Limiter → `CABLE Input`;
Effekte → Monitor-Bus → Limiter → Kopfhörer. Jedes Gerät hat eine eigene Uhr, deshalb hat jeder Bus eigene
Voice-Cursor; nur der Mikrofon-Pfad (Capture-Uhr ≠ Main-Uhr) braucht den Drift-Ausgleich.

## Sicherheit

Das Board ist bewusst ohne Passwort. Die Verwaltung ist nur von `localhost` erreichbar (Loopback-Client,
`Host`-Header gegen DNS-Rebinding, eigener Header gegen CSRF aus fremden Webseiten). Beim ersten Start fragt die
Windows-Firewall – „Privates Netzwerk“ zulassen.

Offene Punkte und getroffene Entscheidungen: [OPEN_QUESTIONS.md](OPEN_QUESTIONS.md). Ursprünglicher Plan: [PLAN.md](PLAN.md).
