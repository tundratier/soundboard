package audio

import (
	"math"
	"sync/atomic"
)

// MicReader liest das Mikrofon aus dem Ring und gleicht die Taktdrift zwischen Capture- und
// Playback-Gerät per leicht variierender Wiedergaberate (lineare Interpolation) aus.
// Die Rate wird über einen PI-Regler am Füllstand des Rings geführt.
type MicReader struct {
	ring   *Ring
	target int // Ziel-Füllstand in Frames

	primed bool
	frac   float64 // Position zwischen Frame 0 und 1 des Rings
	avg    float64 // geglätteter Füllstand
	integ  float64 // I-Anteil der Rate-Abweichung

	// Statistik (atomar lesbar)
	Underruns atomic.Uint64
	Overruns  atomic.Uint64
	ratioBits atomic.Uint64
}

func NewMicReader(ring *Ring, targetFrames int) *MicReader {
	return &MicReader{ring: ring, target: targetFrames, avg: float64(targetFrames)}
}

// Ratio liefert die aktuelle Rate (1.0 = nominal), für Diagnose.
func (m *MicReader) Ratio() float64 { return math.Float64frombits(m.ratioBits.Load()) }

const (
	micKp        = 1e-5  // Rate-Abweichung pro Frame Füllstandsfehler
	micKi        = 1e-8  // Integral pro Callback
	micMaxDev    = 0.005 // ±0,5 % maximale Ratenabweichung
	micSmoothing = 0.2
)

// Add mischt (addiert) frames Frames des Mikrofons mit Faktor gain in out.
func (m *MicReader) Add(out []float32, gain float32) {
	frames := len(out) / Channels
	avail := m.ring.Available()

	if !m.primed {
		if avail < m.target {
			return // erst Zielfüllstand aufbauen
		}
		m.primed = true
		m.frac = 0
		m.avg = float64(avail)
		m.integ = 0
	}

	// Füllstand ist beim Eintritt sägezahnförmig (Capture liefert in Perioden) → glätten.
	m.avg += (float64(avail) - m.avg) * micSmoothing
	err := m.avg - float64(m.target)
	m.integ += err * micKi
	m.integ = clamp64(m.integ, -micMaxDev, micMaxDev)
	dev := clamp64(m.integ+err*micKp, -micMaxDev, micMaxDev)
	ratio := 1 + dev
	m.ratioBits.Store(math.Float64bits(ratio))

	// Gewaltige Überfüllung (z. B. nach Stall) → hart auf Ziel zurück.
	if avail > m.target*4+frames {
		m.ring.skip(avail - m.target)
		m.Overruns.Add(1)
		m.frac = 0
		avail = m.target
	}

	for i := 0; i < frames; i++ {
		// Für Interpolation werden Frame 0 und 1 benötigt.
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
