package main

import (
	"bufio"
	"bytes"
	"encoding/csv"
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
	lowerOctaveKeys = make([]string, 19) // set in init()
	upperOctaveKeys = make([]string, 19) // set in init()
	isoCenterX      = 4
	isoCenterY      = 2

	midiRegexp = regexp.MustCompile(`^m(\d+)$`)
)

func init() {
	for i := 0; i < 19; i++ {
		lowerOctaveKeys[i] = qwertyLayout[3-(i%2)][(i+1)/2]
		upperOctaveKeys[i] = qwertyLayout[1-(i%2)][(i+1)/2]
	}
}

// turns key events into note events
type keymap struct {
	Name  string
	Items []*keyInfo

	midimap     [128]float64
	isPerc      bool
	keyNotes    map[string]*trackEvent // map of keys to note on events
	midiNotes   [128]*trackEvent       // map of midi notes to note on events
	activeNotes [24]bool
	keySig      map[float64]*pitchSrc
}

// return a copy of a keySig map
func copyKeySig(in map[float64]*pitchSrc) map[float64]*pitchSrc {
	out := make(map[float64]*pitchSrc)
	for k, v := range in {
		out[k] = v.add(newSemiPitch(0))
	}
	return out
}

// an entry in a keymap
type keyInfo struct {
	Key      string
	IsMod    bool
	Interval float64 // kept only for backwards compatibility, use semitones()
	Name     string
	PitchSrc *pitchSrc
}

// initialize a new key. does not report errors for bad src.
func newKeyInfo(key string, isMod bool, name string, src *pitchSrc) *keyInfo {
	ki := &keyInfo{
		Key:      key,
		IsMod:    isMod,
		Name:     name,
		PitchSrc: src,
	}
	return ki
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
	var errs []string
	k := newEmptyKeymap(strings.Replace(filepath.Base(path), ".csv", "", 1))
	if records, err := readCSV(joinTreePath(keymapPath, path), false); err == nil {
		errs = k.applyRecords(records)
	} else if records, err := readCSV(filepath.Join(keymapPath, path), true); err == nil {
		errs = k.applyRecords(records)
	} else {
		k.Name = "none"
		return k, err
	}
	k.duplicateOctave(newRatPitch(2, 1))
	k.setMidiPattern()
	if len(errs) > 0 {
		return k, errors.New(strings.Join(errs, "\n"))
	}
	return k, nil
}

// apply CSV records
func (k *keymap) applyRecords(records [][]string) []string {
	errs := []string{}
	for _, rec := range records {
		ok := false
		if len(rec) == 3 {
			if pitch, err := parsePitch(rec[2], k); err == nil {
				name := rec[1]
				k.Items = append(k.Items,
					newKeyInfo(rec[0], strings.HasPrefix(rec[2], "*"), name, pitch))
				ok = true
			}
		}
		if !ok {
			errs = append(errs, fmt.Sprintf("bad keymap record: %q", rec))
		}
	}
	return errs
}

// initialize a new empty keymap
func newEmptyKeymap(name string) *keymap {
	return &keymap{
		Name:     name,
		keyNotes: make(map[string]*trackEvent),
		keySig:   make(map[float64]*pitchSrc),
	}
}

// write a keymap to a file
func (k *keymap) write(path string) error {
	return writeCSV(joinTreePath(keymapPath, path), k.genRecords())
}

// return a string representation of the keymap
func (k *keymap) String() string {
	var b bytes.Buffer
	csv.NewWriter(&b).WriteAll(k.genRecords())
	return strings.Trim(b.String(), "\n")
}

// generate CSV records from current keymap
func (k *keymap) genRecords() [][]string {
	records := [][]string{}
	octavesAreDuplicated := k.areOctavesDuplicated()
	for _, ki := range k.Items {
		if !(octavesAreDuplicated && stringInSlice(ki.Key, lowerOctaveKeys)) {
			prefix := ""
			if ki.IsMod {
				prefix = "*"
			}
			records = append(records, []string{
				ki.Key,
				ki.Name,
				prefix + ki.PitchSrc.String(),
			})
		}
	}
	return records
}

