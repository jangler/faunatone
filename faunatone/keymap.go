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
		{"A", "S", "D", "F", "G", "H", "J", "K", "L", ";"},
		{"Z", "X", "C", "V", "B", "N", "M", ",", ".", "/"},
	}
	isoCenterX = 4
	isoCenterY = 2

	midiRegexp = regexp.MustCompile(`m(\d+)`)
)

// turns key events into note events
type keymap struct {
	Name  string
	Items []*keyInfo

	midimap   [128]float64
	isPerc    bool
	keyNotes  map[string]*trackEvent // map of keys to note on events
	midiNotes [128]*trackEvent       // map of midi notes to note on events
}

// an entry in a keymap
type keyInfo struct {
	Key      string
	IsMod    bool
	Interval float64
	Name     string
	Origin   string // the way the interval was written originally

	class float64 // like pitch class; derived from Interval
}

// initialize a new key
func newKeyInfo(key string, isMod bool, interval float64, name, origin string) *keyInfo {
	return &keyInfo{
		Key:      key,
		IsMod:    isMod,
		Interval: interval,
		Name:     name,
		Origin:   origin,
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
	k := newEmptyKeymap(strings.Replace(filepath.Base(path), ".csv", "", 1))
	if records, err := readCSV(filepath.Join(keymapPath, path)); err == nil {
		for _, rec := range records {
			ok := false
			if len(rec) == 3 {
				if pitch, err := parsePitch(rec[2], k); err == nil {
					k.Items = append(k.Items, newKeyInfo(
						rec[0], strings.HasPrefix(rec[2], "*"), pitch, rec[1], rec[2]))
					ok = true
				}
			}
			if !ok {
				errs = append(errs, fmt.Sprintf("bad keymap record: %q", rec))
			}
		}
	} else {
		k.Name = "none"
		return k, err
	}
	k.setMidiPattern()
	if len(errs) > 0 {
		return k, errors.New(strings.Join(errs, "\n"))
	}
	return k, nil
}

// initialize a new empty keymap
func newEmptyKeymap(name string) *keymap {
	return &keymap{
		Name:     name,
		keyNotes: make(map[string]*trackEvent),
	}
}

// write a keymap to a file
func (k *keymap) write(path string) error {
	records := make([][]string, len(k.Items))
	for i, ki := range k.Items {
		pitchString := ki.Origin
		if pitchString == "" {
			pitchString = fmt.Sprintf("%f", ki.Interval)
		}
		records[i] = []string{ki.Key, ki.Name, pitchString}
	}
	return writeCSV(filepath.Join(keymapPath, path), records)
}

// generate the midi mappings from the existing keyInfo items
func (k *keymap) setMidiPattern() {
	firstMidi, lastMidi := -1, -1
	for _, ki := range k.Items {
		if midiRegexp.MatchString(ki.Key) {
			if i, err := strconv.ParseUint(ki.Key[1:], 10, 8); err == nil && i < 128 {
				k.midimap[i] = ki.Interval
				if firstMidi == -1 || int(i) < firstMidi {
					firstMidi = int(i)
				}
				if int(i) > lastMidi {
					lastMidi = int(i)
				}
			}
		}
	}
	k.repeatMidiPattern(firstMidi, lastMidi)
}

// repeats the pattern of midi notes already present in the keymap across the
// entire range
// TODO restrict notes to allowable range
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

// generate a keymap for an edo. this gets you full coverage on computer
// keyboard for up to 38edo. (sorry, 41edo enthusiasts. there's not enough
// keys. midi should work up to 127edo.)
func genEdoKeymap(n int) *keymap {
	k := newEmptyKeymap(fmt.Sprintf("%dedo", n))
	w, h := len(qwertyLayout[0]), len(qwertyLayout)
	x, y := 0, 1
	midiRoot := 60
	for i := n; i > 67; i -= 12 {
		midiRoot -= 12
	}
	for i := 0; i <= n && y >= 0; i++ {
		// computer keyboard
		if n <= w {
			// two rows, Q-P and Z-/
			if x >= w {
				break
			}
			k.Items = append(k.Items, newKeyInfo(qwertyLayout[y][x], false,
				12/float64(n)*float64(i), fmt.Sprintf("%d'", (i%int(n))+1),
				fmt.Sprintf("%d\\%d", i, n)))
			k.Items = append(k.Items, newKeyInfo(qwertyLayout[y+2][x], false,
				12/float64(n)*float64(i)-12, fmt.Sprintf("%d'", (i%int(n))+1),
				fmt.Sprintf("%d\\%d", i-n, n)))
		} else if n < w*h/2 {
			// two sets of two alternating rows, Q2W3... and ZSXD...
			if x >= w*2-1 {
				break
			}
			y = 1 - (x % 2)
			k.Items = append(k.Items, newKeyInfo(qwertyLayout[y][(x+1)/2], false,
				12/float64(n)*float64(i), fmt.Sprintf("%d'", (i%int(n))+1),
				fmt.Sprintf("%d\\%d", i, n)))
			k.Items = append(k.Items, newKeyInfo(qwertyLayout[y+2][(x+1)/2], false,
				12/float64(n)*float64(i)-12, fmt.Sprintf("%d'", (i%int(n))+1),
				fmt.Sprintf("%d\\%d", i-n, n)))
		} else {
			// Q2W3 same as above, then go backwards down /;.L
			if x >= w*2-1 {
				break
			}
			y = 1 - (x % 2)
			k.Items = append(k.Items, newKeyInfo(qwertyLayout[y][(x+1)/2], false,
				12/float64(n)*float64(i), fmt.Sprintf("%d'", (i%int(n))+1),
				fmt.Sprintf("%d\\%d", i, n)))
			k.Items = append(k.Items, newKeyInfo(qwertyLayout[y+2][w-x/2-1], false,
				-12/float64(n)*float64(i+1), fmt.Sprintf("%d'", ((n-i-1)%int(n))+1),
				fmt.Sprintf("%d\\%d", n-i-1, n)))
		}
		// midi is simpler
		if i <= n {
			k.Items = append(k.Items, newKeyInfo(fmt.Sprintf("m%d", midiRoot+i), false,
				12/float64(n)*float64(i), fmt.Sprintf("%d'", (i%int(n))+1),
				fmt.Sprintf("%d\\%d", i, n)))
		}
		x++
	}
	// 1 and A are unused by these layouts, so map them to octaves. this is
	// useful for edos 10 and >18
	k.Items = append(k.Items, newKeyInfo("1", false,
		12, "1'", fmt.Sprintf("%d\\%d", n, n)))
	k.Items = append(k.Items, newKeyInfo("A", false,
		-12, "1'", fmt.Sprintf("%d\\%d", -n, n)))
	k.setMidiPattern()
	return k
}

// generate a two-dimensional isomorphic keyboard keymap from two intervals
func genIsoKeymap(interval1, interval2 float64) *keymap {
	k := newEmptyKeymap("gen-iso")
	for y, row := range qwertyLayout {
		for x, key := range row {
			k.Items = append(k.Items, newKeyInfo(key, false,
				interval1*float64(x-isoCenterX+y-2)+interval2*float64(isoCenterY-y),
				fmt.Sprintf("(%d,%d)", x-isoCenterX+y-2, isoCenterY-y), "",
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
	for _, ki := range k.Items {
		if ki.Key == key {
			return ki
		}
	}
	return nil
}

// respond to keyboard events
func (k *keymap) keyboardEvent(e *sdl.KeyboardEvent, pe *patternEditor, p *player, keyjazz bool) {
	shiftPressed := e.Keysym.Mod&sdl.KMOD_SHIFT != 0
	if e.Repeat != 0 {
		return
	}
	s := strings.Replace(formatKeyEvent(e, true), "Shift+", "", 1)
	if pitch, ok := k.pitchFromString(s, pe.refPitch); ok {
		if e.State == sdl.PRESSED {
			if k.getByKey(s).IsMod {
				if shiftPressed {
					pe.modifyRefPitch(pitch - pe.refPitch)
				} else {
					pe.transposeSelection(pitch - pe.refPitch)
				}
			} else if shiftPressed == k.isPerc {
				var te *trackEvent
				if !k.isPerc {
					te = newTrackEvent(&trackEvent{
						Type:      noteOnEvent,
						FloatData: pitch,
						ByteData1: pe.velocity,
					}, k)
				} else {
					note, _ := pitchToMIDI(pitch)
					te = newTrackEvent(&trackEvent{
						Type:      drumNoteOnEvent,
						ByteData1: note,
						ByteData2: pe.velocity,
					}, k)
				}
				processKeymapNoteOn(te, pe, p, keyjazz)
				k.keyNotes[s] = te
			}
		} else if te, ok := k.keyNotes[s]; ok {
			p.signal <- playerSignal{typ: signalEvent, event: &trackEvent{
				Type:  noteOffEvent,
				track: te.track,
			}}
			delete(k.keyNotes, s)
		}
	}
}

// respond to midi input events
func (k *keymap) midiEvent(msg []byte, pe *patternEditor, p *player, keyjazz bool) {
	if msg[0]&0xf0 == 0x90 && msg[2] > 0 { // note on
		var te *trackEvent
		if sdl.GetModState()&sdl.KMOD_SHIFT == 0 {
			pitch := k.midimap[msg[1]] + pe.refPitch
			te = newTrackEvent(&trackEvent{
				Type:      noteOnEvent,
				FloatData: pitch,
				ByteData1: msg[2],
			}, k)
		} else {
			te = newTrackEvent(&trackEvent{
				Type:      drumNoteOnEvent,
				ByteData1: msg[1],
				ByteData2: msg[2],
			}, k)
		}
		processKeymapNoteOn(te, pe, p, keyjazz)
		k.midiNotes[msg[1]] = te
	} else if msg[0]&0xf0 == 0x80 || (msg[0]&0xf0 == 0x90 && msg[2] == 0) { // note off
		if te := k.midiNotes[msg[1]]; te != nil {
			p.signal <- playerSignal{typ: signalEvent, event: &trackEvent{
				Type:  noteOffEvent,
				track: te.track,
			}}
			k.midiNotes[msg[1]] = nil
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

// set the track and write/play a note on / drum note on event as appropriate
func processKeymapNoteOn(te *trackEvent, pe *patternEditor, p *player, keyjazz bool) {
	te.track = -1
	te.trackMin, te.trackMax, _, _ = pe.getSelection()
	if keyjazz {
		p.signal <- playerSignal{typ: signalEvent, event: te}
	} else {
		pe.writeEvent(te, p)
	}
}
