package audio

import (
	"math"
	"sync"
	"testing"
	"time"
)

func constSound(frames int, v float32) *Sound {
	d := make([]float32, frames*2)
	for i := range d {
		d[i] = v
	}
	return &Sound{Name: "const", Data: d, Frames: frames}
}

func TestBalance(t *testing.T) {
	cases := []struct{ pan, l, r float32 }{{0, 1, 1}, {-1, 1, 0}, {1, 0, 1}, {0.5, 0.5, 1}, {-0.25, 1, 0.75}, {5, 0, 1}}
	for _, c := range cases {
		l, r := Balance(c.pan)
		if l != c.l || r != c.r {
			t.Errorf("Balance(%v) = %v,%v want %v,%v", c.pan, l, r, c.l, c.r)
		}
	}
}

func TestRingWriteRead(t *testing.T) {
	r := NewRing(8)
	in := make([]float32, 6*2)
	for i := range in {
		in[i] = float32(i)
	}
	if d := r.Write(in); d != 0 {
		t.Fatalf("dropped %d", d)
	}
	if r.Available() != 6 {
		t.Fatalf("avail %d", r.Available())
	}
	l, rr := r.peek(2)
	if l != 4 || rr != 5 {
		t.Fatalf("peek %v %v", l, rr)
	}
	r.skip(4)
	// Wrap-around
	if d := r.Write(make([]float32, 6*2)); d != 0 {
		t.Fatalf("dropped %d", d)
	}
	if r.Available() != 8 {
		t.Fatalf("avail %d", r.Available())
	}
	// full → the surplus is dropped
	if d := r.Write(make([]float32, 3*2)); d != 3 {
		t.Fatalf("dropped %d want 3", d)
	}
}

func TestBusMixAndLimiter(t *testing.T) {
	ended := make(chan string, 4)
	b := newBus("main", func(_, tile string) { ended <- tile })
	b.send(command{kind: cmdPlay, tile: "a", snd: constSound(100, 0.25), gL: 1, gR: 0.5})
	out := make([]float32, 64*2)
	b.Fill(out)
	if out[0] != 0.25 || out[1] != 0.125 {
		t.Fatalf("mix: %v %v", out[0], out[1])
	}
	b.Fill(out) // remaining 36 frames, then the end
	select {
	case tile := <-ended:
		if tile != "a" {
			t.Fatal(tile)
		}
	default:
		t.Fatal("no end reported")
	}
	if out[36*2] != 0 {
		t.Fatalf("silence must follow the end, got %v", out[36*2])
	}

	// Limiter: two loud sounds must not exceed the threshold.
	b.send(command{kind: cmdPlay, tile: "x", snd: constSound(1000, 0.9), gL: 1, gR: 1})
	b.send(command{kind: cmdPlay, tile: "y", snd: constSound(1000, 0.9), gL: 1, gR: 1})
	b.Fill(out)
	for i, s := range out {
		if s > limThresh+1e-4 {
			t.Fatalf("limiter exceeded at %d: %v", i, s)
		}
	}
}

func TestBusStopFade(t *testing.T) {
	b := newBus("main", nil)
	b.send(command{kind: cmdPlay, tile: "a", snd: constSound(100000, 0.5), gL: 1, gR: 1, loop: true})
	out := make([]float32, 480*2)
	b.Fill(out)
	b.send(command{kind: cmdStop, tile: "a"})
	b.Fill(out)
	if out[0] < 0.4 || out[stopFadeLen*2-2] > 0.01 {
		t.Fatalf("implausible fade: first %v, last %v", out[0], out[stopFadeLen*2-2])
	}
	if len(b.voices) != 0 {
		t.Fatal("voice should have ended")
	}
}

// Simulates two clocks with drift: capture delivers 10 ms periods at rate (1+drift), playback consumes nominally.
func TestMicReaderDriftConverges(t *testing.T) {
	for _, drift := range []float64{0, 200e-6, -200e-6, 1500e-6} {
		ring := NewRing(SampleRate / 4)
		target := 960
		m := NewMicReader(ring, target)
		const period = 480
		in := make([]float32, period*2)
		for i := range in {
			in[i] = 0.5
		}
		out := make([]float32, period*2)

		capAcc := 0.0
		var sumFill float64
		var cnt int
		seconds := 60
		for step := 0; step < seconds*100; step++ {
			// Capture clock: period*(1+drift) frames arrive per playback period
			capAcc += float64(period) * (1 + drift)
			for capAcc >= period {
				ring.Write(in)
				capAcc -= period
			}
			for i := range out {
				out[i] = 0
			}
			if step > 30*100 {
				sumFill += float64(ring.Available()) // fill level on entry, as the controller sees it
				cnt++
			}
			m.Add(out, 1)
		}
		avg := sumFill / float64(cnt)
		if math.Abs(avg-float64(target)) > 100 {
			t.Errorf("drift %+.0fppm: mean fill level %.0f is far from target %d", drift*1e6, avg, target)
		}
		if u := m.Underruns.Load(); u > 3 {
			t.Errorf("drift %+.0fppm: %d Underruns", drift*1e6, u)
		}
		if o := m.Overruns.Load(); o > 3 {
			t.Errorf("drift %+.0fppm: %d Overruns", drift*1e6, o)
		}
		t.Logf("drift %+6.0fppm: mean fill %.0f (target %d), Ratio %.6f, under %d over %d",
			drift*1e6, avg, target, m.Ratio(), m.Underruns.Load(), m.Overruns.Load())
	}
}

