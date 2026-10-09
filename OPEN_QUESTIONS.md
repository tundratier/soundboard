# Open points & decisions taken

While you were away I implemented phases 0-5 of the plan. This file lists what I **decided on my own** (for you to
review) and what **only you can test or clarify**.

## 1. Not verified - please check on your machine

The development machine had neither VB-Cable nor a headset; I could only verify the program flow (opening devices,
microphone and playback paths running, trigger → level at the monitor, web board via WebSocket).
**Never listened to or measured:**

- [ ] **Latency and crackles** of the microphone path with a real cable. The buffer target is 2 periods
      (≈ 20 ms at a 10 ms period) plus the device buffers, estimated at 40-60 ms total latency. If it is too high or
      crackles: Admin → Audio → "Erweitert" → buffer size. The drift compensation has only been tested in simulation
      (±1500 ppm, 60 s, no underruns/overruns) - a **1-hour soak test** on real hardware is still missing
      (`dist\spike.exe run <mic> "CABLE Input" <headphones>` shows underruns/overruns and clock drift).
- [ ] **"Als Standard-Kommunikationsgerät setzen"** (set as default communications device; `IPolicyConfig`, Windows
      COM). The code compiles but was deliberately not executed because it changes system settings. Please try it once.
- [ ] **Tray icon** and clicking it (built, but never looked at).
- [ ] **Windows ducking:** while a communications device is active, Windows lowers other audio streams (including the
      monitor). I do not reset that programmatically; the admin UI shows a hint (Sound → Communications → "Do
      nothing"). If it bothers you, the opt-out could be set per stream via
      `IAudioSessionControl2::SetDuckingPreference` - miniaudio does not offer that, so it would be custom code.
- [ ] **Firewall dialog** on first start and reachability from a phone.
- [ ] **Antivirus/SmartScreen:** an unsigned Go exe can trigger false positives.

## 2. Decisions I took for you

| Topic | Decision | Why / alternative |
|---|---|---|
| Balance | A true balance control (the opposite channel is attenuated, the other stays), not constant-power panning | You said "balance control"; that is what people expect for stereo sources. Sounds are always stereo internally (mono is mirrored). |
| Tile behavior | `Einmal` (once), `Neustart` (restart), `Start/Stopp` (toggle), `Schleife` (loop); plus volume and color per tile | Needed, otherwise loops/aborting are impossible. |
| Board access | No PIN, everyone on the network; default `bindLan = true` | As requested. Can be switched to "localhost only". |
| Admin | Only reachable via `http://localhost:<port>/admin` - **not** via the PC's own LAN IP | Protection against access from the network/DNS rebinding. |
| Port | 8723, changeable (takes effect after a restart) | |
| Data folder | `%APPDATA%\Soundboard`, portable if `data\` sits next to the exe | |
| First start | Microphone = default mic, monitor = default output, main = `CABLE Input` (only if installed, otherwise empty) | A real loudspeaker as main would feed the microphone back immediately. Also creates a test beep and an example tile. |
| Feedback protection | `CABLE Output` cannot be selected as the microphone | |
| Sound format | Everything is decoded to 48 kHz/stereo/float32 on load and kept in RAM (≈ 23 MB per minute); max. 10 minutes per file | No disk access on the audio thread. Long music files are impractical. |
| Limiter | Hard limiter at 0.95, ~100 ms release; aborting a sound fades out over 5 ms | Against overload/clicks |
| Shrinking the grid | Tiles outside the grid stay stored but are invisible (hint in the admin UI) | Prevents data loss when shrinking by accident. |
| Icons | Emoji (picker + free input) or uploaded images (png/jpg/gif/webp/svg, ≤ 2 MB) | The Lucide icon set for tiles is not implemented. |
| Frontend libraries | React, lucide-react (UI icons), nosleep.js (keep the screen on), nothing else. HTML5 drag & drop instead of dnd-kit, plain `fetch` instead of TanStack Query | Few dependencies, enough for this size. HTML5 drag & drop does not work by touch in the admin UI (the admin UI is meant for the desktop; creating a tile also works by click). |
| API types | Mirrored by hand (`frontend/src/shared/types.ts`), no `tygo` | A generator is not worth it at this size. |
| Auto-save | Audio settings and boards are saved 0.4-0.5 s after the last change | No "save" button; errors appear as a toast at the bottom. |
| Language | UI texts are German (frontend; on the Go side tray menu, startup error dialogs and the example tile, all collected in `backend/cmd/soundboard/ui_de.go`); code, comments, logs, API error messages, docs and commit messages are English | API errors that reach the UI as toasts therefore appear in English after a German prefix; device status errors have a code that the UI translates. |
| License | MIT (`LICENSE`), copyright holder `tundratier` (the GitHub account) | You asked for MIT. I did not know your legal name, so please put the name you want into `LICENSE`. All dependencies are permissive. |

## 3. Not implemented (phase 6 / optional)

Global hotkeys · autostart with Windows · microphone ducking while effects play · fades per tile ·
duplicating a board · import/export · native window (Wails) · PWA service worker (manifest only: installing as an
"app" works, offline use does not) · listening to the microphone in the monitor (you only wanted effects on the
headphones).

## 4. Questions for you

1. **Bundle VB-Cable?** The license only allows redistribution by agreement; I only built detection + instructions.
   For a purely private project you can of course install it yourself - or should I look at alternatives
   (e.g. the Voicemeeter cable)?
2. **Name** of the app - currently simply "Soundboard" (exe, tray, title).
3. **Optional PIN** for the boards, in case you ever use it on foreign networks?
4. Should tiles get **hotkeys** later (e.g. F13-F24 for a Stream-Deck-like setup)?

## 5. Multiple boards (decisions)

- **Paths:** `/boards` is the overview, `/boards/<path>` is one board. `/` redirects to `/boards` (so old bookmarks
  to `/` land in the overview). The old API `GET /api/board` no longer exists.
- **Path = board ID:** generated from the name when the board is created (`Zum Lachen` → `zum-lachen`), stays stable
  when renaming, and can be changed separately (a-z, 0-9, `-`, max. 40 characters). Changing a path invalidates
  existing bookmarks/QR codes for that board.
- **Tile IDs are globally unique** (triggering and "currently playing" are keyed by tile ID alone); identical grid
  positions on different boards are of course fine.
- **Overview:** shown even when there is only one board (no auto-redirect) - bookmark `/boards/<path>` to start
  directly.
- **Migration:** a `config.json` with a single board is converted to the new format on start; the original is kept as
  `config.json.v1.bak`. There are at most 50 boards, at least one.
- Deleting a board removes its tiles, not the sounds. A board that is open on a phone then shows "Dieses Board gibt es
  nicht (mehr)" (this board no longer exists).

## 6. CI release workflow (GitHub Actions)

- **Verified on GitHub:** pushing `v0.1.2` ran the workflow (frontend build, mingw cross-compile on the runner,
  `gh release create`) and published the release. Re-running for an existing release (update notes, replace assets) and
  the error paths were only tested locally against stand-ins for `gh` and `zip`.
- **Moved from Bitbucket:** the earlier Bitbucket pipeline published to the repository downloads, which the free plan does
  not have. GitHub has native releases, so the release page, notes and assets replace that. No secrets are needed; the
  workflow uses the built-in `GITHUB_TOKEN` with `contents: write`.
- **Trigger:** every push to `master`, but it only publishes when `HEAD` carries a release tag. Pushing `master` without
  the tag does nothing; start the workflow by hand after pushing the tag. Tags are pushed with `--follow-tags`.
- **No releases for `v0.1.0` and `v0.1.1`.** They predate the workflow and exist on GitHub only as tags. `v0.1.2` is the
  first GitHub release; it does not contain the `LICENSE` file yet (the zip includes it from the next release on).
- Not covered: tests or a typecheck on `develop`/pull requests (easy to add as another workflow), signing the exe.
