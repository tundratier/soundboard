package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreRoundTripAndValidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	s, created, err := Open(path)
	if err != nil || !created {
		t.Fatal(err, created)
	}
	_, err = s.Update(func(c *Config) error {
		c.Boards[0].Tiles = []Tile{{X: 0, Y: 0, Sound: "a.wav", Pan: 7, Gain: -1}, {X: 1, Y: 0, Sound: "b.wav", Mode: "loop"}}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	s2, created, err := Open(path)
	if err != nil || created {
		t.Fatal(err, created)
	}
	c := s2.Get()
	b := c.Boards[0]
	if len(b.Tiles) != 2 || b.Tiles[0].ID == "" {
		t.Fatalf("%+v", b)
	}
	if b.Tiles[0].Pan != 1 || b.Tiles[0].Gain != 0 || b.Tiles[0].Mode != "oneshot" {
		t.Fatalf("clamping/default missing: %+v", b.Tiles[0])
	}

	// duplicate position is rejected, previous state is kept
	_, err = s2.Update(func(c *Config) error {
		c.Boards[0].Tiles[1].X = 0
		return nil
	})
	if err == nil {
		t.Fatal("duplicate position accepted")
	}
	if s2.Get().Boards[0].Tiles[1].X != 1 {
		t.Fatal("failed update changed the state")
	}
	_, err = s2.Update(func(c *Config) error { c.Boards[0].Cols = 99; return nil })
	if err == nil {
		t.Fatal("grid 99 accepted")
	}
}

func TestStoreBrokenFileIsBackedUp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	os.WriteFile(path, []byte("{broken"), 0o644)
	s, created, err := Open(path)
	if err == nil || !created || s == nil {
		t.Fatalf("err=%v created=%v", err, created)
	}
	if _, err := os.Stat(path + ".broken"); err != nil {
		t.Fatal("corrupt file was not backed up")
	}
}

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Soundboard":            "soundboard",
		"Zum Lachen!":           "zum-lachen",
		"  Größe & Maße  ":      "groesse-masse",
		"---":                   "board",
		"":                      "board",
		"Ärger 2024":            "aerger-2024",
		strings.Repeat("a", 60): strings.Repeat("a", 40),
	}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
		if !slugRe.MatchString(Slugify(in)) {
			t.Errorf("Slugify(%q) = %q is not a valid ID", in, Slugify(in))
		}
	}
	taken := map[string]bool{"x": true, "x-2": true}
	if got := UniqueSlug("x", func(s string) bool { return taken[s] }); got != "x-3" {
		t.Errorf("UniqueSlug = %q", got)
	}
	long := strings.Repeat("a", 40)
	if got := UniqueSlug(long, func(s string) bool { return s == long }); len(got) > 40 || !slugRe.MatchString(got) {
		t.Errorf("UniqueSlug(long) = %q", got)
	}
}

func TestMultipleBoards(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	s, _, _ := Open(path)

	// new boards get an ID derived from the name, unique even for equal names
	c, err := s.Update(func(c *Config) error {
		c.Boards = append(c.Boards,
			Board{Name: "Funny", Cols: 3, Rows: 3, Tiles: []Tile{{ID: "t1", Sound: "a.wav"}}},
			Board{Name: "Funny", Cols: 2, Rows: 2},
		)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := []string{c.Boards[1].ID, c.Boards[2].ID}; got[0] != "funny" || got[1] != "funny-2" {
		t.Fatalf("IDs: %v", got)
	}
	if b, ok := c.Board("funny"); !ok || b.Name != "Funny" {
		t.Fatal("Board() finds nothing")
	}
	if tile, ok := c.Tile("t1"); !ok || tile.Sound != "a.wav" {
		t.Fatal("Tile() does not search all boards")
	}

	bad := map[string]func(*Config){
		"duplicate board ID":    func(c *Config) { c.Boards[2].ID = "funny" },
		"invalid board ID":      func(c *Config) { c.Boards[2].ID = "Invalid/Path" },
		"no boards":             func(c *Config) { c.Boards = nil },
		"duplicate tile ID":     func(c *Config) { c.Boards[2].Tiles = []Tile{{ID: "t1"}} },
		"empty grid in board 2": func(c *Config) { c.Boards[2].Rows = 0 },
	}
	for name, mutate := range bad {
		if _, err := s.Update(func(c *Config) error { mutate(c); return nil }); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if len(s.Get().Boards) != 3 {
		t.Fatal("rejected updates changed the state")
	}

	// the same tile position on different boards is fine
	if _, err := s.Update(func(c *Config) error {
		c.Boards[0].Tiles = []Tile{{ID: "p0", X: 0, Y: 0}}
		c.Boards[2].Tiles = []Tile{{ID: "p2", X: 0, Y: 0}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestMigrationFromV1(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	v1 := `{
  "version": 1,
  "audio": {"inputDeviceId": "mic", "mainDeviceId": "main", "monitorDeviceId": "mon", "micGain": 1, "fxGainMain": 1, "fxGainMonitor": 1, "periodMs": 10},
  "server": {"port": 9000, "bindLan": false},
  "board": {"name": "My Board", "cols": 5, "rows": 2, "tiles": [{"id": "abc", "x": 1, "y": 1, "sound": "x.wav", "pan": 0, "gain": 1, "mode": "toggle"}]}
}`
	if err := os.WriteFile(path, []byte(v1), 0o644); err != nil {
		t.Fatal(err)
	}
	s, created, err := Open(path)
	if err != nil || created {
		t.Fatal(err, created)
	}
	c := s.Get()
	if c.Version != CurrentVersion || len(c.Boards) != 1 {
		t.Fatalf("%+v", c)
	}
	b := c.Boards[0]
	if b.ID != "my-board" || b.Name != "My Board" || b.Cols != 5 || len(b.Tiles) != 1 || b.Tiles[0].ID != "abc" || b.Tiles[0].Mode != "toggle" {
		t.Fatalf("board not migrated: %+v", b)
	}
	if c.Audio.InputID != "mic" || c.Server.Port != 9000 || c.Server.BindLAN {
		t.Fatalf("other settings lost: %+v %+v", c.Audio, c.Server)
	}

	// old file is kept, migrated file is written and stable on reopen
	if bak, err := os.ReadFile(path + ".v1.bak"); err != nil || string(bak) != v1 {
		t.Fatalf("backup missing or changed: %v", err)
	}
	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), `"board":`) || !strings.Contains(string(raw), `"boards":`) {
		t.Fatalf("migrated file still in the old layout:\n%s", raw)
	}
	s2, _, err := Open(path)
	if err != nil || s2.Get().Boards[0].ID != "my-board" {
		t.Fatalf("reopen: %v", err)
	}
}

func TestLanguageSetting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	s, _, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Get().Language; got != "" {
		t.Fatalf("default language = %q, want automatic", got)
	}
	for in, want := range map[string]string{"de": "de", "EN": "en", "de-DE": "de", "auto": "", "klingon": ""} {
		c, err := s.Update(func(c *Config) error { c.Language = in; return nil })
		if err != nil || c.Language != want {
			t.Errorf("Language %q → %q, %v; want %q", in, c.Language, err, want)
		}
	}
	// persisted across a restart
	if _, err := s.Update(func(c *Config) error { c.Language = "de"; return nil }); err != nil {
		t.Fatal(err)
	}
	s2, _, err := Open(path)
	if err != nil || s2.Get().Language != "de" {
		t.Fatalf("reopened: %q, %v", s2.Get().Language, err)
	}
}
