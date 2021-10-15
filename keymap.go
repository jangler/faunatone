package main

import (
	"log"
	"math"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/veandco/go-sdl2/sdl"
	"gitlab.com/gomidi/midi/writer"
)

var keymapPath = filepath.Join(configPath, "keymap.tsv")

// turns key events into note events
type keymap map[string]float64

// load a keymap from a file
func newKeymap(path string) keymap {
	k := make(map[string]float64)
	if records, err := readTSV(path); err == nil {
		for _, rec := range records {
			ok := false
			if len(rec) == 2 {
				if pitch, ok2 := parsePitch(rec[1]); ok2 {
					k[rec[0]] = pitch
					ok = true
				}
			}
			if !ok {
				log.Printf("bad keymap record: %q", rec)
			}
		}
	} else {
		log.Print(err)
	}
	return keymap(k)
}

var (
	ratioRegexp   = regexp.MustCompile(`([0-9.]+)/([0-9.]+)`)
	edoStepRegexp = regexp.MustCompile(`(-?[0-9.]+)\\([0-9.]+)`)
)

// convert a string to a floating-point midi pitch offset
func parsePitch(s string) (float64, bool) {
	if m := ratioRegexp.FindAllStringSubmatch(s, 1); m != nil {
		num, _ := strconv.ParseFloat(m[0][1], 64)
		den, _ := strconv.ParseFloat(m[0][2], 64)
		return 12 * math.Log(num/den) / math.Log(2), true
	} else if m := edoStepRegexp.FindAllStringSubmatch(s, 1); m != nil {
		step, _ := strconv.ParseFloat(m[0][1], 64)
		edo, _ := strconv.ParseFloat(m[0][2], 64)
		return 12 / edo * step, true
	} else if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f, true
	}
	return 0, false
}

// respond to keyboard events
func (k keymap) keyboardEvent(e *sdl.KeyboardEvent, pe *patternEditor, wr *writer.Writer) {
	if e.Repeat != 0 || e.State != sdl.PRESSED {
		return
	}
	if pitch, ok := k[formatKeyEvent(e)]; ok {
		pitch += float64(pe.octave * 12)
		if pitch < -2 {
			pitch = -2
		} else if pitch > 129 {
			pitch = 129
		}
		pe.writeEvent(newTrackEvent(&trackEvent{
			Type:      noteOnEvent,
			FloatData: pitch,
			ByteData1: pe.velocity,
		}))
		note, bend := pitchToMIDI(pitch)
		wr.SetChannel(0)
		writer.Pitchbend(wr, bend)
		writer.NoteOn(wr, note, pe.velocity)
		writer.NoteOff(wr, note)
	}
}
