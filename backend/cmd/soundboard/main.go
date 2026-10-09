// soundboard: local soundboard for Windows - microphone + effects → virtual cable, board in the browser.
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
	"soundboard/internal/i18n"
	"soundboard/internal/library"
	"soundboard/internal/server"
	"soundboard/internal/store"
	"soundboard/web"
)

// version is set at build time (-ldflags "-X main.version=..."), derived from git tags by scripts/version.ps1.
var version = "dev"

// All user-facing texts (dialogs, tray menu, flag help, console output) come from internal/i18n, in the language
// given by -lang, else the saved setting, else the Windows display language. The log file always stays English.

func main() {
	// The flag help is localized, so the language must be known before the flags are defined: take -lang from the
	// raw arguments for now and settle the final language once the saved setting is loaded.
	i18n.Set(i18n.Resolve(langFromArgs(os.Args[1:])))

	dataFlag := flag.String("data", "", i18n.T(i18n.FlagData))
	portFlag := flag.Int("port", 0, i18n.T(i18n.FlagPort))
	noBrowser := flag.Bool("no-browser", false, i18n.T(i18n.FlagNoBrowser))
	showVersion := flag.Bool("version", false, i18n.T(i18n.FlagVersion))
	langFlag := flag.String("lang", "", i18n.T(i18n.FlagLang))
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), i18n.T(i18n.FlagUsage)+"\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return
	}
	langOverride, langOverridden := "", *langFlag != ""
	if langOverridden {
		var ok bool
		if langOverride, ok = i18n.Normalize(*langFlag); !ok {
			fatalBox(fmt.Sprintf(i18n.T(i18n.ErrBadLang), *langFlag), "unknown language "+*langFlag)
		}
	}

	dataDir := resolveDataDir(*dataFlag)
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		fatalBox(fmt.Sprintf(i18n.T(i18n.ErrDataFolder), err), "cannot create data folder: "+err.Error())
	}
	setupLogging(dataDir)

	cfgPath := filepath.Join(dataDir, "config.json")
	st, created, err := store.Open(cfgPath)
	if st == nil {
		fatalBox(fmt.Sprintf(i18n.T(i18n.ErrConfig), err), "cannot read config: "+err.Error())
	}
	// The flag wins for this run; otherwise the saved setting counts ("" = follow the system language).
	langSetting := st.Get().Language
	if langOverridden {
		langSetting = langOverride
	}
	i18n.Set(i18n.Resolve(langSetting))

	i18n.Logf(i18n.LogStart, version, dataDir)
	i18n.Logf(i18n.LogLanguage, string(i18n.Current()))
	if err != nil {
		i18n.Logf(i18n.LogConfig, err)
	}
	if *portFlag > 0 {
		st.Update(func(c *store.Config) error { c.Server.Port = *portFlag; return nil })
	}
	adminURL := fmt.Sprintf("http://localhost:%d/admin", st.Get().Server.Port)

	if !acquireSingleInstance() {
		i18n.Logf(i18n.LogAlreadyRunning)
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
		fatalBox(fmt.Sprintf(i18n.T(i18n.ErrSoundFolder), err), "sound folder: "+err.Error())
	}
	if created {
		seedFirstRun(lib, st)
	}
	t0 := time.Now()
	lib.Scan()
	i18n.Logf(i18n.LogLibraryLoaded, len(lib.List()), time.Since(t0).Round(time.Millisecond))
	stopWatch := make(chan struct{})
	if err := lib.Watch(stopWatch); err != nil {
		i18n.Logf(i18n.LogWatchUnavailable, err)
	}

	// Audio
	be, err := miniaudio.New()
	if err != nil {
		fatalBox(fmt.Sprintf(i18n.T(i18n.ErrAudioSystem), err), "audio system unavailable: "+err.Error())
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
			i18n.Logf(i18n.LogServerError, err)
			fatalBox(fmt.Sprintf(i18n.T(i18n.ErrWebServer), err), "web server could not start: "+err.Error())
		}
	}()

	i18n.Logf(i18n.LogAdminURL, adminURL)
	if created && !*noBrowser {
		go func() { time.Sleep(500 * time.Millisecond); openBrowser(adminURL) }()
	}

	// Chosen in the tray menu: save, switch the shell texts and tell open web pages to reload in the new language.
	setLanguage := func(setting string) {
		if _, err := st.Update(func(c *store.Config) error { c.Language = setting; return nil }); err != nil {
			i18n.Logf(i18n.LogConfig, err)
		}
		i18n.Set(i18n.Resolve(setting))
		i18n.Logf(i18n.LogLanguage, string(i18n.Current()))
		srv.Broadcast(map[string]string{"type": "language"})
	}

	boardURL := fmt.Sprintf("http://localhost:%d/boards", st.Get().Server.Port)
	runTray(trayConfig{AdminURL: adminURL, BoardURL: boardURL, Language: langSetting, OnLanguage: setLanguage}, shutdown) // blocks until "Quit"
	i18n.Logf(i18n.LogExited)
}