func TestMicReaderPassesSignal(t *testing.T) {
	ring := NewRing(4096)
	m := NewMicReader(ring, 100)
	in := make([]float32, 480*2)
	for i := range in {
		in[i] = 0.5
	}
	ring.Write(in)
	out := make([]float32, 100*2)
	m.Add(out, 0.5)
	if math.Abs(float64(out[50])-0.25) > 1e-3 {
		t.Fatalf("want 0.25, got %v", out[50])
	}
}

// --- Engine with fake backend ---

type fakeStream struct{}

func (fakeStream) Close() {}

type fakeBackend struct {
	mu       sync.Mutex
	devs     []DeviceInfo
	playFill map[string]func([]float32)
}

func (f *fakeBackend) Devices() ([]DeviceInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]DeviceInfo(nil), f.devs...), nil
}
func (f *fakeBackend) OpenCapture(id string, _ int, _ func([]float32), _ func()) (Stream, error) {
	return fakeStream{}, nil
}
func (f *fakeBackend) OpenPlayback(id string, _ int, fill func([]float32), _ func()) (Stream, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.playFill == nil {
		f.playFill = map[string]func([]float32){}
	}
	f.playFill[id] = fill
	return fakeStream{}, nil
}
func (f *fakeBackend) Close() {}

func (f *fakeBackend) pump(id string, frames int) []float32 {
	f.mu.Lock()
	fill := f.playFill[id]
	f.mu.Unlock()
	out := make([]float32, frames*2)
	fill(out)
	return out
}

func waitEvent(t *testing.T, e *Engine, typ, tile string) {
	t.Helper()
	timeout := time.After(2 * time.Second)
	for {
		select {
		case ev := <-e.Events():
			if ev.Type == typ && ev.Tile == tile {
				return
			}
		case <-timeout:
			t.Fatalf("event %s/%s not received", typ, tile)
		}
	}
}

func TestEngineRoutingAndModes(t *testing.T) {
	fb := &fakeBackend{devs: []DeviceInfo{
		{ID: "mic", Name: "Microphone", Kind: KindCapture},
		{ID: "cable", Name: "CABLE Input", Kind: KindPlayback},
		{ID: "phones", Name: "Headphones", Kind: KindPlayback},
	}}
	e := NewEngine(fb)
	defer e.Close()
	e.ApplyNow(Settings{InputID: "mic", MainID: "cable", MonitorID: "phones", MicGain: 1, FxGainMain: 1, FxGainMonitor: 0.5, PeriodMs: 10})

	st := e.Status()
	if !st.Input.Running || !st.Main.Running || !st.Monitor.Running {
		t.Fatalf("Status: %+v", st)
	}

	snd := constSound(48000, 0.4)
	e.Trigger("t1", snd, 1, 0, ModeOneShot)
	waitEvent(t, e, "started", "t1")
	main := fb.pump("cable", 100)
	mon := fb.pump("phones", 100)
	if math.Abs(float64(main[10])-0.4) > 1e-4 {
		t.Errorf("Main: %v", main[10])
	}
	if math.Abs(float64(mon[10])-0.2) > 1e-4 { // FxGainMonitor 0.5
		t.Errorf("Monitor: %v", mon[10])
	}

	// toggle mode stops on the second press
	e.Trigger("t2", snd, 1, 0, ModeToggle)
	waitEvent(t, e, "started", "t2")
	e.Trigger("t2", snd, 1, 0, ModeToggle)
	fb.pump("cable", 480)
	fb.pump("phones", 480)
	waitEvent(t, e, "stopped", "t2")

	// Loop: keeps running past the end of the sound
	e.Trigger("loop", constSound(100, 0.1), 1, 0, ModeLoop)
	waitEvent(t, e, "started", "loop")
	fb.pump("cable", 1000)
	if out := fb.pump("cable", 10); out[0] == 0 {
		t.Error("loop should keep running")
	}
	e.StopAll()
	fb.pump("cable", 480)
	fb.pump("phones", 480)
	waitEvent(t, e, "stopped", "loop")

	// Preview on the monitor only
	e.Preview(constSound(1000, 0.3), 1, 0)
	if out := fb.pump("cable", 10); out[0] != 0 {
		t.Error("preview must not play on main")
	}
	if out := fb.pump("phones", 10); out[0] == 0 {
		t.Error("preview missing on monitor")
	}
}

func TestEngineFeedbackGuard(t *testing.T) {
	fb := &fakeBackend{devs: []DeviceInfo{
		{ID: "cableout", Name: "CABLE Output (VB-Audio Virtual Cable)", Kind: KindCapture},
		{ID: "cable", Name: "CABLE Input", Kind: KindPlayback},
	}}
	e := NewEngine(fb)
	defer e.Close()
	e.ApplyNow(Settings{InputID: "cableout", MainID: "cable", PeriodMs: 10})
	st := e.Status()
	if st.Input.Running || st.Input.Error == "" || st.Input.ErrorCode != "feedback" {
		t.Fatalf("feedback protection did not trigger: %+v", st.Input)
	}
}

func TestEngineMissingDevice(t *testing.T) {
	e := NewEngine(&fakeBackend{})
	defer e.Close()
	e.ApplyNow(Settings{MainID: "weg", PeriodMs: 10})
	if st := e.Status(); st.Main.Running || st.Main.Error == "" {
		t.Fatalf("%+v", st.Main)
	}
}
