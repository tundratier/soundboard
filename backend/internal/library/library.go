// Package library verwaltet den Sound-Ordner: Dekodieren (WAV/MP3/FLAC/OGG) in 48-kHz-Stereo-float32,
// Überwachen des Ordners und Upload/Löschen.
package library

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/flac"
	"github.com/gopxl/beep/v2/mp3"
	"github.com/gopxl/beep/v2/vorbis"
	"github.com/gopxl/beep/v2/wav"

	"soundboard/internal/audio"
)

// maxFrames begrenzt die Länge eines Sounds (RAM: ~23 MB pro Minute).
const maxFrames = 10 * 60 * audio.SampleRate

var extensions = map[string]bool{".wav": true, ".mp3": true, ".flac": true, ".ogg": true}

// Info beschreibt einen Sound für die UI.
type Info struct {
	Name       string `json:"name"`
	DurationMs int    `json:"durationMs"`
	SizeBytes  int64  `json:"sizeBytes"`
	Error      string `json:"error,omitempty"`
}

type entry struct {
	info  Info
	snd   *audio.Sound
	mtime time.Time
}

type Library struct {
	dir string

	mu      sync.RWMutex
	entries map[string]*entry

	onChange func()
}

// New öffnet (und erzeugt ggf.) den Sound-Ordner. onChange wird nach jeder Änderung aufgerufen (darf nil sein).
func New(dir string, onChange func()) (*Library, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Library{dir: dir, entries: map[string]*entry{}, onChange: onChange}, nil
}

func (l *Library) Dir() string { return l.dir }

// Supported prüft die Dateiendung.
func Supported(name string) bool { return extensions[strings.ToLower(filepath.Ext(name))] }

// Scan lädt alle Dateien des Ordners (neu).
func (l *Library) Scan() {
	files, err := os.ReadDir(l.dir)
	if err != nil {
		log.Printf("library: Ordner lesen: %v", err)
		return
	}
	seen := map[string]bool{}
	for _, f := range files {
		if f.IsDir() || !Supported(f.Name()) {
			continue
		}
		seen[f.Name()] = true
		l.reload(f.Name())
	}
	l.mu.Lock()
	for name := range l.entries {
		if !seen[name] {
			delete(l.entries, name)
		}
	}
	l.mu.Unlock()
	l.changed()
}

func (l *Library) changed() {
	if l.onChange != nil {
		l.onChange()
	}
}

// reload lädt eine Datei, sofern sie neu oder geändert ist. Liefert true, wenn sich etwas geändert hat.
func (l *Library) reload(name string) bool {
	path := filepath.Join(l.dir, name)
	st, err := os.Stat(path)
	if err != nil { // verschwunden
		l.mu.Lock()
		_, existed := l.entries[name]
		delete(l.entries, name)
		l.mu.Unlock()
		return existed
	}
	l.mu.RLock()
	old := l.entries[name]
	l.mu.RUnlock()
	if old != nil && old.mtime.Equal(st.ModTime()) && old.info.SizeBytes == st.Size() {
		return false
	}
	e := &entry{mtime: st.ModTime(), info: Info{Name: name, SizeBytes: st.Size()}}
	snd, err := Decode(path)
	if err != nil {
		e.info.Error = err.Error()
		log.Printf("library: %s: %v", name, err)
	} else {
		snd.Name = name
		e.snd = snd
		e.info.DurationMs = snd.Frames * 1000 / audio.SampleRate
	}
	l.mu.Lock()
	l.entries[name] = e
	l.mu.Unlock()
	return true
}

// Get liefert den dekodierten Sound (nil, wenn unbekannt oder fehlerhaft).
func (l *Library) Get(name string) *audio.Sound {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if e := l.entries[name]; e != nil {
		return e.snd
	}
	return nil
}

// List liefert alle Sounds alphabetisch.
func (l *Library) List() []Info {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]Info, 0, len(l.entries))
	for _, e := range l.entries {
		out = append(out, e.info)
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out
}

// SanitizeName macht aus einem Upload-Namen einen sicheren Dateinamen.
func SanitizeName(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`<>:"/\|?*`, r) || r < 32 {
			return '_'
		}
		return r
	}, name)
	return strings.Trim(name, " .")
}

