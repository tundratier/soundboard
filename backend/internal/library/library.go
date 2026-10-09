// Package library manages the sound folder: decoding (WAV/MP3/FLAC/OGG) to 48 kHz stereo float32,
// watching the folder, and upload/delete.
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

// maxFrames limits the length of a sound (RAM: ~23 MB per minute).
const maxFrames = 10 * 60 * audio.SampleRate

var extensions = map[string]bool{".wav": true, ".mp3": true, ".flac": true, ".ogg": true}

// Info describes a sound for the UI.
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

// New opens (and creates if needed) the sound folder. onChange is called after every change (may be nil).
func New(dir string, onChange func()) (*Library, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Library{dir: dir, entries: map[string]*entry{}, onChange: onChange}, nil
}

func (l *Library) Dir() string { return l.dir }

// Supported checks the file extension.
func Supported(name string) bool { return extensions[strings.ToLower(filepath.Ext(name))] }

// Scan (re)loads all files in the folder.
func (l *Library) Scan() {
	files, err := os.ReadDir(l.dir)
	if err != nil {
		log.Printf("library: reading folder: %v", err)
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

// reload loads a file if it is new or changed. Returns true if anything changed.
func (l *Library) reload(name string) bool {
	path := filepath.Join(l.dir, name)
	st, err := os.Stat(path)
	if err != nil { // gone
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

// Get returns the decoded sound (nil if unknown or broken).
func (l *Library) Get(name string) *audio.Sound {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if e := l.entries[name]; e != nil {
		return e.snd
	}
	return nil
}

// List returns all sounds alphabetically.
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

// SanitizeName turns an upload name into a safe file name.
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

// Save stores an upload. If the name already exists, " (2)" etc. is appended. Returns the final name.
func (l *Library) Save(name string, r io.Reader) (string, error) {
	name = SanitizeName(name)
	if name == "" || !Supported(name) {
		return "", errors.New("unsupported file type (allowed: wav, mp3, flac, ogg)")
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

// Delete removes a sound including its file.
func (l *Library) Delete(name string) error {
	if name != SanitizeName(name) || !Supported(name) {
		return errors.New("invalid name")
	}
	if err := os.Remove(filepath.Join(l.dir, name)); err != nil && !os.IsNotExist(err) {
		return err
	}
	l.reload(name)
	l.changed()
	return nil
}

// Path returns the file path (for downloads/audition in the browser), empty if unknown.
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

// Watch watches the folder until stop is closed.
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
					pending[name] = time.Now().Add(400 * time.Millisecond) // debounce: the file may still be being written
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

// Decode reads an audio file completely and converts it to 48 kHz stereo float32.
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
		return nil, errors.New("unsupported file type")
	}
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("decode: %w", err)
	}
	defer s.Close() // also closes f

	if format.SampleRate <= 0 {
		return nil, errors.New("invalid sample rate")
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
			return nil, fmt.Errorf("too long (max. %d minutes)", maxFrames/audio.SampleRate/60)
		}
		if !ok {
			break
		}
	}
	if err := st.Err(); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if len(data) == 0 {
		return nil, errors.New("empty file")
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
