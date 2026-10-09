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
		t.Fatal("not interleaved stereo")
	}
	var peak float32
	for i := 0; i < len(snd.Data); i += 2 {
		if snd.Data[i] != snd.Data[i+1] {
			t.Fatal("mono should be mirrored to both channels")
		}
		if a := snd.Data[i]; a > peak {
			peak = a
		}
	}
	if peak < 0.2 || peak > 0.5 {
		t.Fatalf("implausible level: %v", peak)
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
	name, err := lib.Save("../evil/My Sound?.wav", f)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(name, `/\?`) {
		t.Fatalf("unsafe name: %q", name)
	}
	f = read()
	name2, _ := lib.Save("../evil/My Sound?.wav", f)
	f.Close()
	if name2 == name {
		t.Fatal("name collision not resolved")
	}
	if lib.Get(name) == nil || len(lib.List()) != 2 {
		t.Fatalf("List: %+v", lib.List())
	}
	if _, err := lib.Save("virus.exe", strings.NewReader("x")); err == nil {
		t.Fatal("exe must not be allowed")
	}
	if err := lib.Delete(name); err != nil {
		t.Fatal(err)
	}
	if lib.Get(name) != nil {
		t.Fatal("still present after delete")
	}
	if err := lib.Delete("../x.wav"); err == nil {
		t.Fatal("path traversal not rejected")
	}
}

func TestLibraryBrokenFileReportsError(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "broken.mp3"), []byte("this is not an mp3"), 0o644)
	lib, _ := New(dir, nil)
	lib.Scan()
	l := lib.List()
	if len(l) != 1 || l[0].Error == "" || lib.Get("broken.mp3") != nil {
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
		t.Fatal("watcher did not detect the new file")
	}
	if lib.Get("neu.wav") == nil {
		t.Fatal("not loaded")
	}
	os.Remove(filepath.Join(dir, "neu.wav"))
	select {
	case <-changes:
	case <-time.After(3 * time.Second):
		t.Fatal("deletion not detected")
	}
	if lib.Get("neu.wav") != nil {
		t.Fatal("still present")
	}
	_ = audio.SampleRate
}
