// Package audio contains the real-time core: ring buffer, drift compensation, mixer buses and the engine.
// It is plain Go; the actual device binding (cgo/miniaudio) lives in audio/miniaudio.
package audio

const (
	SampleRate = 48000
	Channels   = 2
)

// Sound is a fully decoded clip: interleaved float32, stereo, 48 kHz.
type Sound struct {
	Name   string
	Data   []float32
	Frames int
}

// PlayMode determines a tile's behavior when it is triggered again.
type PlayMode string

const (
	ModeOneShot PlayMode = "oneshot" // every press starts another instance
	ModeRestart PlayMode = "restart" // a running instance is aborted and restarted
	ModeToggle  PlayMode = "toggle"  // second press stops
	ModeLoop    PlayMode = "loop"    // like toggle, but loops endlessly
)

// DeviceKind distinguishes capture and playback devices.
type DeviceKind string

const (
	KindCapture  DeviceKind = "capture"
	KindPlayback DeviceKind = "playback"
)

// DeviceInfo describes an audio device. ID is the WASAPI endpoint ID (stable across restarts).
type DeviceInfo struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Kind      DeviceKind `json:"kind"`
	IsDefault bool       `json:"isDefault"`
}

// Stream is a running device stream.
type Stream interface {
	Close()
}

// Backend abstracts the audio hardware (miniaudio in production, a fake in tests).
// Callbacks run on audio threads and must not block.
type Backend interface {
	Devices() ([]DeviceInfo, error)
	// OpenCapture opens a microphone: 48 kHz, float32, stereo, interleaved.
	OpenCapture(deviceID string, periodMs int, onData func(in []float32), onStop func()) (Stream, error)
	// OpenPlayback opens an output: 48 kHz, float32, stereo, interleaved.
	OpenPlayback(deviceID string, periodMs int, fill func(out []float32), onStop func()) (Stream, error)
	Close()
}
