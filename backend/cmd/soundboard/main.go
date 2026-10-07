// soundboard: local soundboard for Windows – microphone + effects → virtual cable, board in the browser.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"soundboard/internal/audio"
	"soundboard/internal/audio/miniaudio"
	"soundboard/internal/library"
	"soundboard/internal/server"
	"soundboard/internal/store"
	"soundboard/web"
)

var version = "0.1.0"

func main() {
	dataFlag := flag.String("data", "", "Datenordner (Standard: ./data neben der exe, falls vorhanden, sonst %APPDATA%\\Soundboard)")
	portFlag := flag.Int("port", 0, "Port überschreiben")
	noBrowser := flag.Bool("no-browser", false, "Admin-Seite beim Start nicht öffnen")
	showVersion := flag.Bool("version", false, "Version ausgeben")
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return
	}

	dataDir := resolveDataDir(*dataFlag)
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		fatalBox("Datenordner nicht anlegbar: " + err.Error())
	}
	setupLogging(dataDir)
	log.Printf("Soundboard %s, Daten in %s", version, dataDir)

	cfgPath := filepath.Join(dataDir, "config.json")
	st, created, err := store.Open(cfgPath)
	if st == nil {
		fatalBox("Konfiguration nicht lesbar: " + err.Error())
	}
	if err != nil {
		log.Printf("config: %v", err)
	}
	if *portFlag > 0 {
		st.Update(func(c *store.Config) error { c.Server.Port = *portFlag; return nil })
	}
	adminURL := fmt.Sprintf("http://localhost:%d/admin", st.Get().Server.Port)

	if !acquireSingleInstance() {
		log.Printf("läuft bereits – öffne Admin-Seite")
		openBrowser(adminURL)
		return
	}

	// Sounds
	var srv *server.Server
	lib, err := library.New(filepath.Join(dataDir, "sounds"), func() {
		if srv != nil {
			srv.Broadcast(map[string]string{"type": "board"})
		}
	})
	if err != nil {
		fatalBox("Sound-Ordner: " + err.Error())
	}
	if created {
		seedFirstRun(lib, st)
	}
	t0 := time.Now()
	lib.Scan()
	log.Printf("library: %d Sounds in %v geladen", len(lib.List()), time.Since(t0).Round(time.Millisecond))
	stopWatch := make(chan struct{})
	if err := lib.Watch(stopWatch); err != nil {
		log.Printf("library: Ordner-Überwachung nicht verfügbar: %v", err)
	}

	// Audio
	be, err := miniaudio.New()
	if err != nil {
		fatalBox("Audio-System nicht verfügbar: " + err.Error())
	}
	eng := audio.NewEngine(be)
	if created {
		autoSelectDevices(eng, st)
	}
	eng.Apply(st.Get().Audio)

	// Server
	srv = server.New(server.Deps{
		Engine: eng, Lib: lib, Store: st,
		IconsDir: filepath.Join(dataDir, "icons"),
		Web:      web.FS(), Platform: windowsPlatform{}, DataDir: dataDir, Version: version,
	})
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Run() }()

	shutdown := func() {
		close(stopWatch)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
		eng.Close()
	}

	go func() {
		if err := <-serveErr; err != nil {
			log.Printf("server: %v", err)
			fatalBox("Webserver konnte nicht starten:\n" + err.Error() + "\n\nAnderer Port: soundboard.exe -port 9000")
		}
	}()

	if created && !*noBrowser {
		go func() { time.Sleep(500 * time.Millisecond); openBrowser(adminURL) }()
	}

	boardURL := fmt.Sprintf("http://localhost:%d/", st.Get().Server.Port)
	runTray(adminURL, boardURL, shutdown) // blocks until "Quit"
	log.Printf("beendet")
}

func resolveDataDir(flagVal string) string {
	if flagVal != "" {
		return flagVal
	}
	if exe, err := os.Executable(); err == nil {
		portable := filepath.Join(filepath.Dir(exe), "data")
		if st, err := os.Stat(portable); err == nil && st.IsDir() {
			return portable
		}
	}
	if appdata := os.Getenv("APPDATA"); appdata != "" {
		return filepath.Join(appdata, "Soundboard")
	}
	return "data"
}

func setupLogging(dataDir string) {
	f, err := os.OpenFile(filepath.Join(dataDir, "soundboard.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	if st, err := f.Stat(); err == nil && st.Size() > 2<<20 { // simple rotation
		f.Close()
		os.Rename(f.Name(), f.Name()+".old")
		f, err = os.OpenFile(filepath.Join(dataDir, "soundboard.log"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			return
		}
	}
	log.SetOutput(teeLog{file: f, console: os.Stderr})
}

// seedFirstRun creates an example sound and an example tile so there is something to try right away.
func seedFirstRun(lib *library.Library, st *store.Store) {
	name := "Testton.wav"
	if err := library.WriteTestTone(filepath.Join(lib.Dir(), name)); err != nil {
		log.Printf("seed: %v", err)
		return
	}
	_, err := st.Update(func(c *store.Config) error {
		c.Board.Tiles = append(c.Board.Tiles, store.Tile{
			ID: store.NewID(), X: 0, Y: 0, Label: "Testton", Sound: name,
			Icon: &store.Icon{Kind: "emoji", Value: "🔔"}, Gain: 1, Mode: audio.ModeOneShot, Color: "#4dabf7",
		})
		return nil
	})
	if err != nil {
		log.Printf("seed: %v", err)
	}
}

// autoSelectDevices picks sensible devices on first start:
// microphone = default capture device, main = CABLE Input (if installed), monitor = default playback device.
// Otherwise main stays empty – a real loudspeaker would feed the microphone back immediately.
func autoSelectDevices(eng *audio.Engine, st *store.Store) {
	devs, err := eng.Devices()
	if err != nil {
		log.Printf("autoselect: %v", err)
		return
	}
	var in, mon, cable string
	for _, d := range devs {
		switch {
		case d.Kind == audio.KindCapture && d.IsDefault && !strings.Contains(strings.ToLower(d.Name), "cable output"):
			in = d.ID
		case d.Kind == audio.KindPlayback && d.IsDefault && !strings.Contains(strings.ToLower(d.Name), "cable input"):
			mon = d.ID
		case d.Kind == audio.KindPlayback && strings.Contains(strings.ToLower(d.Name), "cable input"):
			cable = d.ID
		}
	}
	st.Update(func(c *store.Config) error {
		c.Audio.InputID, c.Audio.MonitorID, c.Audio.MainID = in, mon, cable
		return nil
	})
}

// teeLog writes to the log file and – best effort – to stderr. The GUI exe has no valid stderr;
// io.MultiWriter would abort there on the first error.
type teeLog struct{ file, console io.Writer }

func (t teeLog) Write(p []byte) (int, error) {
	t.console.Write(p) //nolint:errcheck
	return t.file.Write(p)
}
