package main

import (
	"bufio"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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

	midimap     [128]float64
	isPerc      bool
	keyNotes    map[string]*trackEvent // map of keys to note on events
	midiNotes   [128]*trackEvent       // map of midi notes to note on events
	activeNotes int
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
					name := rec[1]
					if name == "" {
						name = rec[2] + "-" // use origin as name if name is absent
					}
					k.Items = append(k.Items, newKeyInfo(
						rec[0], strings.HasPrefix(rec[2], "*"), pitch, name, rec[2]))
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
	k.duplicateOctave(12)
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

// duplicate Q-0 keys in Z-; if matching Z-; keys are free
func (k *keymap) duplicateOctave(interval float64) {
	for i := 0; i < 19; i++ {
		x, y := (i+1)/2, 1-(i%2)
		if k.getByKey(qwertyLayout[y][x]) != nil && k.getByKey(qwertyLayout[y+2][x]) != nil {
			return // Z-; equivalents not free
		}
	}
	for i := 0; i < 19; i++ {
		x, y := (i+1)/2, 1-(i%2)
		if ki := k.getByKey(qwertyLayout[y][x]); ki != nil {
			k.Items = append(k.Items, newKeyInfo(
				qwertyLayout[y+2][x], ki.IsMod, ki.Interval-interval, ki.Name, ""))
		}
	}
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

// convert a scala .scl file into a keymap
func keymapFromSclFile(path string) (*keymap, error) {
	f, err := os.Open(filepath.Join(keymapPath, path))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var scale []float64
	scanner := bufio.NewScanner(f)
	i := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "!") {
			if i == 1 {
				if n, err := strconv.ParseUint(line, 10, 16); err == nil {
					scale = make([]float64, n+1)
				} else {
					return nil, fmt.Errorf("Invalid scale file.")
				}
			} else if i > 1 && i-2 < len(scale) {
				if pitch, err := parseScalaPitch(line); err == nil {
					scale[i-1] = pitch
				} else {
					return nil, fmt.Errorf("Invalid scale file.")
				}
			}
			i++
		}
	}
	k := genScaleKeymap(strings.Replace(filepath.Base(path), ".scl", "", 1), scale)
	k.duplicateOctave(scale[len(scale)-1])
	k.setMidiPattern()
	return k, err
}

// convert a scala pitch string into a midi interval
func parseScalaPitch(s string) (float64, error) {
	if m := ratioRegexp.FindAllStringSubmatch(s, 1); m != nil {
		num, _ := strconv.ParseFloat(m[0][1], 64)
		den, _ := strconv.ParseFloat(m[0][2], 64)
		return 12 * math.Log(num/den) / math.Log(2), nil
	}
	f, err := strconv.ParseFloat(s, 64)
	return f / 100, err
}

// generate a keymap for an equal division of an interval. this gets you full
// coverage on computer keyboard for up to 38edo. (sorry, 41edo enthusiasts.
// there's not enough keys. midi should work up to 127edo.)
func genEqualDivisionKeymap(interval float64, n int) *keymap {
	scale := make([]float64, n+1)
	for i := 0; i < n+1; i++ {
		scale[i] = interval / float64(n) * float64(i)
	}
	k := genScaleKeymap(fmt.Sprintf("%ded%s", n, getEdxChar(interval)), scale)
	k.duplicateOctave(interval)
	k.setMidiPattern()
	return k
}

// return o for octave, f for fifth, etc, x for other
func getEdxChar(interval float64) string {
	if interval == 12 {
		return "o"
	} else if math.Abs(math.Mod(interval, 12)) < 0.01 {
		return fmt.Sprintf("%.f", math.Pow(2, interval/12))
	} else if math.Abs(interval-7.02) < 0.01 {
		return "f"
	} else if math.Abs(interval-19.02) < 0.01 {
		return "t"
	}
	return "x"
}

// generate a keymap for a rank-2 temperament scale
func genRank2Keymap(per, gen float64, n int) (*keymap, error) {
	if math.Mod(12, per) != 0 {
		return nil, fmt.Errorf("Octave must be divisible by period.")
	}
	nPeriods := int(12 / per)
	scale := make([]float64, n+1)
	for i := 0; i < n/nPeriods; i++ {
		for j := 0; j < nPeriods; j++ {
			scale[i+j*n/nPeriods] = math.Mod(gen*float64(i), per) + per*float64(j)
		}
	}
	scale[len(scale)-1] = 12
	sort.Float64s(scale)
	k := genScaleKeymap("gen-rank2", scale)
	k.duplicateOctave(12)
	k.setMidiPattern()
	return k, nil
}

