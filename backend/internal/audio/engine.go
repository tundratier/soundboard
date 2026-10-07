package audio

import (
	"log"
	"strings"
	"sync"
	"time"
)

// Settings are the audio settings the engine applies.
type Settings struct {
	InputID       string  `json:"inputDeviceId"`   // microphone
	MainID        string  `json:"mainDeviceId"`    // mix of microphone + effects (e.g. CABLE Input)
	MonitorID     string  `json:"monitorDeviceId"` // effects only, e.g. headphones
	MicGain       float32 `json:"micGain"`
	FxGainMain    float32 `json:"fxGainMain"`
	FxGainMonitor float32 `json:"fxGainMonitor"`
	PeriodMs      int     `json:"periodMs"`
}

// Event reports a tile's state change ("started" / "stopped").
type Event struct {
	Type string `json:"type"`
	Tile string `json:"tile"`
}

// RoleStatus describes the state of one of the three device roles.
type RoleStatus struct {
	DeviceID string `json:"deviceId"`
	Running  bool   `json:"running"`
	Error    string `json:"error,omitempty"`
}

// Status is the engine's live state (for UI/diagnostics).
type Status struct {
	Input        RoleStatus `json:"input"`
	Main         RoleStatus `json:"main"`
	Monitor      RoleStatus `json:"monitor"`
	MainPeak     float32    `json:"mainPeak"`
	MicPeak      float32    `json:"micPeak"`
	MonitorPeak  float32    `json:"monitorPeak"`
	MicUnderruns uint64     `json:"micUnderruns"`
	MicOverruns  uint64     `json:"micOverruns"`
	MicRatio     float64    `json:"micRatio"`
}

const previewTile = "__preview__"

type role struct {
	name    string
	wantID  string
	stream  Stream
	openID  string // ID the running stream was opened with
	openPer int
	err     string
	retryAt time.Time
}

type endEv struct{ bus, tile string }

// Engine manages devices, buses and playback state.
type Engine struct {
	backend Backend

	mu      sync.Mutex
	set     Settings
	input   role
	main    role
	monitor role
	names   map[string]string // device ID → name (last seen)
	playing map[string]map[string]struct{}

	mainBus, monBus *Bus
	ended           chan endEv
	events          chan Event
	quit            chan struct{}
	wake            chan struct{}
	wg              sync.WaitGroup
}

func NewEngine(b Backend) *Engine {
	e := &Engine{
		backend: b,
		names:   map[string]string{},
		playing: map[string]map[string]struct{}{},
		ended:   make(chan endEv, 256),
		events:  make(chan Event, 128),
		quit:    make(chan struct{}),
		wake:    make(chan struct{}, 1),
		set:     Settings{MicGain: 1, FxGainMain: 1, FxGainMonitor: 1, PeriodMs: 10},
	}
	e.input.name, e.main.name, e.monitor.name = "input", "main", "monitor"
	onEnd := func(bus, tile string) {
		select {
		case e.ended <- endEv{bus, tile}:
		default:
		}
	}
	e.mainBus = newBus("main", onEnd)
	e.monBus = newBus("monitor", onEnd)
	e.wg.Add(2)
	go e.endLoop()
	go e.supervise()
	return e
}

// Events delivers start/stop events. The channel is buffered; events are dropped under backpressure.
func (e *Engine) Events() <-chan Event { return e.events }

// Apply takes over new settings; devices are (re)opened asynchronously.
func (e *Engine) Apply(s Settings) {
	e.applySettings(s)
	select {
	case e.wake <- struct{}{}:
	default:
	}
}

// ApplyNow is like Apply but waits until the devices have been opened (or failed).
func (e *Engine) ApplyNow(s Settings) {
	e.applySettings(s)
	e.reconcile(true)
}

func (e *Engine) applySettings(s Settings) {
	if s.PeriodMs < 5 {
		s.PeriodMs = 10
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.set = s
	e.input.wantID, e.main.wantID, e.monitor.wantID = s.InputID, s.MainID, s.MonitorID
	e.mainBus.SetMicGain(s.MicGain)
	e.mainBus.SetFxGain(s.FxGainMain)
	e.monBus.SetFxGain(s.FxGainMonitor)
}

func (e *Engine) supervise() {
	defer e.wg.Done()
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-e.quit:
			return
		case <-e.wake:
			e.reconcile(true)
		case <-t.C:
			e.reconcile(false)
		}
	}
}