// langFromArgs finds the value of -lang / --lang (also in the -lang=de form) without involving the flag package.
// Anything it cannot make sense of yields "", i.e. the system language; the flag package reports real errors later.
func langFromArgs(args []string) string {
	for i, a := range args {
		a = strings.TrimLeft(a, "-")
		switch {
		case a == "lang" && i+1 < len(args):
			return args[i+1]
		case strings.HasPrefix(a, "lang="):
			return strings.TrimPrefix(a, "lang=")
		}
	}
	return ""
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

// setupLogging sets up the log file (English, rotated) and the console. Messages logged through i18n go to the
// file in English and to the console in the active language; plain log lines (for example from the audio library)
// go to both unchanged.
func setupLogging(dataDir string) {
	console := log.New(os.Stderr, "", log.LstdFlags) // the GUI exe has no valid stderr; Logger ignores write errors
	f, err := os.OpenFile(filepath.Join(dataDir, "soundboard.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		i18n.SetLogSinks(nil, console)
		return
	}
	if st, err := f.Stat(); err == nil && st.Size() > 2<<20 { // simple rotation
		f.Close()
		os.Rename(f.Name(), f.Name()+".old")
		f, err = os.OpenFile(filepath.Join(dataDir, "soundboard.log"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			i18n.SetLogSinks(nil, console)
			return
		}
	}
	log.SetOutput(teeLog{file: f, console: os.Stderr})
	i18n.SetLogSinks(log.New(f, "", log.LstdFlags), console)
}

// seedFirstRun creates an example sound and an example tile so there is something to try right away.
// Its name and label are user-visible and therefore come from the active language.
func seedFirstRun(lib *library.Library, st *store.Store) {
	name := i18n.T(i18n.ExampleSound)
	if err := library.WriteTestTone(filepath.Join(lib.Dir(), name)); err != nil {
		i18n.Logf(i18n.LogSeed, err)
		return
	}
	_, err := st.Update(func(c *store.Config) error {
		c.Boards[0].Tiles = append(c.Boards[0].Tiles, store.Tile{
			ID: store.NewID(), X: 0, Y: 0, Label: i18n.T(i18n.ExampleTile), Sound: name,
			Icon: &store.Icon{Kind: "emoji", Value: "🔔"}, Gain: 1, Mode: audio.ModeOneShot, Color: "#4dabf7",
		})
		return nil
	})
	if err != nil {
		i18n.Logf(i18n.LogSeed, err)
	}
}

// autoSelectDevices picks sensible devices on first start:
// microphone = default capture device, main = CABLE Input (if installed), monitor = default playback device.
// Otherwise main stays empty - a real loudspeaker would feed the microphone back immediately.
func autoSelectDevices(eng *audio.Engine, st *store.Store) {
	devs, err := eng.Devices()
	if err != nil {
		i18n.Logf(i18n.LogAutoselect, err)
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

// teeLog writes to the log file and - best effort - to stderr. The GUI exe has no valid stderr;
// io.MultiWriter would abort there on the first error.
type teeLog struct{ file, console io.Writer }

func (t teeLog) Write(p []byte) (int, error) {
	t.console.Write(p) //nolint:errcheck
	return t.file.Write(p)
}
