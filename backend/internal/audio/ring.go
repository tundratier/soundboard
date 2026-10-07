package audio

import "sync/atomic"

// Ring ist ein Single-Producer/Single-Consumer-Ringpuffer für interleaved Stereo-Samples.
// Kapazität und Zähler sind in Frames; Producer und Consumer laufen auf verschiedenen Audio-Threads.
type Ring struct {
	buf  []float32
	mask uint64
	w, r atomic.Uint64 // monoton steigende Frame-Zähler
}

// NewRing erzeugt einen Ring mit mindestens capFrames Frames (aufgerundet auf 2^n).
func NewRing(capFrames int) *Ring {
	n := 1
	for n < capFrames {
		n <<= 1
	}
	return &Ring{buf: make([]float32, n*Channels), mask: uint64(n - 1)}
}

func (r *Ring) Capacity() int { return len(r.buf) / Channels }

// Available liefert die Anzahl lesbarer Frames.
func (r *Ring) Available() int { return int(r.w.Load() - r.r.Load()) }

// Write schreibt Frames; bei vollem Puffer werden die ältesten verworfen (Rückgabe: verworfene Frames).
func (r *Ring) Write(in []float32) (dropped int) {
	frames := len(in) / Channels
	capacity := r.Capacity()
	if frames > capacity { // nur die neuesten behalten
		dropped += frames - capacity
		in = in[(frames-capacity)*Channels:]
		frames = capacity
	}
	w := r.w.Load()
	// Platz schaffen, indem der Reader nach vorn geschoben wird, ist beim SPSC-Ring nicht erlaubt
	// (nur der Consumer bewegt r). Stattdessen: überzählige Frames nicht schreiben.
	free := capacity - int(w-r.r.Load())
	if frames > free {
		dropped += frames - free
		in = in[:free*Channels]
		frames = free
	}
	for i := 0; i < frames; i++ {
		idx := ((w + uint64(i)) & r.mask) * Channels
		r.buf[idx] = in[i*Channels]
		r.buf[idx+1] = in[i*Channels+1]
	}
	r.w.Store(w + uint64(frames))
	return dropped
}

// peek liefert Frame i relativ zur Leseposition (ohne zu konsumieren). Nur Consumer.
func (r *Ring) peek(i int) (l, rr float32) {
	idx := ((r.r.Load() + uint64(i)) & r.mask) * Channels
	return r.buf[idx], r.buf[idx+1]
}

// skip konsumiert n Frames. Nur Consumer.
func (r *Ring) skip(n int) { r.r.Add(uint64(n)) }
