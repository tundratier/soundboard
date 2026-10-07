package audio

import (
	"math"
	"sync/atomic"
)

// MicReader reads the microphone from the ring and compensates clock drift between the capture and
// playback devices by slightly varying the playback rate (linear interpolation).
// The rate is steered by a PI controller on the ring's fill level.
type MicReader struct {
	ring   *Ring
	target int // target fill level in frames

	primed bool
	frac   float64 // position between frame 0 and 1 of the ring
	avg    float64 // smoothed fill level
	integ  float64 // integral term of the rate deviation

	// Statistics (readable atomically)
	Underruns atomic.Uint64
	Overruns  atomic.Uint64
	ratioBits atomic.Uint64
}

func NewMicReader(ring *Ring, targetFrames int) *MicReader {
	return &MicReader{ring: ring, target: targetFrames, avg: float64(targetFrames)}
}

// Ratio returns the current rate (1.0 = nominal), for diagnostics.
func (m *MicReader) Ratio() float64 { return math.Float64frombits(m.ratioBits.Load()) }

const (
	micKp        = 1e-5  // rate deviation per frame of fill-level error
	micKi        = 1e-8  // integral per callback
	micMaxDev    = 0.005 // ±0.5 % maximum rate deviation
	micSmoothing = 0.2
)

// Add mixes (adds) the microphone frames into out with factor gain.
func (m *MicReader) Add(out []float32, gain float32) {
	frames := len(out) / Channels
	avail := m.ring.Available()

	if !m.primed {
		if avail < m.target {
			return // build up the target fill level first
		}
		m.primed = true
		m.frac = 0
		m.avg = float64(avail)
		m.integ = 0
	}

	// The fill level is sawtooth-shaped on entry (capture delivers in periods) → smooth it.
	m.avg += (float64(avail) - m.avg) * micSmoothing
	err := m.avg - float64(m.target)
	m.integ += err * micKi
	m.integ = clamp64(m.integ, -micMaxDev, micMaxDev)
	dev := clamp64(m.integ+err*micKp, -micMaxDev, micMaxDev)
	ratio := 1 + dev
	m.ratioBits.Store(math.Float64bits(ratio))

	// Huge overfill (e.g. after a stall) → jump hard back to the target.
	if avail > m.target*4+frames {
		m.ring.skip(avail - m.target)
		m.Overruns.Add(1)
		m.frac = 0
		avail = m.target
	}

	for i := 0; i < frames; i++ {
		// Interpolation needs frames 0 and 1.
		if m.ring.Available() < 2 {
			m.Underruns.Add(1)
			m.primed = false
			return
		}
		l0, r0 := m.ring.peek(0)
		l1, r1 := m.ring.peek(1)
		f := float32(m.frac)
		out[i*2] += (l0 + (l1-l0)*f) * gain
		out[i*2+1] += (r0 + (r1-r0)*f) * gain

		m.frac += ratio
		if adv := int(m.frac); adv > 0 {
			m.ring.skip(adv)
			m.frac -= float64(adv)
		}
	}
}

func clamp64(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
