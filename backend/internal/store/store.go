// Package store hält die Konfiguration (config.json) und validiert Änderungen.
package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"soundboard/internal/audio"
)

const (
	CurrentVersion = 1
	MaxGrid        = 12
)

type Icon struct {
	Kind  string `json:"kind"`  // "emoji" | "file"
	Value string `json:"value"` // Emoji bzw. Dateiname im icons/-Ordner
}

type Tile struct {
	ID    string         `json:"id"`
	X     int            `json:"x"`
	Y     int            `json:"y"`
	Label string         `json:"label,omitempty"`
	Sound string         `json:"sound"` // Dateiname im sounds/-Ordner
	Icon  *Icon          `json:"icon,omitempty"`
	Pan   float32        `json:"pan"`  // Balance -1 … +1
	Gain  float32        `json:"gain"` // 0 … 2
	Mode  audio.PlayMode `json:"mode"`
	Color string         `json:"color,omitempty"`
}

type Board struct {
	Name  string `json:"name"`
	Cols  int    `json:"cols"`
	Rows  int    `json:"rows"`
	Tiles []Tile `json:"tiles"`
}

type Server struct {
	Port    int  `json:"port"`
	BindLAN bool `json:"bindLan"` // true: im ganzen Netz erreichbar (Board), false: nur localhost
}

type Config struct {
	Version int            `json:"version"`
	Audio   audio.Settings `json:"audio"`
	Server  Server         `json:"server"`
	Board   Board          `json:"board"`
}

func Default() Config {
	return Config{
		Version: CurrentVersion,
		Audio:   audio.Settings{MicGain: 1, FxGainMain: 1, FxGainMonitor: 1, PeriodMs: 10},
		Server:  Server{Port: 8723, BindLAN: true},
		Board:   Board{Name: "Soundboard", Cols: 4, Rows: 3, Tiles: []Tile{}},
	}
}

// NewID erzeugt eine zufällige Kachel-ID.
func NewID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return hex.EncodeToString(b)
}

type Store struct {
	path string

	mu  sync.RWMutex
	cfg Config
}

// Open lädt die Konfiguration oder legt Defaults an. created ist true, wenn keine Datei existierte.
func Open(path string) (s *Store, created bool, err error) {
	s = &Store{path: path, cfg: Default()}
	b, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return s, true, nil
	case err != nil:
		return nil, false, err
	}
	if err := json.Unmarshal(b, &s.cfg); err != nil {
		// Defekte Datei sichern statt überschreiben.
		_ = os.Rename(path, path+".broken")
		s.cfg = Default()
		return s, true, fmt.Errorf("config.json defekt (gesichert als .broken): %w", err)
	}
	if s.cfg.Version > CurrentVersion {
		return nil, false, fmt.Errorf("config.json stammt aus einer neueren Version (%d)", s.cfg.Version)
	}
	s.cfg.Version = CurrentVersion
	if s.cfg.Board.Tiles == nil {
		s.cfg.Board.Tiles = []Tile{}
	}
	if err := Validate(&s.cfg); err != nil {
		return s, false, fmt.Errorf("config.json ungültig: %w", err)
	}
	return s, false, nil
}

// Get liefert eine Kopie der Konfiguration.
func (s *Store) Get() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return clone(s.cfg)
}

func clone(c Config) Config {
	c.Board.Tiles = append([]Tile(nil), c.Board.Tiles...)
	return c
}

// Update wendet fn auf eine Kopie an, validiert, speichert atomar und übernimmt die Änderung.
func (s *Store) Update(fn func(*Config) error) (Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := clone(s.cfg)
	if err := fn(&c); err != nil {
		return clone(s.cfg), err
	}
	if err := Validate(&c); err != nil {
		return clone(s.cfg), err
	}
	if err := writeAtomic(s.path, &c); err != nil {
		return clone(s.cfg), err
	}
	s.cfg = c
	return clone(c), nil
}

// Save schreibt den aktuellen Stand (z. B. nach dem Anlegen der Defaults).
func (s *Store) Save() error {
	_, err := s.Update(func(*Config) error { return nil })
	return err
}

func writeAtomic(path string, c *Config) error {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Validate korrigiert harmlose Werte (Clamping, Defaults) und lehnt Inkonsistenzen ab.
func Validate(c *Config) error {
	a := &c.Audio
	a.MicGain = clampf(a.MicGain, 0, 4)
	a.FxGainMain = clampf(a.FxGainMain, 0, 4)
	a.FxGainMonitor = clampf(a.FxGainMonitor, 0, 4)
	if a.PeriodMs < 5 || a.PeriodMs > 50 {
		a.PeriodMs = 10
	}
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return errors.New("Port muss zwischen 1 und 65535 liegen")
	}
	b := &c.Board
	if b.Cols < 1 || b.Cols > MaxGrid || b.Rows < 1 || b.Rows > MaxGrid {
		return fmt.Errorf("Raster muss zwischen 1x1 und %dx%d liegen", MaxGrid, MaxGrid)
	}
	ids := map[string]bool{}
	pos := map[[2]int]string{}
	for i := range b.Tiles {
		t := &b.Tiles[i]
		if t.ID == "" {
			t.ID = NewID()
		}
		if ids[t.ID] {
			return fmt.Errorf("Kachel-ID %q doppelt", t.ID)
		}
		ids[t.ID] = true
		if t.X < 0 || t.Y < 0 {
			return fmt.Errorf("Kachel %q: negative Position", t.ID)
		}
		if other, dup := pos[[2]int{t.X, t.Y}]; dup {
			return fmt.Errorf("Kacheln %q und %q liegen auf derselben Position (%d,%d)", other, t.ID, t.X, t.Y)
		}
		pos[[2]int{t.X, t.Y}] = t.ID
		t.Pan = clampf(t.Pan, -1, 1)
		t.Gain = clampf(t.Gain, 0, 2)
		switch t.Mode {
		case audio.ModeOneShot, audio.ModeRestart, audio.ModeToggle, audio.ModeLoop:
		case "":
			t.Mode = audio.ModeOneShot
		default:
			return fmt.Errorf("Kachel %q: unbekannter Modus %q", t.ID, t.Mode)
		}
		if t.Icon != nil && t.Icon.Kind != "emoji" && t.Icon.Kind != "file" {
			return fmt.Errorf("Kachel %q: unbekannter Icon-Typ %q", t.ID, t.Icon.Kind)
		}
	}
	return nil
}

func clampf(v, lo, hi float32) float32 {
	if v != v { // NaN
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// Tile liefert eine Kachel per ID.
func (c Config) Tile(id string) (Tile, bool) {
	for _, t := range c.Board.Tiles {
		if t.ID == id {
			return t, true
		}
	}
	return Tile{}, false
}
