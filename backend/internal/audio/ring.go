package audio

import "sync/atomic"

// Ring is a single-producer/single-consumer ring buffer for interleaved stereo samples.
// Capacity and counters are in frames; producer and consumer run on different audio threads.
type Ring struct {
	buf  []float32
	mask uint64
	w, r atomic.Uint64 // monotonically increasing frame counters
}

// NewRing creates a ring with at least capFrames frames (rounded up to 2^n).
func NewRing(capFrames int) *Ring {
	n := 1
	for n < capFrames {
		n <<= 1
	}
	return &Ring{buf: make([]float32, n*Channels), mask: uint64(n - 1)}
}

func (r *Ring) Capacity() int { return len(r.buf) / Channels }

// Available returns the number of readable frames.
func (r *Ring) Available() int { return int(r.w.Load() - r.r.Load()) }

// Write appends frames. Frames that do not fit are dropped (returns the number of dropped frames).
func (r *Ring) Write(in []float32) (dropped int) {
	frames := len(in) / Channels
	capacity := r.Capacity()
	if frames > capacity { // a single oversized write keeps only the newest frames
		dropped += frames - capacity
		in = in[(frames-capacity)*Channels:]
		frames = capacity
	}
	w := r.w.Load()
	// Making room by advancing the reader is not allowed in an SPSC ring
	// (only the consumer moves r). Instead: do not write the surplus frames.
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

// peek returns frame i relative to the read position (without consuming). Consumer only.
func (r *Ring) peek(i int) (l, rr float32) {
	idx := ((r.r.Load() + uint64(i)) & r.mask) * Channels
	return r.buf[idx], r.buf[idx+1]
}

// skip consumes n frames. Consumer only.
func (r *Ring) skip(n int) { r.r.Add(uint64(n)) }
