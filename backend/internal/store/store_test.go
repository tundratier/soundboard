package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreRoundTripAndValidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	s, created, err := Open(path)
	if err != nil || !created {
		t.Fatal(err, created)
	}
	_, err = s.Update(func(c *Config) error {
		c.Board.Tiles = []Tile{{X: 0, Y: 0, Sound: "a.wav", Pan: 7, Gain: -1}, {X: 1, Y: 0, Sound: "b.wav", Mode: "loop"}}
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
	if len(c.Board.Tiles) != 2 || c.Board.Tiles[0].ID == "" {
		t.Fatalf("%+v", c.Board)
	}
	if c.Board.Tiles[0].Pan != 1 || c.Board.Tiles[0].Gain != 0 || c.Board.Tiles[0].Mode != "oneshot" {
		t.Fatalf("Clamping/Default fehlt: %+v", c.Board.Tiles[0])
	}

	// duplicate position is rejected, previous state is kept
	_, err = s2.Update(func(c *Config) error {
		c.Board.Tiles[1].X = 0
		return nil
	})
	if err == nil {
		t.Fatal("doppelte Position akzeptiert")
	}
	if s2.Get().Board.Tiles[1].X != 1 {
		t.Fatal("fehlgeschlagenes Update hat den Stand verändert")
	}
	_, err = s2.Update(func(c *Config) error { c.Board.Cols = 99; return nil })
	if err == nil {
		t.Fatal("Raster 99 akzeptiert")
	}
}

func TestStoreBrokenFileIsBackedUp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	os.WriteFile(path, []byte("{kaputt"), 0o644)
	s, created, err := Open(path)
	if err == nil || !created || s == nil {
		t.Fatalf("err=%v created=%v", err, created)
	}
	if _, err := os.Stat(path + ".broken"); err != nil {
		t.Fatal("defekte Datei nicht gesichert")
	}
}
