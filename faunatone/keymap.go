package main

import (
	"fmt"
	"log"
	"math"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/veandco/go-sdl2/sdl"
)

var (
	keymapPath        = filepath.Join(configPath, "keymaps")
	defaultKeymapPath = "12edo.tsv"

	qwertyLayout = [][]string{
		{"1", "2", "3", "4", "5", "6", "7", "8", "9", "0"},
		{"Q", "W", "E", "R", "T", "Y", "U", "I", "O", "P"},
		{"A", "S", "D", "F", "G", "H", "J", "K", "L"},
		{"Z", "X", "C", "V", "B", "N", "M"},
	}
	isoCenterX = 4
	isoCenterY = 2
)

// turns key events into note events
type keymap struct {
	keymap  map[string]float64
	name    string
	lastKey string
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
				if pitch, err := parsePitch(rec[1], k); err == nil {
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

// generate a two-dimensional isomorphic keyboard keymap from two intervals
func genIsoKeymap(interval1, interval2 float64) *keymap {
	k := &keymap{
		keymap: make(map[string]float64),
		name:   "gen-iso",
	}
	for y, row := range qwertyLayout {
		for x, key := range row {
			k.keymap[key] = interval1*float64(x-isoCenterX+y-2) + interval2*float64(isoCenterY-y)
		}
	}
	return k
}

var (
	ratioRegexp   = regexp.MustCompile(`([0-9.]+)/([0-9.]+)`)
	edoStepRegexp = regexp.MustCompile(`(-?[0-9.]+)\\([0-9.]+)`)
	keyRefRegexp  = regexp.MustCompile(`@(.+)`)
)

// convert a string to a floating-point midi pitch offset
func parsePitch(s string, k *keymap) (float64, error) {
	if m := ratioRegexp.FindAllStringSubmatch(s, 1); m != nil {
		num, _ := strconv.ParseFloat(m[0][1], 64)
		den, _ := strconv.ParseFloat(m[0][2], 64)
		return 12 * math.Log(num/den) / math.Log(2), nil
	} else if m := edoStepRegexp.FindAllStringSubmatch(s, 1); m != nil {
		step, _ := strconv.ParseFloat(m[0][1], 64)
		edo, _ := strconv.ParseFloat(m[0][2], 64)
		return 12 / edo * step, nil
	} else if m := keyRefRegexp.FindAllStringSubmatch(s, 1); m != nil {
		if f, ok := k.keymap[m[0][1]]; ok {
			return f, nil
		}
		return 0, fmt.Errorf("no key \"%s\" in keymap", m[0][1])
	} else if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f, nil
	}
	return 0, fmt.Errorf("invalid pitch syntax")
}

// respond to keyboard events
func (k *keymap) keyboardEvent(e *sdl.KeyboardEvent, pe *patternEditor, p *player) {
	if e.Repeat != 0 {
		return
	}
	s := strings.Replace(formatKeyEvent(e), "Shift+", "", 1)
	if pitch, ok := k.keymap[s]; ok {
		if e.State == sdl.PRESSED {
			k.lastKey = s
			pitch += pe.refPitch
			if pitch < minPitch {
				pitch = minPitch
			} else if pitch > maxPitch {
				pitch = maxPitch
			}
			note, _ := pitchToMIDI(pitch)
			if e.Keysym.Mod&sdl.KMOD_SHIFT == 0 {
				pe.writeEvent(newTrackEvent(&trackEvent{
					Type:      noteOnEvent,
					FloatData: pitch,
					ByteData1: pe.velocity,
				}), p)
			} else {
				pe.writeEvent(newTrackEvent(&trackEvent{
					Type:      drumNoteOnEvent,
					ByteData1: note,
					ByteData2: pe.velocity,
				}), p)
			}
		} else if s == k.lastKey {
			k.lastKey = ""
			pe.playSelectionNoteOff(p)
		}
	}
}