// return true if high octave and low octave keys are identical separated by an
// octave
func (k *keymap) areOctavesDuplicated() bool {
	octave := newRatPitch(2, 1)
	for i, key := range lowerOctaveKeys {
		lo := k.getByKey(key)
		hi := k.getByKey(upperOctaveKeys[i])
		if (lo == nil) != (hi == nil) {
			return false
		} else if lo != nil && *lo.PitchSrc.add(octave) != *hi.PitchSrc {
			return false
		}
	}
	return true
}

// return true if s is in a
func stringInSlice(s string, a []string) bool {
	for _, v := range a {
		if v == s {
			return true
		}
	}
	return false
}

// duplicate Q-0 keys in Z-; if matching Z-; keys are free
func (k *keymap) duplicateOctave(interval *pitchSrc) {
	interval = interval.invert()
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
				qwertyLayout[y+2][x], ki.IsMod, "", ki.PitchSrc.add(interval)))
		}
	}
}

// generate the midi mappings from the existing keyInfo items
func (k *keymap) setMidiPattern() {
	firstMidi, lastMidi := -1, -1
	for _, ki := range k.Items {
		if midiRegexp.MatchString(ki.Key) {
			if i, err := strconv.ParseUint(ki.Key[1:], 10, 8); err == nil && i < 128 {
				k.midimap[i] = ki.PitchSrc.semitones()
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
	f, err := os.Open(joinTreePath(keymapPath, path))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var scale []*pitchSrc
	scanner := bufio.NewScanner(f)
	i := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "!") {
			if i == 1 {
				if n, err := strconv.ParseUint(line, 10, 16); err == nil {
					scale = make([]*pitchSrc, n+1)
					scale[0] = newRatPitch(1, 1)
				} else {
					return nil, fmt.Errorf("invalid scale file")
				}
			} else if i > 1 && i-2 < len(scale) {
				if pitch, err := parseScalaPitch(line); err == nil {
					scale[i-1] = pitch
				} else {
					return nil, fmt.Errorf("invalid scale file")
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

// convert a scala pitch string into a pitch struct
func parseScalaPitch(s string) (*pitchSrc, error) {
	if m := ratioRegexp.FindAllStringSubmatch(s, 1); m != nil {
		num, _ := strconv.ParseUint(m[0][1], 10, 64)
		den, _ := strconv.ParseUint(m[0][2], 10, 64)
		return newRatPitch(int(num), int(den)), nil
	}
	f, err := strconv.ParseFloat(s, 64)
	return newSemiPitch(f / 100), err
}

// generate a keymap for an equal division of an interval. this gets you full
// coverage on computer keyboard for up to 38edo. (sorry, 41edo enthusiasts.
// there's not enough keys. midi should work up to 127edo.)
func genEqualDivisionKeymap(interval float64, n int) *keymap {
	scale := make([]*pitchSrc, n+1)
	for i := 0; i < n+1; i++ {
		scale[i] = newEdxPitch(interval, i, n)
	}
	k := genScaleKeymap(fmt.Sprintf("%ded%s", n, getEdxChar(interval)), scale)
	k.duplicateOctave(newSemiPitch(interval))
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
func genRank2Keymap(per, gen *pitchSrc, n int) (*keymap, error) {
	if math.Mod(12, per.semitones()) != 0 {
		return nil, fmt.Errorf("octave must be divisible by period")
	}
	nPeriods := int(12 / per.semitones())
	scale := make([]*pitchSrc, n+1)
	for i := 0; i < n/nPeriods; i++ {
		for j := 0; j < nPeriods; j++ {
			scale[i+j*n/nPeriods] = gen.multiply(i).modulo(per).add(per.multiply(j))
		}
	}
	scale[len(scale)-1] = scale[0].add(newSemiPitch(12))
	sort.Slice(scale, func(i, j int) bool {
		return scale[i].semitones() < scale[j].semitones()
	})
	k := genScaleKeymap("gen-rank2", scale)
	k.duplicateOctave(scale[len(scale)-1])
	k.setMidiPattern()
	return k, nil
}

// helper function for genEqualDivisionKeymap and genRank2Keymap
// scale should contain both the identity and the period (ex, 1/1 and 2/1)
func genScaleKeymap(name string, scale []*pitchSrc) *keymap {
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
				fmt.Sprintf("%d'", (i%n)+1), scale[i]))
		} else {
			if x >= w*2-1 {
				break
			}
			y = 1 - (x % 2)
			// Q2W3 ascending
			k.Items = append(k.Items, newKeyInfo(qwertyLayout[y][(x+1)/2], false,
				fmt.Sprintf("%d'", (i%n)+1), scale[i]))
			if n >= w*h/2 {
				// /;.L descending
				k.Items = append(k.Items, newKeyInfo(qwertyLayout[y+2][w-x/2-1], false,
					fmt.Sprintf("%d'", ((n-i-1)%n)+1),
					scale[int(posMod(float64(-i-1), float64(n)))].add(newRatPitch(1, 2))))
			}
		}
		x++
	}
	// midi is simpler, but make sure not to duplicate notation
	for i := 0; i <= n; i++ {
		notation := fmt.Sprintf("%d'", (i%n)+1)
		for _, ki := range k.Items {
			if ki.Name == notation {
				notation = ""
				break
			}
		}
		k.Items = append(k.Items, newKeyInfo(fmt.Sprintf("m%d", midiRoot+i), false,
			notation, scale[i]))
	}
	// 1 and A are unused by these layouts, so map them to octaves. this is
	// useful for edos 10 and >18
	k.Items = append(k.Items, newKeyInfo("1", false, "", newRatPitch(2, 1)))
	k.Items = append(k.Items, newKeyInfo("A", false, "", newRatPitch(1, 2)))
	return k
}

// generate a two-dimensional isomorphic keyboard keymap from two intervals
func genIsoKeymap(ps1, ps2 *pitchSrc) *keymap {
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
			fmt.Sprintf("(%d.%d)", x-isoCenterX+y-2, isoCenterY-y),
			ps1.multiply(x-isoCenterX+y-2).add(ps2.multiply((isoCenterY-y))),
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

// convert a string to a pitch struct
func parsePitch(s string, k *keymap) (*pitchSrc, error) {
	s = strings.TrimPrefix(s, "*")
	if m := ratioRegexp.FindAllStringSubmatch(s, 1); m != nil {
		num, _ := strconv.ParseFloat(m[0][1], 64)
		den, _ := strconv.ParseFloat(m[0][2], 64)
		if math.Round(num) == num && math.Round(den) == den {
			return newRatPitch(int(num), int(den)), nil
		}
		return newSemiPitch(12 * math.Log(num/den) / math.Log(2)), nil
	} else if m := edoStepRegexp.FindAllStringSubmatch(s, 1); m != nil {
		step, _ := strconv.ParseFloat(m[0][1], 64)
		edo, _ := strconv.ParseFloat(m[0][2], 64)
		if math.Round(step) == step && math.Round(edo) == edo {
			return newEdxPitch(12, int(step), int(edo)), nil
		}
		return newSemiPitch(12 * step / edo), nil
	} else if m := keyRefRegexp.FindAllStringSubmatch(s, 1); m != nil {
		if ki := k.getByKey(m[0][1]); ki != nil {
			return ki.PitchSrc, nil
		}
		return nil, fmt.Errorf("no key \"%s\" in keymap", m[0][1])
	} else if f, err := strconv.ParseFloat(s, 64); err == nil {
		return newSemiPitch(f), nil
	}
	return nil, fmt.Errorf("invalid pitch syntax")
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
					note, _ := pitchToMidi(pitch, p.song.MidiMode)
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
			k.setActiveNote(te.chordIndex, false)
		}
	}
}

// respond to midi input events
func (k *keymap) midiEvent(msg []byte, pe *patternEditor, p *player, keyjazz bool) {
	if msg[0]&0xf0 == 0x90 && msg[2] > 0 { // note on
		var te *trackEvent
		if sdl.GetModState()&sdl.KMOD_SHIFT == 0 {
			pitch := k.adjustPerKeySig(k.midimap[msg[1]]) + pe.refPitch
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
			k.setActiveNote(te.chordIndex, false)
		}
	}
}

// convert a key string to an absolute pitch
func (k *keymap) pitchFromString(s string, refPitch float64) (float64, bool) {
	if ki := k.getByKey(s); ki != nil {
		pitch := k.adjustPerKeySig(ki.PitchSrc.semitones()) + refPitch
		pitch = math.Max(minPitch, math.Min(maxPitch, pitch))
		return pitch, true
	} else if midiRegexp.MatchString(s) {
		if i, _ := strconv.ParseUint(s[1:], 10, 8); int(i) < len(k.midimap) {
			pitch := k.adjustPerKeySig(k.midimap[i]) + refPitch
			pitch = math.Max(minPitch, math.Min(maxPitch, pitch))
			return pitch, true
		}
	}
	return 0, false
}

// adjust a pitch according to the current key signature
func (k *keymap) adjustPerKeySig(pitch float64) float64 {
	normPitch := posMod(pitch, 12)
	for key, v := range k.keySig {
		if math.Abs(key-normPitch) < 0.01 {
			return pitch + v.semitones()
		}
	}
	return pitch
}

// set the track and write/play a note on / drum note on event as appropriate
func (k *keymap) processKeymapNoteOn(te *trackEvent, pe *patternEditor, p *player, keyjazz bool) {
	trackMin, trackMax, _, _ := pe.getSelection()
	te.chordIndex = k.getFirstFreeChordIndex()
	te.track = trackMin + int(te.chordIndex)
	if te.track > trackMax {
		te.chordIndex -= uint8(te.track - trackMax)
		te.track = trackMax
	}
	k.setActiveNote(te.chordIndex, true)
	for i, v := range k.keyNotes {
		if v.track == te.track {
			delete(k.keyNotes, i)
			k.setActiveNote(v.chordIndex, false)
			break
		}
	}
	for i, v := range k.midiNotes {
		if v != nil && v.track == te.track {
			k.midiNotes[i] = nil
			k.setActiveNote(v.chordIndex, false)
			break
		}
	}
	if keyjazz {
		p.signal <- playerSignal{typ: signalEvent, event: te}
	} else {
		pe.writeEvent(te, p)
	}
}

// get index of first false entry in activeNotes
func (k *keymap) getFirstFreeChordIndex() uint8 {
	for i, v := range k.activeNotes {
		if !v {
			return uint8(i)
		}
	}
	return 0
}

// set activeNotes[i] to v, with bounds checking
func (k *keymap) setActiveNote(i uint8, v bool) {
	if int(i) < len(k.activeNotes) {
		k.activeNotes[i] = v
	}
}

// set all activeNotes values to false
func (k *keymap) clearActiveNotes() {
	for i := range k.activeNotes {
		k.activeNotes[i] = false
	}
}

// return a string with notation for a pitch, or empty if none matched
func (k *keymap) notatePitch(f float64, octave bool) string {
	if s := k.notatePitchHelper(f, false, octave); s != "" {
		return s
	}
	return k.notatePitchHelper(f, true, octave)
}

// helper for notatePitch
func (k *keymap) notatePitchHelper(f float64, auto, octave bool) string {
	// first try to find a match with no accidentals
	if s := k.notatePitchWithMods(f, auto, octave); s != "" {
		return s
	}
	// then try one accidental
	for _, mod1 := range k.Items {
		if mod1.IsMod {
			if s := k.notatePitchWithMods(f, auto, octave, mod1); s != "" {
				return s
			}
		}
	}
	// then try two
	for _, mod1 := range k.Items {
		if mod1.IsMod {
			for _, mod2 := range k.Items {
				if mod2.IsMod {
					if s := k.notatePitchWithMods(f, auto, octave, mod1, mod2); s != "" {
						return s
					}
				}
			}
		}
	}
	return ""
}

var endsWithDigitRegexp = regexp.MustCompile(`\d$`)

// helper function for notatePitch
func (k *keymap) notatePitchWithMods(f float64, auto, octave bool, mods ...*keyInfo) string {
	modString := ""
	for _, mod := range mods {
		f -= mod.PitchSrc.semitones()
		if mod.Name == "" {
			modString += mod.Key
		} else {
			modString += mod.Name
		}
	}
	target := posMod(f, 12)
	for _, ki := range k.Items {
		diff := math.Abs(ki.PitchSrc.class(12) - target)
		if !ki.IsMod && (diff < 0.01 || diff > 11.99) {
			var base string
			if auto {
				base = ki.PitchSrc.String()
			} else if ki.Name != "" {
				base = ki.Name
			} else {
				continue
			}
			if octave {
				digitSpacer := ""
				if endsWithDigitRegexp.MatchString(ki.Name + modString) {
					digitSpacer = "-"
				}
				// add +0.01 to prevent -0.00000001 (or whatever) from being lower than 0
				octave := int(f+0.01) / 12
				return fmt.Sprintf("%s%s%s%d", base, modString, digitSpacer, octave)
			} else {
				return fmt.Sprintf("%s%s", base, modString)
			}
		}
	}
	return ""
}
