# Offene Punkte & getroffene Entscheidungen

Während du weg warst, habe ich Phase 0–5 des Plans umgesetzt. Hier steht, was ich **selbst entschieden** habe
(zum Gegenlesen) und was **nur du testen/klären kannst**.

## 1. Nicht verifiziert – bitte auf deinem Rechner prüfen

Der Entwicklungsrechner hat weder `VB-Cable` noch ein Headset; ich konnte nur den Programmablauf prüfen
(Geräte öffnen, Mikrofon- und Wiedergabepfad laufen, Trigger → Pegel am Monitor, Web-Board per WebSocket).
**Nie gehört/gemessen wurde:**

- [ ] **Latenz und Knackser** der Mikrofon-Strecke mit echtem Kabel. Der Puffer-Zielwert ist 2 Perioden
      (≈ 20 ms bei 10 ms Periode) plus Geräte-Puffer, geschätzt 40–60 ms Gesamtlatenz. Falls zu hoch/knackst:
      Admin → Audio → Erweitert → Puffergröße. Der Drift-Ausgleich ist nur simuliert getestet (±1500 ppm, 60 s,
      keine Unter-/Überläufe) – ein **1-Stunden-Dauertest** auf echter Hardware steht aus
      (`dist\spike.exe run <mikro> "CABLE Input" <kopfhörer>` zeigt Unter-/Überläufe und Taktabweichung).
- [ ] **„Als Standard-Kommunikationsgerät setzen“** (`IPolicyConfig`, Windows-COM). Der Code kompiliert, wurde aber
      bewusst nicht ausgeführt, weil er Systemeinstellungen ändert. Bitte einmal ausprobieren.
- [ ] **Tray-Symbol** und Klick auf das Symbol (nur gebaut, nicht angesehen).
- [ ] **Windows-Ducking:** Ist ein Kommunikationsgerät aktiv, senkt Windows andere Audiostreams ab (auch den
      Monitor). Ich setze das nicht programmatisch zurück; im Admin steht ein Hinweis (Sound → Kommunikation →
      „Nichts unternehmen“). Falls das stört, könnte man per `IAudioSessionControl2::SetDuckingPreference` den
      Opt-out im Stream setzen – miniaudio bietet das nicht, wäre eigener Code.
- [ ] **Firewall-Dialog** beim ersten Start und Erreichbarkeit vom Handy.
- [ ] **Antivirus/SmartScreen:** unsignierte Go-exe, kann Fehlalarme auslösen.

## 2. Entscheidungen, die ich für dich getroffen habe

