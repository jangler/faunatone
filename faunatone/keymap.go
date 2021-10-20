package main

import (
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/veandco/go-sdl2/sdl"
)

var (
	keymapPath = filepath.Join(configPath, "keymaps")

	qwertyLayout = [][]string{
		{"1", "2", "3", "4", "5", "6", "7", "8", "9", "0"},
		{"Q", "W", "E", "R", "T", "Y", "U", "I", "O", "P"},
		{"A", "S", "D", "F", "G", "H", "J", "K", "L"},
		{"Z", "X", "C", "V", "B", "N", "M"},
	}
	isoCenterX = 4
	isoCenterY = 2

	midiRegexp = regexp.MustCompile(`m(\d+)`)
)

// turns key events into note events
type keymap struct {
	keymap   []*keyInfo
	midimap  [128]float64
	name     string
	lastKey  string
	lastMidi byte
}

// an entry in a keymap
type keyInfo struct {
	Key      string
	IsMod    bool
	Interval float64
	Name     string

	class float64 // like pitch class; derived from Interval
}

// initialize a new key
func newKeyInfo(key string, isMod bool, interval float64, name string) *keyInfo {
	return &keyInfo{
		Key:      key,
		IsMod:    isMod,
		Interval: interval,
		Name:     name,
		class:    posMod(interval, 12),
	}
}

// modulo where result is always in the range [0, y)
func posMod(x, y float64) float64 {
	x = math.Mod(x, y)
	if x < 0 {
		x += y
	}
	return x
}

// load a keymap from a file
func newKeymap(path string) (*keymap, error) {
	errs := []string{}
	k := &keymap{
		name:     strings.Replace(filepath.Base(path), ".csv", "", 1),
		lastMidi: byteNil,
	}
	firstMidi, lastMidi := -1, -1
	if records, err := readCSV(filepath.Join(keymapPath, path)); err == nil {
		for _, rec := range records {
			ok := false
			if len(rec) == 3 {
				if pitch, err := parsePitch(rec[2], k); err == nil {
					k.keymap = append(k.keymap, newKeyInfo(
						rec[0], strings.HasPrefix(rec[2], "*"), pitch, rec[1]))
					if midiRegexp.MatchString(rec[0]) {
						if i, err := strconv.ParseUint(rec[0][1:], 10, 8); err == nil && i < 128 {
							k.midimap[i] = pitch
							ok = true
							if firstMidi == -1 || int(i) < firstMidi {
								firstMidi = int(i)
							}
							if int(i) > lastMidi {
								lastMidi = int(i)
							}
						}
					} else {
						ok = true
					}
				}
			}
			if !ok {
				errs = append(errs, fmt.Sprintf("bad keymap record: %q", rec))
			}
		}
	} else {
		k.name = "none"
		return k, err
	}
	k.repeatMidiPattern(firstMidi, lastMidi)
	if len(errs) > 0 {
		return k, errors.New(strings.Join(errs, "\n"))
	}
	return k, nil
}

// repeats the pattern of midi notes already present in the keymap across the
// entire range
func (k *keymap) repeatMidiPattern(firstIndex, lastIndex int) {
	if firstIndex != -1 && lastIndex != -1 {
		octave := k.midimap[lastIndex] - k.midimap[firstIndex]
		for i := range k.midimap {
			period := math.Floor(float64(i-firstIndex) / float64(lastIndex-firstIndex))
			index := firstIndex + ((i - firstIndex) % (lastIndex - firstIndex))
			if index < firstIndex {
				index += lastIndex - firstIndex
			}
			k.midimap[i] = k.midimap[index] + period*octave
		}
	}
}

// generate a two-dimensional isomorphic keyboard keymap from two intervals
func genIsoKeymap(interval1, interval2 float64) *keymap {
	k := &keymap{
		name: "gen-iso",
	}
	for y, row := range qwertyLayout {
		for x, key := range row {
			k.keymap = append(k.keymap, newKeyInfo(key, false,
				interval1*float64(x-isoCenterX+y-2)+interval2*float64(isoCenterY-y),
				fmt.Sprintf("(%d,%d)", x-isoCenterX+y-2, isoCenterY-y),
			))
		}
	}
	return k
}