// helper function for genEqualDivisionKeymap and genRank2Keymap
// scale should contain both the identity and the period (ex, 1/1 and 2/1)
func genScaleKeymap(name string, scale []float64) *keymap {
	k := newEmptyKeymap(name)
	n := len(scale) - 1
	w, h := len(qwertyLayout[0]), len(qwertyLayout)
	x, y := 0, 1
	midiRoot := 60
	for i := n; i > 67; i -= 12 {
		midiRoot -= 12
	}
	// computer keyboard
	for i := 0; i <= n && y >= 0; i++ {
		if n <= w {
			// two rows, Q-P and Z-/ (second row set by duplicateOctave)
			if x >= w {
				break
			}
			k.Items = append(k.Items, newKeyInfo(qwertyLayout[y][x], false,
				scale[i], fmt.Sprintf("%d'", (i%int(n))+1), ""))
		} else if n < w*h/2 {
			// two sets of two alternating rows, Q2W3... and ZSXD...
			// (second row set by duplicateOctave)
			if x >= w*2-1 {
				break
			}
			y = 1 - (x % 2)
			k.Items = append(k.Items, newKeyInfo(qwertyLayout[y][(x+1)/2], false,
				scale[i], fmt.Sprintf("%d'", (i%int(n))+1), ""))
		} else {
			// Q2W3 same as above, then go backwards down /;.L
			if x >= w*2-1 {
				break
			}
			y = 1 - (x % 2)
			k.Items = append(k.Items, newKeyInfo(qwertyLayout[y][(x+1)/2], false,
				scale[i], fmt.Sprintf("%d'", (i%int(n))+1), ""))
			k.Items = append(k.Items, newKeyInfo(qwertyLayout[y+2][w-x/2-1], false,
				-scale[i+1], fmt.Sprintf("%d'", ((n-i-1)%int(n))+1), ""))
		}
		x++
	}
	// midi is simpler
	for i := 0; i <= n; i++ {
		k.Items = append(k.Items, newKeyInfo(fmt.Sprintf("m%d", midiRoot+i), false,
			scale[i], fmt.Sprintf("%d'", (i%int(n))+1), ""))
	}
	// 1 and A are unused by these layouts, so map them to octaves. this is
	// useful for edos 10 and >18
	k.Items = append(k.Items, newKeyInfo("1", false, 12, "1'", "2/1"))
	k.Items = append(k.Items, newKeyInfo("A", false, -12, "1'", "1/2"))
	return k
}

// generate a two-dimensional isomorphic keyboard keymap from two intervals
func genIsoKeymap(interval1, interval2 float64) *keymap {
	k := newEmptyKeymap("gen-iso")
	vectors := make([][3]int, len(qwertyLayout)*len(qwertyLayout[0]))
	i := 0
	for y, row := range qwertyLayout {
		for x := range row {
			vectors[i] = [3]int{x, y, intAbs(x-isoCenterX+y-2) + intAbs(isoCenterY-y)}
			i++
		}
	}
	// sort by distance from root so that simpler vectors are used for notation
	sort.Slice(vectors, func(i, j int) bool {
		return vectors[i][2] < vectors[j][2]
	})
	for _, v := range vectors {
		x, y := v[0], v[1]
		key := qwertyLayout[y][x]
		k.Items = append(k.Items, newKeyInfo(key, false,
			interval1*float64(x-isoCenterX+y-2)+interval2*float64(isoCenterY-y),
			fmt.Sprintf("(%d,%d)", x-isoCenterX+y-2, isoCenterY-y), "",
		))
	}
	return k
}

// return absolute value of a
func intAbs(a int) int {
	if a < 0 {
		return -a
	}
	return a
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
				k.processKeymapNoteOn(te, pe, p, keyjazz)
				k.keyNotes[s] = te
			}
		} else if te, ok := k.keyNotes[s]; ok {
			p.signal <- playerSignal{typ: signalEvent, event: &trackEvent{
				Type:  noteOffEvent,
				track: te.track,
			}}
			delete(k.keyNotes, s)
			k.activeNotes--
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
		k.processKeymapNoteOn(te, pe, p, keyjazz)
		k.midiNotes[msg[1]] = te
	} else if msg[0]&0xf0 == 0x80 || (msg[0]&0xf0 == 0x90 && msg[2] == 0) { // note off
		if te := k.midiNotes[msg[1]]; te != nil {
			p.signal <- playerSignal{typ: signalEvent, event: &trackEvent{
				Type:  noteOffEvent,
				track: te.track,
			}}
			k.midiNotes[msg[1]] = nil
			k.activeNotes--
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
func (k *keymap) processKeymapNoteOn(te *trackEvent, pe *patternEditor, p *player, keyjazz bool) {
	trackMin, trackMax, _, _ := pe.getSelection()
	te.track = trackMin + k.activeNotes
	if te.track > trackMax {
		te.track = trackMax
	}
	for i, v := range k.keyNotes {
		if v.track == te.track {
			delete(k.keyNotes, i)
			k.activeNotes--
			break
		}
	}
	for i, v := range k.midiNotes {
		if v != nil && v.track == te.track {
			k.midiNotes[i] = nil
			k.activeNotes--
			break
		}
	}
	k.activeNotes++
	if keyjazz {
		p.signal <- playerSignal{typ: signalEvent, event: te}
	} else {
		pe.writeEvent(te, p)
	}
}

// return a string with notation for a pitch, or empty if none matched
func (k *keymap) notatePitch(f float64) string {
	if s := k.notatePitchWithMods(f); s != "" {
		return s
	}
	for _, mod1 := range k.Items {
		if mod1.IsMod {
			if s := k.notatePitchWithMods(f, mod1); s != "" {
				return s
			}
			for _, mod2 := range k.Items {
				if mod2.IsMod {
					if s := k.notatePitchWithMods(f, mod1, mod2); s != "" {
						return s
					}
				}
			}
		}
	}
	return ""
}

// helper function for notatePitch
func (k *keymap) notatePitchWithMods(f float64, mods ...*keyInfo) string {
	modString := ""
	for _, mod := range mods {
		f -= mod.Interval
		modString += mod.Name
	}
	target := posMod(f, 12)
	for _, ki := range k.Items {
		if !ki.IsMod && ki.Name != "" && math.Abs(ki.class-target) < 0.01 {
			return fmt.Sprintf("%s%s%d", ki.Name, modString, int(f)/12)
		}
	}
	return ""
}