| Thema | Entscheidung | Warum / Alternative |
|---|---|---|
| Balance | Echte Balance (der gegenüberliegende Kanal wird leiser, der andere bleibt), kein Constant-Power-Panning | Du hattest „Balance-Regler“ gesagt; bei Stereo-Quellen ist das erwartbarer. Sounds sind intern immer Stereo (Mono wird gespiegelt). |
| Kachel-Verhalten | `Einmal`, `Neustart`, `Start/Stopp`, `Schleife`; zusätzlich pro Kachel Lautstärke, Farbe | Steuerung nötig, sonst sind Loops/Abbruch nicht möglich. |
| Board-Zugriff | Ohne PIN, jeder im Netz; Standard `bindLan = true` | Wie gewünscht. Umschaltbar auf „nur localhost“. |
| Admin | Nur über `http://localhost:<Port>/admin` erreichbar – **nicht** über die LAN-IP des eigenen Rechners | Schutz gegen Zugriff aus dem Netz/DNS-Rebinding. |
| Port | 8723, änderbar (wirkt nach Neustart) | |
| Datenordner | `%APPDATA%\Soundboard`, portabel wenn `data\` neben der exe liegt | |
| Erster Start | Mikrofon = Standard-Mikro, Monitor = Standard-Ausgabe, Main = `CABLE Input` (nur wenn installiert, sonst leer) | Ein echter Lautsprecher als Main würde sofort rückkoppeln. Dazu ein Testton + Beispielkachel. |
| Rückkopplungsschutz | `CABLE Output` kann nicht als Mikrofon gewählt werden | |
| Sound-Format | Alles wird beim Laden nach 48 kHz/Stereo/float32 dekodiert und liegt im RAM (≈ 23 MB/Minute); max. 10 Minuten pro Datei | Kein Festplattenzugriff im Audio-Thread. Lange Musik-Dateien sind damit unpraktisch. |
| Limiter | Harter Limiter bei 0,95, ca. 100 ms Release; Abbruch eines Sounds blendet 5 ms aus | gegen Übersteuern/Knackser |
| Raster verkleinern | Kacheln außerhalb bleiben gespeichert, sind aber unsichtbar (Hinweis im Admin) | Verhindert Datenverlust beim versehentlichen Verkleinern. |
| Icons | Emoji (Auswahl + freie Eingabe) oder hochgeladene Bilder (png/jpg/gif/webp/svg, ≤ 2 MB) | Lucide-Icon-Set für Kacheln nicht umgesetzt. |
| Frontend-Libs | React, lucide-react (UI-Icons), sonst nichts. Drag & Drop per HTML5 statt dnd-kit, Daten per `fetch` statt TanStack Query | Wenig Abhängigkeiten, reicht für den Umfang. HTML5-DnD funktioniert im Admin nicht per Touch (Admin ist für den Desktop gedacht; Kachel anlegen geht auch per Klick). |
| API-Typen | Von Hand gespiegelt (`frontend/src/shared/types.ts`), kein `tygo` | Ein Generator lohnt sich bei dieser Größe noch nicht. |
| Auto-Speichern | Audio-Einstellungen und Board speichern 0,4–0,5 s nach der letzten Änderung automatisch | Kein „Speichern“-Knopf; Fehler erscheinen als Hinweis unten. |
| Versionsverwaltung | `git init` gemacht, **nichts committet** | Du hast keinen Commit verlangt. |

## 3. Nicht umgesetzt (Phase 6 / optional)

Globale Hotkeys · Autostart mit Windows · Mikrofon-Ducking während Effekten · Fades pro Kachel ·
Board duplizieren · Import/Export · natives Fenster (Wails) · PWA-Service-Worker (nur Manifest, Installation als
„App“ geht, Offline-Betrieb nicht) · Mikrofon-Mithören im Monitor (du wolltest nur Effekte auf den Kopfhörern).

## 4. Fragen an dich

1. **VB-Cable mitliefern?** Die Lizenz erlaubt Weitergabe nur mit Vereinbarung; ich habe nur Erkennung + Anleitung
   eingebaut. Für ein rein privates Projekt kannst du es natürlich selbst installieren – oder soll ich Alternativen
   (z. B. Voicemeeter-Kabel) prüfen?
2. **Lizenz** fürs Repo (du sagtest „maximal Open Source“): MIT, Apache-2.0 oder GPL? Die genutzten Libs
   (malgo/miniaudio, beep, fsnotify, coder/websocket, systray, go-ole, go-qrcode) sind alle permissiv.
3. **Name** der App – aktuell einfach „Soundboard“ (exe, Tray, Titel).
4. **Optionaler PIN** für das Board, falls du es doch mal in fremden Netzen nutzt?
5. Sollen Kacheln später **Hotkeys** bekommen (z. B. F13–F24 für ein Stream-Deck-artiges Setup)?

## 5. Mehrere Boards (Entscheidungen)

- **Pfade:** `/boards` ist die Übersicht, `/boards/<pfad>` ein Board. `/` leitet auf `/boards` um (frühere Lesezeichen auf `/`
  landen also in der Übersicht). Die alte API `GET /api/board` gibt es nicht mehr.
- **Pfad = Board-ID:** wird beim Anlegen aus dem Namen erzeugt (`Zum Lachen` → `zum-lachen`), bleibt beim Umbenennen
  stabil und ist separat änderbar (a–z, 0–9, `-`, max. 40 Zeichen). Wer einen Pfad ändert, macht bestehende Lesezeichen/QR-Codes ungültig.
- **Kachel-IDs sind global eindeutig** (Auslösen und „spielt gerade“ laufen nur über die Kachel-ID); gleiche Rasterpositionen
  auf verschiedenen Boards sind selbstverständlich erlaubt.
- **Übersicht:** auch bei nur einem Board eine Übersicht (kein Auto-Weiterleiten) – wer direkt starten will, legt `/boards/<pfad>`
  als Lesezeichen an.
- **Migration:** eine `config.json` mit einem einzelnen Board wird beim Start in das neue Format überführt, das Original bleibt als
  `config.json.v1.bak` liegen. Es gibt höchstens 50 Boards, mindestens eins.
- Ein Board zu löschen entfernt seine Kacheln, nicht die Sounds. Ein offenes Board auf einem Handy zeigt danach „Dieses Board gibt es nicht (mehr)“.
