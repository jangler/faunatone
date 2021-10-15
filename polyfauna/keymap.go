package main

import (
	"log"
	"math"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/veandco/go-sdl2/sdl"
	"gitlab.com/gomidi/midi/writer"
)

var (
	keymapPath        = filepath.Join(configPath, "keymaps")
	defaultKeymapPath = "12edo.tsv"
)

// turns key events into note events
type keymap struct {
	keymap map[string]float64
	name   string
}

// load a keymap from a file
func newKeymap(path string) (*keymap, error) {
	k := &keymap{
		keymap: make(map[string]float64),
		name:   strings.Replace(filepath.Base(path), ".tsv", "", 1),
	}
	if records, err := readTSV(filepath.Join(keymapPath, path)); err == nil {
		for _, rec := range records {
			ok := false
			if len(rec) == 2 {
				if pitch, ok2 := parsePitch(rec[1]); ok2 {
					k.keymap[rec[0]] = pitch
					ok = true
				}
			}
			if !ok {
				log.Printf("bad keymap record: %q", rec)
			}
		}
	} else {
		k.name = "none"
		return k, err
	}
	return k, nil
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
func (k *keymap) keyboardEvent(e *sdl.KeyboardEvent, pe *patternEditor, wr *writer.Writer) {
	if e.Repeat != 0 || e.State != sdl.PRESSED {
		return
	}
	s := strings.Replace(formatKeyEvent(e), "Shift+", "", 1)
	if pitch, ok := k.keymap[s]; ok {
		pitch += float64(pe.octave * 12)
		if pitch < -2 {
			pitch = -2
		} else if pitch > 129 {
			pitch = 129
		}
		note, bend := pitchToMIDI(pitch)
		if e.Keysym.Mod&sdl.KMOD_SHIFT == 0 {
			pe.writeEvent(newTrackEvent(&trackEvent{
				Type:      noteOnEvent,
				FloatData: pitch,
				ByteData1: pe.velocity,
			}))
			wr.SetChannel(0)
			writer.Pitchbend(wr, bend)
		} else {
			pe.writeEvent(newTrackEvent(&trackEvent{
				Type:      drumNoteOnEvent,
				ByteData1: note,
				ByteData2: pe.velocity,
			}))
			wr.SetChannel(percussionChannelIndex)
		}
		writer.NoteOn(wr, note, pe.velocity)
		writer.NoteOff(wr, note)
	}
}
