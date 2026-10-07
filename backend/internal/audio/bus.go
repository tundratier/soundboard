package audio

import (
	"math"
	"sync/atomic"
)

const (
	maxVoices   = 64
	stopFadeLen = 240 // 5 ms at 48 kHz: prevents clicks when aborting a sound
	limThresh   = 0.95
)

type cmdKind uint8

const (
	cmdPlay cmdKind = iota
	cmdStop
	cmdStopAll
)

type command struct {
	kind     cmdKind
	tile     string
	snd      *Sound
	gL, gR   float32
	loop     bool
	instance uint64
}

type voice struct {
	tile     string
	snd      *Sound
	pos      int
	gL, gR   float32
	loop     bool
	fadeLeft int // >0: voice is fading out
}

// Bus mixes voices (and optionally the microphone) for exactly one output device.
// Each bus has its own voice cursors because every device has its own clock.
// All state except the atomics belongs to the audio thread; control goes through cmds.
type Bus struct {
	name string
	cmds chan command

	voices []voice

	mic     atomic.Pointer[MicReader] // main bus only; replaced when the microphone is (re)opened
	micGain atomic.Uint32
	fxGain  atomic.Uint32

	limGain float32
	peak    atomic.Uint32
	micPeak atomic.Uint32

	// onEnd is called from the audio thread when the last instance of a tile ends.
	onEnd func(bus, tile string)
}

func newBus(name string, onEnd func(bus, tile string)) *Bus {
	b := &Bus{
		name:    name,
		cmds:    make(chan command, 256),
		voices:  make([]voice, 0, maxVoices),
		limGain: 1,
		onEnd:   onEnd,
	}
	b.SetFxGain(1)
	b.SetMicGain(1)
	return b
}

func (b *Bus) SetFxGain(g float32)  { b.fxGain.Store(math.Float32bits(g)) }
func (b *Bus) SetMicGain(g float32) { b.micGain.Store(math.Float32bits(g)) }

// Peaks returns the most recently measured peak levels (output, microphone), linear 0..1+.
func (b *Bus) Peaks() (out, mic float32) {
	return math.Float32frombits(b.peak.Load()), math.Float32frombits(b.micPeak.Load())
}

// send enqueues a command; returns false if the queue is full.
func (b *Bus) send(c command) bool {
	select {
	case b.cmds <- c:
		return true
	default:
		return false
	}
}

func (b *Bus) drain() {
	for {
		select {
		case c := <-b.cmds:
			switch c.kind {
			case cmdPlay:
				if len(b.voices) == maxVoices { // evict the oldest
					copy(b.voices, b.voices[1:])
					b.voices = b.voices[:maxVoices-1]
				}
				b.voices = append(b.voices, voice{tile: c.tile, snd: c.snd, gL: c.gL, gR: c.gR, loop: c.loop})
			case cmdStop:
				for i := range b.voices {
					if b.voices[i].tile == c.tile && b.voices[i].fadeLeft == 0 {
						b.voices[i].fadeLeft = stopFadeLen
					}
				}
			case cmdStopAll:
				for i := range b.voices {
					if b.voices[i].fadeLeft == 0 {
						b.voices[i].fadeLeft = stopFadeLen
					}
				}
			}
		default:
			return
		}
	}
}

// Fill fills an interleaved stereo buffer. Runs in the audio callback: no allocations, no locks.
func (b *Bus) Fill(out []float32) {
	for i := range out {
		out[i] = 0
	}
	b.drain()

	var micPk float32
	if mic := b.mic.Load(); mic != nil {
		mg := math.Float32frombits(b.micGain.Load())
		mic.Add(out, mg)
		for _, s := range out {
			if a := abs32(s); a > micPk {
				micPk = a
			}
		}
	}

	fg := math.Float32frombits(b.fxGain.Load())
	frames := len(out) / Channels
	for vi := 0; vi < len(b.voices); {
		v := &b.voices[vi]
		done := false
		data := v.snd.Data
		for f := 0; f < frames; f++ {
			if v.pos >= v.snd.Frames {
				if v.loop && v.snd.Frames > 0 {
					v.pos = 0
				} else {
					done = true
					break
				}
			}
			g := fg
			if v.fadeLeft > 0 {
				g *= float32(v.fadeLeft) / stopFadeLen
				v.fadeLeft--
				if v.fadeLeft == 0 {
					done = true
				}
			}
			out[f*2] += data[v.pos*2] * v.gL * g
			out[f*2+1] += data[v.pos*2+1] * v.gR * g
			v.pos++
			if done {
				break
			}
		}
		if done {
			tile := v.tile
			b.voices = append(b.voices[:vi], b.voices[vi+1:]...)
			if b.onEnd != nil && !b.hasTile(tile) {
				b.onEnd(b.name, tile)
			}
			continue
		}
		vi++
	}

	// Limiter: instant attack, ~100 ms release.
	const release = 0.0002
	var pk float32
	for f := 0; f < frames; f++ {
		l, r := out[f*2], out[f*2+1]
		p := abs32(l)
		if a := abs32(r); a > p {
			p = a
		}
		// Gain required for this frame; the release moves toward it (not toward 1) so the threshold is never exceeded.
		need := float32(1)
		if p > limThresh {
			need = limThresh / p
		}
		if need < b.limGain {
			b.limGain = need
		} else {
			b.limGain += (need - b.limGain) * release
		}
		l *= b.limGain
		r *= b.limGain
		out[f*2], out[f*2+1] = l, r
		if a := abs32(l); a > pk {
			pk = a
		}
		if a := abs32(r); a > pk {
			pk = a
		}
	}
	b.storePeak(&b.peak, pk)
	b.storePeak(&b.micPeak, micPk)
}

func (b *Bus) hasTile(tile string) bool {
	for i := range b.voices {
		if b.voices[i].tile == tile {
			return true
		}
	}
	return false
}

// storePeak holds the peak value with a gentle decay so the UI (polling) does not miss it.
func (b *Bus) storePeak(dst *atomic.Uint32, v float32) {
	old := math.Float32frombits(dst.Load()) * 0.97
	if v < old {
		v = old
	}
	dst.Store(math.Float32bits(v))
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// Balance converts a balance value (-1 left … +1 right) into channel factors.
// Like a stereo balance control: the opposite channel is attenuated, the other stays unchanged.
func Balance(pan float32) (gL, gR float32) {
	if pan < -1 {
		pan = -1
	}
	if pan > 1 {
		pan = 1
	}
	gL, gR = 1, 1
	if pan > 0 {
		gL = 1 - pan
	} else {
		gR = 1 + pan
	}
	return
}
