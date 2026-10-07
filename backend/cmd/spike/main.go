// spike: diagnostic tool (console). Lists devices and can run a long-duration test:
//
//	spike                      → list devices
//	spike run <in> <main> [<monitor>]  → microphone → main, tone every 5 s on main+monitor, statistics every second
//
// <in>/<main>/<monitor> are substrings of the device names.
package main

import (
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"soundboard/internal/audio"
	"soundboard/internal/audio/miniaudio"
)

func find(devs []audio.DeviceInfo, kind audio.DeviceKind, part string) string {
	for _, d := range devs {
		if d.Kind == kind && strings.Contains(strings.ToLower(d.Name), strings.ToLower(part)) {
			return d.ID
		}
	}
	return ""
}

func main() {
	be, err := miniaudio.New()
	if err != nil {
		fmt.Println("FEHLER:", err)
		os.Exit(1)
	}
	devs, err := be.Devices()
	if err != nil {
		fmt.Println("FEHLER:", err)
		os.Exit(1)
	}
	if len(os.Args) < 2 || os.Args[1] != "run" {
		for _, d := range devs {
			def := ""
			if d.IsDefault {
				def = " [Standard]"
			}
			fmt.Printf("%-8s %-60s%s\n         id=%s\n", d.Kind, d.Name, def, d.ID)
		}
		be.Close()
		return
	}
	if len(os.Args) < 4 {
		fmt.Println("usage: spike run <in> <main> [monitor]")
		os.Exit(2)
	}
	set := audio.Settings{
		InputID: find(devs, audio.KindCapture, os.Args[2]),
		MainID:  find(devs, audio.KindPlayback, os.Args[3]),
		MicGain: 1, FxGainMain: 1, FxGainMonitor: 1, PeriodMs: 10,
	}
	if len(os.Args) > 4 {
		set.MonitorID = find(devs, audio.KindPlayback, os.Args[4])
	}
	e := audio.NewEngine(be)
	defer e.Close()
	e.ApplyNow(set)

	// Test tone 440 Hz, 0.5 s
	beep := &audio.Sound{Name: "beep", Frames: audio.SampleRate / 2}
	beep.Data = make([]float32, beep.Frames*2)
	for i := 0; i < beep.Frames; i++ {
		v := 0.3 * float32(math.Sin(2*3.14159265*440*float64(i)/audio.SampleRate))
		beep.Data[i*2], beep.Data[i*2+1] = v, v
	}
	start := time.Now()
	for i := 0; ; i++ {
		time.Sleep(time.Second)
		if i%5 == 0 {
			e.Trigger("beep", beep, 1, 0, audio.ModeOneShot)
		}
		st := e.Status()
		fmt.Printf("%4ds in=%v main=%v mon=%v mic=%.2f out=%.2f under=%d over=%d ratio=%.6f %s%s\n",
			int(time.Since(start).Seconds()), st.Input.Running, st.Main.Running, st.Monitor.Running,
			st.MicPeak, st.MainPeak, st.MicUnderruns, st.MicOverruns, st.MicRatio, st.Input.Error, st.Main.Error)
	}
}
