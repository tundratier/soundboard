package library

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"soundboard/internal/audio"
)

func TestDecodeResamplesToStereo48k(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "tone.wav")
	if err := WriteTestTone(p); err != nil {
		t.Fatal(err)
	}
	snd, err := Decode(p)
	if err != nil {
		t.Fatal(err)
	}
	// 0.4 s at 48 kHz ≈ 19200 frames
	if snd.Frames < 19000 || snd.Frames > 19400 {
		t.Fatalf("Frames = %d", snd.Frames)
	}
	if len(snd.Data) != snd.Frames*2 {
		t.Fatal("kein Stereo-Interleave")
	}
	var peak float32
	for i := 0; i < len(snd.Data); i += 2 {
		if snd.Data[i] != snd.Data[i+1] {
			t.Fatal("Mono sollte auf beide Kanäle gespiegelt sein")
		}
		if a := snd.Data[i]; a > peak {
			peak = a
		}
	}
	if peak < 0.2 || peak > 0.5 {
		t.Fatalf("Pegel unplausibel: %v", peak)
	}
}

func TestLibrarySaveListDelete(t *testing.T) {
	dir := t.TempDir()
	changes := make(chan struct{}, 16)
	lib, err := New(dir, func() { changes <- struct{}{} })
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(t.TempDir(), "x.wav")
	if err := WriteTestTone(src); err != nil {
		t.Fatal(err)
	}
	read := func() *os.File {
		f, err := os.Open(src)
		if err != nil {
			t.Fatal(err)
		}
		return f
	}

	f := read()
	name, err := lib.Save("../evil/Mein Sound?.wav", f)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(name, `/\?`) {
		t.Fatalf("unsicherer Name: %q", name)
	}
	f = read()
	name2, _ := lib.Save("../evil/Mein Sound?.wav", f)
	f.Close()
	if name2 == name {
		t.Fatal("Kollision nicht aufgelöst")
	}
	if lib.Get(name) == nil || len(lib.List()) != 2 {
		t.Fatalf("List: %+v", lib.List())
	}
	if _, err := lib.Save("virus.exe", strings.NewReader("x")); err == nil {
		t.Fatal("exe darf nicht erlaubt sein")
	}
	if err := lib.Delete(name); err != nil {
		t.Fatal(err)
	}
	if lib.Get(name) != nil {
		t.Fatal("nach Delete noch vorhanden")
	}
	if err := lib.Delete("../x.wav"); err == nil {
		t.Fatal("Pfad-Traversal nicht abgewehrt")
	}
}

func TestLibraryBrokenFileReportsError(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "kaputt.mp3"), []byte("das ist kein mp3"), 0o644)
	lib, _ := New(dir, nil)
	lib.Scan()
	l := lib.List()
	if len(l) != 1 || l[0].Error == "" || lib.Get("kaputt.mp3") != nil {
		t.Fatalf("%+v", l)
	}
}

func TestLibraryWatch(t *testing.T) {
	dir := t.TempDir()
	changes := make(chan struct{}, 16)
	lib, _ := New(dir, func() { changes <- struct{}{} })
	stop := make(chan struct{})
	defer close(stop)
	if err := lib.Watch(stop); err != nil {
		t.Fatal(err)
	}
	if err := WriteTestTone(filepath.Join(dir, "neu.wav")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-changes:
	case <-time.After(3 * time.Second):
		t.Fatal("Watcher hat neue Datei nicht erkannt")
	}
	if lib.Get("neu.wav") == nil {
		t.Fatal("nicht geladen")
	}
	os.Remove(filepath.Join(dir, "neu.wav"))
	select {
	case <-changes:
	case <-time.After(3 * time.Second):
		t.Fatal("Löschen nicht erkannt")
	}
	if lib.Get("neu.wav") != nil {
		t.Fatal("noch vorhanden")
	}
	_ = audio.SampleRate
}