// reconcile brings the running streams in line with the desired state. force ignores the retry pause.
func (e *Engine) reconcile(force bool) {
	// Fetch the device list outside the lock (can take a while).
	devs, derr := e.backend.Devices()

	e.mu.Lock()
	defer e.mu.Unlock()
	if derr == nil {
		e.names = make(map[string]string, len(devs))
		for _, d := range devs {
			e.names[d.ID] = d.Name
		}
	}
	per := e.set.PeriodMs

	e.reconcilePlayback(&e.main, e.mainBus, per, force)
	e.reconcilePlayback(&e.monitor, e.monBus, per, force)
	e.reconcileInput(per, force)
}

// due checks whether a role should be (re)opened and sets error texts where needed.
func (e *Engine) due(r *role, force bool) (string, bool) {
	if r.wantID == "" {
		r.err = ""
		return "", false
	}
	if !force && time.Now().Before(r.retryAt) {
		return "", false
	}
	name, ok := e.names[r.wantID]
	if !ok {
		r.err = "Gerät nicht gefunden (abgesteckt?)"
		r.retryAt = time.Now().Add(2 * time.Second)
		return "", false
	}
	return name, true
}

func (e *Engine) reconcilePlayback(r *role, bus *Bus, per int, force bool) {
	if r.stream != nil && (r.openID != r.wantID || r.openPer != per) {
		e.closePlayback(r, bus)
	}
	if r.stream != nil {
		return
	}
	name, ok := e.due(r, force)
	if !ok {
		return
	}
	s, err := e.backend.OpenPlayback(r.wantID, per, bus.Fill, func() { e.streamStopped(r) })
	if err != nil {
		r.err = err.Error()
		r.retryAt = time.Now().Add(5 * time.Second)
		log.Printf("audio: %s öffnen fehlgeschlagen: %v", r.name, err)
		return
	}
	r.stream, r.openID, r.openPer, r.err = s, r.wantID, per, ""
	log.Printf("audio: %s läuft auf %q", r.name, name)
}

func (e *Engine) reconcileInput(per int, force bool) {
	r := &e.input
	if r.stream != nil && (r.openID != r.wantID || r.openPer != per) {
		e.closeInput()
	}
	if r.stream != nil {
		return
	}
	name, ok := e.due(r, force)
	if !ok {
		return
	}
	// Feedback protection: the output end of the virtual cable must never serve as microphone,
	// otherwise the other party hears themselves.
	if strings.Contains(strings.ToLower(name), "cable output") {
		r.err = "Rückkopplung: \"CABLE Output\" ist das Ausgabe-Ende dieser App und kann nicht als Mikrofon dienen"
		r.retryAt = time.Now().Add(5 * time.Second)
		return
	}
	ring := NewRing(SampleRate / 4) // 250 ms reserve
	reader := NewMicReader(ring, per*SampleRate/1000*2)
	s, err := e.backend.OpenCapture(r.wantID, per, func(in []float32) { ring.Write(in) }, func() { e.streamStopped(r) })
	if err != nil {
		r.err = err.Error()
		r.retryAt = time.Now().Add(5 * time.Second)
		log.Printf("audio: input öffnen fehlgeschlagen: %v", err)
		return
	}
	e.mainBus.mic.Store(reader)
	r.stream, r.openID, r.openPer, r.err = s, r.wantID, per, ""
	log.Printf("audio: input läuft auf %q", name)
}

func (e *Engine) closeInput() {
	e.mainBus.mic.Store(nil)
	if s := e.input.stream; s != nil {
		e.input.stream = nil
		s.Close()
	}
}

// closePlayback closes a playback stream and cleans up its playing state. e.mu must be held.
func (e *Engine) closePlayback(r *role, bus *Bus) {
	if s := r.stream; s != nil {
		r.stream = nil
		s.Close()
	}
	bus.send(command{kind: cmdStopAll})
	for tile, set := range e.playing {
		delete(set, bus.name)
		if len(set) == 0 {
			delete(e.playing, tile)
			e.emit(Event{"stopped", tile})
		}
	}
}

// streamStopped is called by the backend when a device stops unexpectedly (e.g. unplugged).
// Runs on a backend thread → in its own goroutine so Close() does not hang inside its own callback.
func (e *Engine) streamStopped(r *role) {
	go func() {
		e.mu.Lock()
		defer e.mu.Unlock()
		switch r {
		case &e.input:
			e.closeInput()
		case &e.main:
			e.closePlayback(r, e.mainBus)
		case &e.monitor:
			e.closePlayback(r, e.monBus)
		}
		r.err = "Gerät gestoppt, versuche erneut zu verbinden"
		r.retryAt = time.Now().Add(2 * time.Second)
	}()
}