// Save speichert einen Upload. Existiert der Name schon, wird " (2)" usw. angehängt. Liefert den endgültigen Namen.
func (l *Library) Save(name string, r io.Reader) (string, error) {
	name = SanitizeName(name)
	if name == "" || !Supported(name) {
		return "", errors.New("Dateityp nicht unterstützt (erlaubt: wav, mp3, flac, ogg)")
	}
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	final := name
	for i := 2; ; i++ {
		if _, err := os.Stat(filepath.Join(l.dir, final)); os.IsNotExist(err) {
			break
		}
		final = fmt.Sprintf("%s (%d)%s", base, i, ext)
	}
	tmp, err := os.CreateTemp(l.dir, ".upload-*.part")
	if err != nil {
		return "", err
	}
	_, err = io.Copy(tmp, r)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	if err := os.Rename(tmp.Name(), filepath.Join(l.dir, final)); err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	l.reload(final)
	l.changed()
	return final, nil
}

// Delete entfernt einen Sound samt Datei.
func (l *Library) Delete(name string) error {
	if name != SanitizeName(name) || !Supported(name) {
		return errors.New("ungültiger Name")
	}
	if err := os.Remove(filepath.Join(l.dir, name)); err != nil && !os.IsNotExist(err) {
		return err
	}
	l.reload(name)
	l.changed()
	return nil
}

// Path liefert den Dateipfad (für Downloads/Vorhören im Browser), leer wenn unbekannt.
func (l *Library) Path(name string) string {
	if name != SanitizeName(name) || !Supported(name) {
		return ""
	}
	p := filepath.Join(l.dir, name)
	if _, err := os.Stat(p); err != nil {
		return ""
	}
	return p
}

// Watch überwacht den Ordner, bis stop geschlossen wird.
func (l *Library) Watch(stop <-chan struct{}) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	if err := w.Add(l.dir); err != nil {
		w.Close()
		return err
	}
	go func() {
		defer w.Close()
		pending := map[string]time.Time{}
		tick := time.NewTicker(200 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-stop:
				return
			case ev, ok := <-w.Events:
				if !ok {
					return
				}
				name := filepath.Base(ev.Name)
				if Supported(name) {
					pending[name] = time.Now().Add(400 * time.Millisecond) // entprellen: Datei wird ggf. noch geschrieben
				}
			case err, ok := <-w.Errors:
				if !ok {
					return
				}
				log.Printf("library: watcher: %v", err)
			case <-tick.C:
				changed := false
				for name, due := range pending {
					if time.Now().After(due) {
						delete(pending, name)
						if l.reload(name) {
							changed = true
						}
					}
				}
				if changed {
					l.changed()
				}
			}
		}
	}()
	return nil
}

// Decode liest eine Audiodatei vollständig und wandelt sie in 48-kHz-Stereo-float32.
func Decode(path string) (*audio.Sound, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	var (
		s      beep.StreamSeekCloser
		format beep.Format
	)
	switch strings.ToLower(filepath.Ext(path)) {
	case ".wav":
		s, format, err = wav.Decode(f)
	case ".mp3":
		s, format, err = mp3.Decode(f)
	case ".flac":
		s, format, err = flac.Decode(f)
	case ".ogg":
		s, format, err = vorbis.Decode(f)
	default:
		f.Close()
		return nil, errors.New("Dateityp nicht unterstützt")
	}
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("dekodieren: %w", err)
	}
	defer s.Close() // schließt auch f

	if format.SampleRate <= 0 {
		return nil, errors.New("ungültige Samplerate")
	}
	var st beep.Streamer = s
	if int(format.SampleRate) != audio.SampleRate {
		st = beep.Resample(4, format.SampleRate, audio.SampleRate, s)
	}

	data := make([]float32, 0, 1<<16)
	buf := make([][2]float64, 4096)
	for {
		n, ok := st.Stream(buf)
		for i := 0; i < n; i++ {
			data = append(data, clamp(buf[i][0]), clamp(buf[i][1]))
		}
		if len(data)/audio.Channels > maxFrames {
			return nil, fmt.Errorf("zu lang (max. %d Minuten)", maxFrames/audio.SampleRate/60)
		}
		if !ok {
			break
		}
	}
	if err := st.Err(); err != nil {
		return nil, fmt.Errorf("dekodieren: %w", err)
	}
	if len(data) == 0 {
		return nil, errors.New("leere Datei")
	}
	return &audio.Sound{Data: data, Frames: len(data) / audio.Channels}, nil
}

func clamp(v float64) float32 {
	if v > 1 {
		return 1
	}
	if v < -1 {
		return -1
	}
	return float32(v)
}
