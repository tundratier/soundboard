// Package store holds the configuration (config.json) and validates changes.
package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"soundboard/internal/audio"
)

const (
	CurrentVersion = 2 // 1: single "board"; 2: "boards" list
	MaxGrid        = 12
	MaxBoards      = 50
)

// slugRe is the format of a board ID, which doubles as the board's URL path (/boards/<id>).
var slugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,39}$`)

type Icon struct {
	Kind  string `json:"kind"`  // "emoji" | "file"
	Value string `json:"value"` // emoji or file name in the icons/ folder
}

type Tile struct {
	ID    string         `json:"id"`
	X     int            `json:"x"`
	Y     int            `json:"y"`
	Label string         `json:"label,omitempty"`
	Sound string         `json:"sound"` // file name in the sounds/ folder
	Icon  *Icon          `json:"icon,omitempty"`
	Pan   float32        `json:"pan"`  // Balance -1 … +1
	Gain  float32        `json:"gain"` // 0 … 2
	Mode  audio.PlayMode `json:"mode"`
	Color string         `json:"color,omitempty"`
}

type Board struct {
	ID    string `json:"id"` // URL-safe slug, unique; the board is served at /boards/<id>
	Name  string `json:"name"`
	Cols  int    `json:"cols"`
	Rows  int    `json:"rows"`
	Tiles []Tile `json:"tiles"`
}

type Server struct {
	Port    int  `json:"port"`
	BindLAN bool `json:"bindLan"` // true: reachable on the whole network (board), false: localhost only
}

type Config struct {
	Version int            `json:"version"`
	Audio   audio.Settings `json:"audio"`
	Server  Server         `json:"server"`
	Boards  []Board        `json:"boards"`
}

func Default() Config {
	return Config{
		Version: CurrentVersion,
		Audio:   audio.Settings{MicGain: 1, FxGainMain: 1, FxGainMonitor: 1, PeriodMs: 10},
		Server:  Server{Port: 8723, BindLAN: true},
		Boards:  []Board{{ID: "main", Name: "Soundboard", Cols: 4, Rows: 3, Tiles: []Tile{}}},
	}
}

// Slugify turns a board name into a URL-safe ID ("Zum Lachen!" → "zum-lachen").
func Slugify(name string) string {
	r := strings.NewReplacer("ä", "ae", "ö", "oe", "ü", "ue", "ß", "ss", "Ä", "ae", "Ö", "oe", "Ü", "ue")
	var b strings.Builder
	dash := true // suppresses leading dashes and collapses runs
	for _, c := range strings.ToLower(r.Replace(name)) {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			b.WriteRune(c)
			dash = false
		} else if !dash {
			b.WriteByte('-')
			dash = true
		}
	}
	s := strings.Trim(b.String(), "-")
	if len(s) > 40 {
		s = strings.Trim(s[:40], "-")
	}
	if s == "" {
		s = "board"
	}
	return s
}

// UniqueSlug returns base, or base-2, base-3 … until taken(candidate) is false.
func UniqueSlug(base string, taken func(string) bool) string {
	if !taken(base) {
		return base
	}
	for i := 2; ; i++ {
		suffix := fmt.Sprintf("-%d", i)
		cand := base
		if len(cand)+len(suffix) > 40 {
			cand = strings.Trim(cand[:40-len(suffix)], "-")
		}
		if cand += suffix; !taken(cand) {
			return cand
		}
	}
}

// NewID generates a random tile ID.
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

// Open loads the configuration or creates defaults. created is true if no file existed.
func Open(path string) (s *Store, created bool, err error) {
	s = &Store{path: path, cfg: Default()}
	b, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return s, true, nil
	case err != nil:
		return nil, false, err
	}
	// Version 1 stored a single "board"; decode it alongside the current layout.
	file := struct {
		Config
		LegacyBoard *Board `json:"board"`
	}{Config: Default()}
	file.Boards = nil
	if err := json.Unmarshal(b, &file); err != nil {
		// Back up a broken file instead of overwriting it.
		_ = os.Rename(path, path+".broken")
		s.cfg = Default()
		return s, true, fmt.Errorf("config.json defekt (gesichert als .broken): %w", err)
	}
	if file.Version > CurrentVersion {
		return nil, false, fmt.Errorf("config.json stammt aus einer neueren Version (%d)", file.Version)
	}
	s.cfg = file.Config
	migrated := file.Version < CurrentVersion
	if migrated && len(s.cfg.Boards) == 0 && file.LegacyBoard != nil {
		legacy := *file.LegacyBoard
		legacy.ID = Slugify(legacy.Name)
		s.cfg.Boards = []Board{legacy}
	}
	if len(s.cfg.Boards) == 0 {
		s.cfg.Boards = Default().Boards
	}
	s.cfg.Version = CurrentVersion
	if err := Validate(&s.cfg); err != nil {
		return s, false, fmt.Errorf("config.json ungültig: %w", err)
	}
	if migrated {
		// Keep the old file next to the migrated one.
		if err := os.WriteFile(path+".v1.bak", b, 0o644); err != nil {
			return s, false, fmt.Errorf("Sicherung der alten config.json: %w", err)
		}
		if err := writeAtomic(path, &s.cfg); err != nil {
			return s, false, fmt.Errorf("Migration speichern: %w", err)
		}
	}
	return s, false, nil
}

// Get returns a copy of the configuration.
func (s *Store) Get() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return clone(s.cfg)
}

func clone(c Config) Config {
	boards := make([]Board, len(c.Boards))
	for i, b := range c.Boards {
		b.Tiles = append([]Tile{}, b.Tiles...)
		boards[i] = b
	}
	c.Boards = boards
	return c
}

// Update applies fn to a copy, validates, saves atomically and adopts the change.
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

// Save writes the current state (e.g. after creating the defaults).
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

// Validate fixes harmless values (clamping, defaults) and rejects inconsistencies.
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
	if len(c.Boards) == 0 {
		return errors.New("Es muss mindestens ein Board geben")
	}
	if len(c.Boards) > MaxBoards {
		return fmt.Errorf("Höchstens %d Boards", MaxBoards)
	}
	boardIDs := map[string]bool{}
	tileIDs := map[string]bool{} // tile IDs are global: triggering and playback state are keyed by tile ID alone
	// Existing IDs are claimed first so that generated ones for new boards cannot collide with them.
	for _, b := range c.Boards {
		if b.ID != "" {
			if !slugRe.MatchString(b.ID) {
				return fmt.Errorf("Board-Pfad %q ungültig (erlaubt: a-z, 0-9, Bindestrich; max. 40 Zeichen)", b.ID)
			}
			if boardIDs[b.ID] {
				return fmt.Errorf("Board-Pfad %q doppelt", b.ID)
			}
			boardIDs[b.ID] = true
		}
	}
	for bi := range c.Boards {
		b := &c.Boards[bi]
		b.Name = strings.TrimSpace(b.Name)
		if b.ID == "" {
			name := b.Name
			if name == "" {
				name = "board"
			}
			b.ID = UniqueSlug(Slugify(name), func(s string) bool { return boardIDs[s] })
			boardIDs[b.ID] = true
		}
		if b.Name == "" {
			b.Name = b.ID
		}
		if b.Cols < 1 || b.Cols > MaxGrid || b.Rows < 1 || b.Rows > MaxGrid {
			return fmt.Errorf("Board %q: Raster muss zwischen 1x1 und %dx%d liegen", b.ID, MaxGrid, MaxGrid)
		}
		if b.Tiles == nil {
			b.Tiles = []Tile{}
		}
		pos := map[[2]int]string{}
		for i := range b.Tiles {
			t := &b.Tiles[i]
			if t.ID == "" {
				t.ID = NewID()
			}
			if tileIDs[t.ID] {
				return fmt.Errorf("Kachel-ID %q doppelt", t.ID)
			}
			tileIDs[t.ID] = true
			if t.X < 0 || t.Y < 0 {
				return fmt.Errorf("Kachel %q: negative Position", t.ID)
			}
			if other, dup := pos[[2]int{t.X, t.Y}]; dup {
				return fmt.Errorf("Board %q: Kacheln %q und %q liegen auf derselben Position (%d,%d)", b.ID, other, t.ID, t.X, t.Y)
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

// Tile returns a tile by ID (tile IDs are unique across all boards).
func (c Config) Tile(id string) (Tile, bool) {
	for _, b := range c.Boards {
		for _, t := range b.Tiles {
			if t.ID == id {
				return t, true
			}
		}
	}
	return Tile{}, false
}

// Board returns a board by ID.
func (c Config) Board(id string) (Board, bool) {
	for _, b := range c.Boards {
		if b.ID == id {
			return b, true
		}
	}
	return Board{}, false
}
