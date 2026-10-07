package library

import (
	"encoding/binary"
	"math"
	"os"
)

// WriteTestTone schreibt einen kurzen Zwei-Ton-Signalton als 16-bit-Mono-WAV (44,1 kHz).
// Dient als Beispiel-Sound beim ersten Start und in Tests.
func WriteTestTone(path string) error {
	const rate = 44100
	n := rate * 4 / 10 // 0,4 s
	pcm := make([]int16, n)
	for i := range pcm {
		f := 660.0
		if i >= n/2 {
			f = 880
		}
		env := math.Min(1, math.Min(float64(i%(n/2))/400, float64(n/2-i%(n/2))/400)) // kurze Ein-/Ausblendung
		pcm[i] = int16(0.35 * env * 32767 * math.Sin(2*math.Pi*f*float64(i)/rate))
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	dataLen := uint32(len(pcm) * 2)
	hdr := struct {
		RIFF       [4]byte
		Size       uint32
		WAVE, FMT  [4]byte
		FmtLen     uint32
		Format     uint16
		Channels   uint16
		Rate       uint32
		ByteRate   uint32
		BlockAlign uint16
		Bits       uint16
		DATA       [4]byte
		DataLen    uint32
	}{[4]byte{'R', 'I', 'F', 'F'}, 36 + dataLen, [4]byte{'W', 'A', 'V', 'E'}, [4]byte{'f', 'm', 't', ' '},
		16, 1, 1, rate, rate * 2, 2, 16, [4]byte{'d', 'a', 't', 'a'}, dataLen}
	if err := binary.Write(f, binary.LittleEndian, hdr); err != nil {
		return err
	}
	return binary.Write(f, binary.LittleEndian, pcm)
}