var (
	ratioRegexp   = regexp.MustCompile(`^([0-9.]+)/([0-9.]+)$`)
	edoStepRegexp = regexp.MustCompile(`^(-?[0-9.]+)\\([0-9.]+)$`)
	keyRefRegexp  = regexp.MustCompile(`^@(.+)$`)
)

// convert a string to a floating-point midi pitch offset
func parsePitch(s string, k *keymap) (float64, error) {
	if strings.HasPrefix(s, "*") {
		s = s[1:]
	}
	if m := ratioRegexp.FindAllStringSubmatch(s, 1); m != nil {
		num, _ := strconv.ParseFloat(m[0][1], 64)
		den, _ := strconv.ParseFloat(m[0][2], 64)
		return 12 * math.Log(num/den) / math.Log(2), nil
	} else if m := edoStepRegexp.FindAllStringSubmatch(s, 1); m != nil {
		step, _ := strconv.ParseFloat(m[0][1], 64)
		edo, _ := strconv.ParseFloat(m[0][2], 64)
		return 12 / edo * step, nil
	} else if m := keyRefRegexp.FindAllStringSubmatch(s, 1); m != nil {
		if ki := k.getByKey(m[0][1]); ki != nil {
			return ki.Interval, nil
		}
		return 0, fmt.Errorf("no key \"%s\" in keymap", m[0][1])
	} else if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f, nil
	}
	return 0, fmt.Errorf("invalid pitch syntax")
}

// return a keyInfo with a matching key, if any
func (k *keymap) getByKey(key string) *keyInfo {
	for _, ki := range k.keymap {
		if ki.Key == key {
			return ki
		}
	}
	return nil
}

// respond to keyboard events
func (k *keymap) keyboardEvent(e *sdl.KeyboardEvent, pe *patternEditor, p *player) {
	if e.Repeat != 0 {
		return
	}
	s := strings.Replace(formatKeyEvent(e), "Shift+", "", 1)
	if pitch, ok := k.pitchFromString(s, pe.refPitch); ok {
		if e.State == sdl.PRESSED {
			k.lastKey = s
			if k.getByKey(s).IsMod {
				pe.transposeSelection(pitch-pe.refPitch, k)
			} else {
				if e.Keysym.Mod&sdl.KMOD_SHIFT == 0 {
					pe.writeEvent(newTrackEvent(&trackEvent{
						Type:      noteOnEvent,
						FloatData: pitch,
						ByteData1: pe.velocity,
					}, k.keymap), p)
				} else {
					note, _ := pitchToMIDI(pitch)
					pe.writeEvent(newTrackEvent(&trackEvent{
						Type:      drumNoteOnEvent,
						ByteData1: note,
						ByteData2: pe.velocity,
					}, k.keymap), p)
				}
			}
		} else if s == k.lastKey {
			k.lastKey = ""
			pe.playSelectionNoteOff(p)
		}
	}
}

// respond to midi input events
func (k *keymap) midiEvent(msg []byte, pe *patternEditor, p *player) {
	if msg[0]&0xf0 == 0x90 && msg[2] > 0 { // note on
		k.lastMidi = msg[1]
		if sdl.GetModState()&sdl.KMOD_SHIFT == 0 {
			pitch := k.midimap[msg[1]] + pe.refPitch
			pe.writeEvent(newTrackEvent(&trackEvent{
				Type:      noteOnEvent,
				FloatData: pitch,
				ByteData1: msg[2],
			}, k.keymap), p)
		} else {
			pe.writeEvent(newTrackEvent(&trackEvent{
				Type:      drumNoteOnEvent,
				ByteData1: msg[1],
				ByteData2: msg[2],
			}, k.keymap), p)
		}
	} else if msg[0]&0xf0 == 0x80 || (msg[0]&0xf0 == 0x90 && msg[2] == 0) { // note off
		if msg[1] == k.lastMidi {
			pe.playSelectionNoteOff(p)
			k.lastMidi = byteNil
		}
	}
}

// convert a key string to an absolute pitch
func (k *keymap) pitchFromString(s string, refPitch float64) (float64, bool) {
	if ki := k.getByKey(s); ki != nil {
		pitch := ki.Interval + refPitch
		if pitch < minPitch {
			pitch = minPitch
		} else if pitch > maxPitch {
			pitch = maxPitch
		}
		return pitch, true
	}
	return 0, false
}
