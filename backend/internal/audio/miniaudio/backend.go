// Package miniaudio implementiert audio.Backend auf Basis von miniaudio/WASAPI (via malgo, cgo).
package miniaudio

import (
	"fmt"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"unicode/utf16"
	"unsafe"

	"github.com/gen2brain/malgo"

	"soundboard/internal/audio"
)

var debug = os.Getenv("SOUNDBOARD_DEBUG") != ""

type Backend struct {
	mu  sync.Mutex // miniaudio-Kontext ist nicht für parallele Enumeration/Init gedacht
	ctx *malgo.AllocatedContext
}

func New() (*Backend, error) {
	ctx, err := malgo.InitContext([]malgo.Backend{malgo.BackendWasapi}, malgo.ContextConfig{}, func(msg string) {
		if debug {
			log.Printf("miniaudio: %s", trimNL(msg))
		}
	})
	if err != nil {
		return nil, fmt.Errorf("miniaudio-Kontext: %w", err)
	}
	return &Backend{ctx: ctx}, nil
}

func trimNL(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}

// idToString dekodiert die WASAPI-Endpoint-ID (UTF-16LE, nullterminiert) aus der ma_device_id.
func idToString(id malgo.DeviceID) string {
	var u []uint16
	for i := 0; i+1 < 128 && i+1 < len(id); i += 2 {
		c := uint16(id[i]) | uint16(id[i+1])<<8
		if c == 0 {
			break
		}
		u = append(u, c)
	}
	return string(utf16.Decode(u))
}

func stringToID(s string) malgo.DeviceID {
	var id malgo.DeviceID
	for i, c := range utf16.Encode([]rune(s)) {
		if i*2+1 >= 128 {
			break
		}
		id[i*2] = byte(c)
		id[i*2+1] = byte(c >> 8)
	}
	return id
}

func (b *Backend) Devices() ([]audio.DeviceInfo, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []audio.DeviceInfo
	for _, k := range []struct {
		t    malgo.DeviceType
		kind audio.DeviceKind
	}{{malgo.Capture, audio.KindCapture}, {malgo.Playback, audio.KindPlayback}} {
		devs, err := b.ctx.Devices(k.t)
		if err != nil {
			return nil, fmt.Errorf("Geräteliste (%s): %w", k.kind, err)
		}
		for i := range devs {
			out = append(out, audio.DeviceInfo{
				ID:        idToString(devs[i].ID),
				Name:      devs[i].Name(),
				Kind:      k.kind,
				IsDefault: devs[i].IsDefault != 0,
			})
		}
	}
	return out, nil
}

// stream kapselt ein malgo-Gerät. closing verhindert, dass ein beabsichtigtes Schließen als Ausfall gemeldet wird.
type stream struct {
	dev     *malgo.Device
	closing atomic.Bool
	once    sync.Once
}

func (s *stream) Close() {
	s.once.Do(func() {
		s.closing.Store(true)
		s.dev.Uninit()
	})
}

func floats(b []byte) []float32 {
	if len(b) == 0 {
		return nil
	}
	return unsafe.Slice((*float32)(unsafe.Pointer(&b[0])), len(b)/4)
}

func (b *Backend) open(t malgo.DeviceType, deviceID string, periodMs int, data malgo.DataProc, onStop func()) (audio.Stream, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	cfg := malgo.DefaultDeviceConfig(t)
	cfg.SampleRate = audio.SampleRate
	cfg.PeriodSizeInMilliseconds = uint32(periodMs)
	cfg.Periods = 3
	cfg.PerformanceProfile = malgo.LowLatency
	id := stringToID(deviceID)
	switch t {
	case malgo.Capture:
		cfg.Capture.Format, cfg.Capture.Channels = malgo.FormatF32, audio.Channels
		cfg.Capture.ShareMode = malgo.Shared
		cfg.Capture.DeviceID = id.Pointer()
	default:
		cfg.Playback.Format, cfg.Playback.Channels = malgo.FormatF32, audio.Channels
		cfg.Playback.ShareMode = malgo.Shared
		cfg.Playback.DeviceID = id.Pointer()
	}

	s := &stream{}
	cbs := malgo.DeviceCallbacks{
		Data: data,
		Stop: func() {
			if !s.closing.Load() && onStop != nil {
				onStop()
			}
		},
	}
	dev, err := malgo.InitDevice(b.ctx.Context, cfg, cbs)
	if err != nil {
		return nil, err
	}
	s.dev = dev
	if err := dev.Start(); err != nil {
		dev.Uninit()
		return nil, err
	}
	return s, nil
}

func (b *Backend) OpenCapture(deviceID string, periodMs int, onData func(in []float32), onStop func()) (audio.Stream, error) {
	return b.open(malgo.Capture, deviceID, periodMs, func(_, in []byte, _ uint32) {
		onData(floats(in))
	}, onStop)
}

func (b *Backend) OpenPlayback(deviceID string, periodMs int, fill func(out []float32), onStop func()) (audio.Stream, error) {
	return b.open(malgo.Playback, deviceID, periodMs, func(out, _ []byte, _ uint32) {
		fill(floats(out))
	}, onStop)
}

func (b *Backend) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.ctx != nil {
		_ = b.ctx.Uninit()
		b.ctx.Free()
		b.ctx = nil
	}
}

var _ audio.Backend = (*Backend)(nil)