func (e *Engine) emit(ev Event) {
	select {
	case e.events <- ev:
	default:
	}
}

func (e *Engine) endLoop() {
	defer e.wg.Done()
	for {
		select {
		case <-e.quit:
			return
		case ev := <-e.ended:
			e.mu.Lock()
			if set, ok := e.playing[ev.tile]; ok {
				delete(set, ev.bus)
				if len(set) == 0 {
					delete(e.playing, ev.tile)
					e.emit(Event{"stopped", ev.tile})
				}
			}
			e.mu.Unlock()
		}
	}
}

// activeBuses returns the buses whose device is currently running. e.mu must be held.
func (e *Engine) activeBuses(monitorOnly bool) []*Bus {
	var out []*Bus
	if !monitorOnly && e.main.stream != nil {
		out = append(out, e.mainBus)
	}
	if e.monitor.stream != nil {
		out = append(out, e.monBus)
	}
	return out
}

// Trigger triggers a tile according to its mode.
func (e *Engine) Trigger(tile string, snd *Sound, gain, pan float32, mode PlayMode) {
	if snd == nil || snd.Frames == 0 {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	_, isPlaying := e.playing[tile]
	switch mode {
	case ModeToggle, ModeLoop:
		if isPlaying {
			e.stopLocked(tile)
			return
		}
	case ModeRestart:
		if isPlaying {
			e.stopLocked(tile)
		}
	}
	e.playLocked(tile, snd, gain, pan, mode == ModeLoop, false)
}

// Preview plays a sound on the monitor only (audition in the admin).
func (e *Engine) Preview(snd *Sound, gain, pan float32) {
	if snd == nil || snd.Frames == 0 {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.stopLocked(previewTile)
	e.playLocked(previewTile, snd, gain, pan, false, true)
}

func (e *Engine) playLocked(tile string, snd *Sound, gain, pan float32, loop, monitorOnly bool) {
	gL, gR := Balance(pan)
	gL, gR = gL*gain, gR*gain
	set := e.playing[tile]
	started := false
	for _, b := range e.activeBuses(monitorOnly) {
		if !b.send(command{kind: cmdPlay, tile: tile, snd: snd, gL: gL, gR: gR, loop: loop}) {
			continue
		}
		if set == nil {
			set = map[string]struct{}{}
		}
		set[b.name] = struct{}{}
		started = true
	}
	if started {
		if _, existed := e.playing[tile]; !existed {
			e.playing[tile] = set
			e.emit(Event{"started", tile})
		}
	}
}

func (e *Engine) stopLocked(tile string) {
	for _, b := range []*Bus{e.mainBus, e.monBus} {
		b.send(command{kind: cmdStop, tile: tile})
	}
}

func (e *Engine) Stop(tile string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.stopLocked(tile)
}

// StopPreview ends the preview.
func (e *Engine) StopPreview() { e.Stop(previewTile) }

func (e *Engine) StopAll() {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, b := range []*Bus{e.mainBus, e.monBus} {
		b.send(command{kind: cmdStopAll})
	}
}

// Playing returns the IDs of currently playing tiles.
func (e *Engine) Playing() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]string, 0, len(e.playing))
	for t := range e.playing {
		if t != previewTile {
			out = append(out, t)
		}
	}
	return out
}

func (e *Engine) Devices() ([]DeviceInfo, error) { return e.backend.Devices() }

// Status returns the live state.
func (e *Engine) Status() Status {
	e.mu.Lock()
	defer e.mu.Unlock()
	st := Status{
		Input:   RoleStatus{e.input.wantID, e.input.stream != nil, e.input.err},
		Main:    RoleStatus{e.main.wantID, e.main.stream != nil, e.main.err},
		Monitor: RoleStatus{e.monitor.wantID, e.monitor.stream != nil, e.monitor.err},
	}
	st.MainPeak, st.MicPeak = e.mainBus.Peaks()
	st.MonitorPeak, _ = e.monBus.Peaks()
	if m := e.mainBus.mic.Load(); m != nil {
		st.MicUnderruns, st.MicOverruns, st.MicRatio = m.Underruns.Load(), m.Overruns.Load(), m.Ratio()
	}
	return st
}

func (e *Engine) Close() {
	close(e.quit)
	e.wg.Wait()
	e.mu.Lock()
	e.closeInput()
	e.closePlayback(&e.main, e.mainBus)
	e.closePlayback(&e.monitor, e.monBus)
	e.mu.Unlock()
	e.backend.Close()
}
