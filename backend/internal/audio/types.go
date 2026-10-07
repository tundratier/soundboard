// Package audio enthält den Echtzeit-Kern: Ringpuffer, Drift-Ausgleich, Mixer-Busse und die Engine.
// Es ist reines Go; die eigentliche Geräteanbindung (cgo/miniaudio) steckt in audio/miniaudio.
package audio

const (
	SampleRate = 48000
	Channels   = 2
)

// Sound ist ein vollständig dekodierter Klang: interleaved float32, Stereo, 48 kHz.
type Sound struct {
	Name   string
	Data   []float32
	Frames int
}

// PlayMode bestimmt das Verhalten einer Kachel beim erneuten Auslösen.
type PlayMode string

const (
	ModeOneShot PlayMode = "oneshot" // jeder Druck startet eine weitere Instanz
	ModeRestart PlayMode = "restart" // laufende Instanz wird abgebrochen und neu gestartet
	ModeToggle  PlayMode = "toggle"  // zweiter Druck stoppt
	ModeLoop    PlayMode = "loop"    // wie toggle, aber in Endlosschleife
)

// DeviceKind unterscheidet Aufnahme- und Wiedergabegeräte.
type DeviceKind string

const (
	KindCapture  DeviceKind = "capture"
	KindPlayback DeviceKind = "playback"
)

// DeviceInfo beschreibt ein Audiogerät. ID ist die WASAPI-Endpoint-ID (stabil über Neustarts).
type DeviceInfo struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Kind      DeviceKind `json:"kind"`
	IsDefault bool       `json:"isDefault"`
}

// Stream ist ein laufender Gerätestrom.
type Stream interface {
	Close()
}

// Backend abstrahiert die Audio-Hardware (miniaudio im Produktivbetrieb, Fake in Tests).
// Callbacks laufen auf Audio-Threads und dürfen nicht blockieren.
type Backend interface {
	Devices() ([]DeviceInfo, error)
	// OpenCapture öffnet ein Mikrofon: 48 kHz, float32, Stereo, interleaved.
	OpenCapture(deviceID string, periodMs int, onData func(in []float32), onStop func()) (Stream, error)
	// OpenPlayback öffnet eine Ausgabe: 48 kHz, float32, Stereo, interleaved.
	OpenPlayback(deviceID string, periodMs int, fill func(out []float32), onStop func()) (Stream, error)
	Close()
}
