package main

import (
	"errors"
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/veandco/go-sdl2/sdl"
)

const (
	maxDirNames        = 1000
	inputCursorBlinkMs = 500
	border             = 2
)

var (
	alphaRegexp = regexp.MustCompile("[A-Za-z]")
)

type tabTarget struct {
	display, value string
}

// modal dialog that displays a message or prompts for input
type dialog struct {
	prompt []string
	input  string
	size   int          // text box will have room for this many chars
	action func(string) // run if dialog is closed with K_RETURN
	shown  bool
	accept bool // accept input
	mode   inputMode
	dir    string // base dir for path input, used for tab complete
	ext    string // extension for path input completion if non-empty

	// for display and tab completion
	targets    []*tabTarget // all possible targets
	curTargets []*tabTarget // filtered by input

	// for keysig input mode
	keymap      *keymap
	keySig      map[float64]*pitchSrc
	keySigNotes []float64
}

// determines how dialog input works
type inputMode uint8

const (
	textInput inputMode = iota
	noteInput
	yesNoInput
	keySigInput
)

// create a new dialog
func newDialog(prompt string, size int, action func(string)) *dialog {
	return &dialog{
		prompt: strings.Split(prompt, "\n"),
		size:   size,
		action: action,
		shown:  true,
	}
}

// set d to a message dialog
func (d *dialog) message(s string) {
	*d = *newDialog(s, 0, nil)
}

// set d to a message dialog if err is non-nil
func (d *dialog) messageIfErr(err error) {
	if err != nil {
		d.message(err.Error())
	}
}

// set d to an integer dialog that checks for range and syntax errors
func (d *dialog) getInt(prompt string, min, max int64, action func(int64)) {
	size := intMax(len(fmt.Sprintf("%d", min)), len(fmt.Sprintf("%d", max)))
	*d = *newDialog(prompt, size, func(s string) {
		if i, err := strconv.ParseInt(s, 10, 64); err == nil && i >= min && i <= max {
			action(i)
		} else if err != nil && errors.Is(err, strconv.ErrSyntax) {
			d.message("Invalid syntax.")
		} else if i < min || i > max || errors.Is(err, strconv.ErrRange) {
			d.message(fmt.Sprintf("Value must be in range [%d, %d].", min, max))
		} else {
			d.message(err.Error())
		}
	})
}

// set d to an integer dialog with completion targets
func (d *dialog) getNamedInts(
	prompt string, offsets []int64, targets []*tabTarget, action func([]int64)) {
	size := 3
	for _, t := range targets {
		n := len(t.display) + 1
		if n > size {
			size = n
		}
	}
	*d = *newDialog(prompt, size, func(s string) {
		errString := ""
		ints := []int64{}

		if len(d.curTargets) > 0 && alphaRegexp.MatchString(s) {
			s = d.curTargets[0].value
		}
		for i, token := range strings.Split(s, " ") {
			if i > len(offsets) {
				continue
			}
			min, max := offsets[i], 127+offsets[i]
			if n, err := strconv.ParseInt(token, 10, 64); err == nil && n >= min && n <= max {
				ints = append(ints, n)
			} else if err != nil && errors.Is(err, strconv.ErrSyntax) {
				errString = "Invalid syntax."
			} else if n < min || n > max || errors.Is(err, strconv.ErrRange) {
				errString = fmt.Sprintf("Value must be in range [%d, %d].", min, max)
			} else {
				errString = err.Error()
			}
		}
		if len(ints) == 0 {
			errString = "Invalid syntax."
		}
		if errString == "" {
			action(ints)
		} else {
			d.message(errString)
		}
	})
	d.targets, d.curTargets = targets, targets
}

// return the larger of two integers
func intMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// return the smaller of two integers
func intMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// set d to a float dialog that checks for range and syntax errors
func (d *dialog) getFloat(prompt string, min, max float64, action func(float64)) {
	*d = *newDialog(prompt, 8, func(s string) {
		if f, err := strconv.ParseFloat(s, 64); err == nil && f >= min && f <= max {
			action(f)
		} else if err != nil && errors.Is(err, strconv.ErrSyntax) {
			d.message("Invalid syntax.")
		} else if f < min || f > max || errors.Is(err, strconv.ErrRange) {
			d.message(fmt.Sprintf("Value must be in range [%.2f, %.2f].", min, max))
		} else {
			d.message(err.Error())
		}
	})
}

// set d to an interval dialog that checks for syntax errors
func (d *dialog) getInterval(prompt string, k *keymap, action func(*pitchSrc)) {
	*d = *newDialog(prompt, 10, func(s string) {
		if ps, err := parsePitch(s, k); err == nil {
			action(ps)
		} else {
			d.message(err.Error())
		}
	})
}

// set d to a file path dialog that allows for path tab completion
func (d *dialog) getPath(
	prompt, dir, ext string, requireExists bool, action func(string)) {
	*d = *newDialog(prompt, 50, func(s string) {
		if requireExists && len(d.curTargets) > 0 {
			s = d.curTargets[0].value
		}
		action(s)
	})
	d.dir, d.ext = joinTreePath(dir), ext
	d.targets = pathTargets(dir, ext)
	d.curTargets = d.targets
}

// return path targets for the given directory and filename extension
func pathTargets(dir, ext string) []*tabTarget {
	ts := []*tabTarget{}
	if f, err := os.Open(dir); err == nil {
		if names, err := f.Readdirnames(maxDirNames); err == nil {
			for _, name := range names {
				if ext == "" || strings.HasSuffix(name, ext) {
					ts = append(ts, &tabTarget{display: name, value: name})
				}
			}
		}
	}
	return ts
}

// draw the dialog
func (d *dialog) draw(p *printer, r *sdl.Renderer) {
	if !d.shown {
		return
	}

	// get displayed position and size
	viewport := r.GetViewport()
	promptWidth := int32(0)
	for _, line := range d.prompt {
		if p.rect.W*int32(len(line)) > promptWidth {
			promptWidth = p.rect.W * int32(len(line))
		}
	}
	var h, cols int32
	for cols == 0 || h+border*2 > viewport.H {
		cols++
		h = (p.rect.H+padding)*int32(math.Ceil(float64(len(d.prompt))/float64(cols))) + padding
		if d.size > 0 {
			h += p.rect.H + padding*2
		}
	}
	maxDisplayedTargets := int(viewport.H/(p.rect.H+padding)) - 7
	if maxDisplayedTargets > 20 {
		maxDisplayedTargets = 20
	}
	if d.targets != nil {
		h += (p.rect.H+padding)*int32(intMin(maxDisplayedTargets, len(d.targets))) +
			padding*2
	}
	inputWidth := p.rect.W * int32(d.size)
	w := (promptWidth+padding)*cols + padding
	if inputWidth+padding > promptWidth {
		w = inputWidth + padding*3
	}
	rect := &sdl.Rect{viewport.W/2 - w/2, viewport.H/2 - h/2, w, h}

	// draw box and prompt
	r.SetDrawColorArray(colorFgArray...)
	r.FillRect(&sdl.Rect{rect.X - border, rect.Y - border, rect.W + border*2, rect.H + border*2})
	r.SetDrawColorArray(colorBg1Array...)
	r.FillRect(rect)
	y := rect.Y + padding
	linesPerCol := int(math.Ceil(float64(len(d.prompt)) / float64(cols)))
	for i, line := range d.prompt {
		if i%linesPerCol == 0 {
			y = rect.Y + padding
		}
		colOffset := int32(i/linesPerCol) * (promptWidth + padding)
		p.draw(r, line, viewport.W/2-(promptWidth*cols)/2-(padding*(cols-1))/2+colOffset, y)
		y += p.rect.H + padding
	}

	// draw input
	if d.size > 0 {
		r.SetDrawColorArray(colorBg2Array...)
		r.FillRect(&sdl.Rect{viewport.W/2 - inputWidth/2 - padding/2, y,
			inputWidth + padding, p.rect.H + padding})
		s := d.input
		if len(d.input) < d.size && (time.Now().UnixMilli()/inputCursorBlinkMs)%2 == 0 {
			s += "_"
		}
		p.draw(r, s, viewport.W/2-inputWidth/2, rect.Y+p.rect.H+padding*5/2)
	}

	// draw completion targets
	y += p.rect.H + padding*3
	for i, t := range d.curTargets {
		s := t.display
		if i >= maxDisplayedTargets {
			break
		} else if i == maxDisplayedTargets-1 &&
			len(d.curTargets) > maxDisplayedTargets {
			s = "..."
		}
		if len(s) > d.size && d.size >= 3 {
			s = s[:d.size-3] + "..."
		}
		p.draw(r, s, viewport.W/2-inputWidth/2, y)
		y += p.rect.H + padding
	}
}

// respond to text input events
func (d *dialog) textInput(e *sdl.TextInputEvent) {
	text := e.GetText()
	if d.accept && d.mode == textInput && len(d.input)+len(text) <= d.size {
		d.input += e.GetText()
		d.updateCurTargets()
	}
}

// respond to keyboard events
func (d *dialog) keyboardEvent(e *sdl.KeyboardEvent) {
	// ignore key release
	if e.State != sdl.PRESSED {
		return
	}

	switch d.mode {
	case textInput:
		switch e.Keysym.Sym {
		case sdl.K_BACKSPACE:
			if e.Keysym.Mod&sdl.KMOD_CTRL != 0 {
				d.input = ""
				d.updateCurTargets()
			} else if len(d.input) > 0 {
				_, size := utf8.DecodeLastRuneInString(d.input)
				d.input = d.input[:len(d.input)-size]
				d.updateCurTargets()
			}
		case sdl.K_ESCAPE:
			d.shown = false
		case sdl.K_RETURN:
			d.shown = false
			if d.action != nil {
				d.action(d.input)
			}
		case sdl.K_TAB:
			if d.curTargets != nil {
				d.tryPathComplete()
			}
		}
	case noteInput:
		switch e.Keysym.Sym {
		case sdl.K_ESCAPE:
			d.shown = false
		case sdl.K_LSHIFT, sdl.K_RSHIFT, sdl.K_LCTRL, sdl.K_RCTRL, sdl.K_LALT, sdl.K_RALT,
			sdl.K_LGUI, sdl.K_RGUI:
			// don't react to modifier keys
		default:
			d.shown = false
			if d.action != nil {
				d.action(formatKeyEvent(e, true))
			}
		}
	case yesNoInput:
		switch e.Keysym.Sym {
		case sdl.K_ESCAPE, sdl.K_n:
			d.shown = false
		case sdl.K_RETURN, sdl.K_y:
			d.shown = false
			if d.action != nil {
				d.action(d.input)
			}
		}
	case keySigInput:
		switch e.Keysym.Sym {
		case sdl.K_ESCAPE:
			d.shown = false
		case sdl.K_RETURN:
			d.shown = false
			if d.action != nil {
				d.action(d.input)
			}
		case sdl.K_LSHIFT, sdl.K_RSHIFT, sdl.K_LCTRL, sdl.K_RCTRL, sdl.K_LALT, sdl.K_RALT,
			sdl.K_LGUI, sdl.K_RGUI:
			// don't react to modifier keys
		default:
			if ki := d.keymap.getByKey(formatKeyEvent(e, true)); ki != nil {
				d.handleKeySigKey(ki.PitchSrc, ki.IsMod)
			}
		}
	}
}

func canBeWord(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

func canStartWord(r rune) bool {
	return unicode.IsUpper(r) || unicode.IsDigit(r)
}

// split s into words by whitespace, capitalization, or punctuation
func splitWords(s string) []string {
	words := []string{}
	curWord := []rune{}
	prevRune := ' '
	for i, r := range s {
		if i > 0 {
			if (canStartWord(r) && !canStartWord(prevRune)) ||
				(canBeWord(r) && !canBeWord(prevRune)) {
				words = append(words, strings.ToLower(string(curWord)))
				curWord = []rune{}
			}
		}
		if canBeWord(r) {
			curWord = append(curWord, r)
		}
		prevRune = r
	}
	if len(curWord) > 0 {
		words = append(words, strings.ToLower(string(curWord)))
	}
	return words
}

func (d *dialog) updateCurTargets() {
	exactMatches := []*tabTarget{}
	prefixMatches := []*tabTarget{}
	substringMatches := []*tabTarget{}
	wordPrefixMatches := []*tabTarget{}
	needle := strings.ToLower(d.input)
	needleWords := strings.Split(needle, " ")
	for _, t := range d.targets {
		haystack := strings.ToLower(t.display)
		if needle == haystack {
			exactMatches = append(exactMatches, t)
		} else if strings.HasPrefix(haystack, needle) {
			prefixMatches = append(prefixMatches, t)
		} else if strings.Contains(haystack, needle) {
			substringMatches = append(substringMatches, t)
		} else if len(needleWords) > 1 {
			haystackWords := splitWords(t.display)
			match := false
			for skip := 0; skip <= len(haystackWords)-len(needleWords); skip++ {
				subMatch := true
				for i := 0; i < len(needleWords) && i < len(haystackWords); i++ {
					if !strings.HasPrefix(haystackWords[skip+i], needleWords[i]) {
						subMatch = false
						break
					}
				}
				if subMatch {
					match = true
				}
			}
			if match {
				wordPrefixMatches = append(wordPrefixMatches, t)
			}
		}
	}
	d.curTargets = append(exactMatches, prefixMatches...)
	d.curTargets = append(d.curTargets, substringMatches...)
	d.curTargets = append(d.curTargets, wordPrefixMatches...)
}

// respond to midi events
func (d *dialog) midiEvent(msg []byte) {
	switch d.mode {
	case noteInput:
		if msg[0]&0xf0 == 0x90 && msg[2] > 0 { // note on
			d.shown = false
			if d.action != nil {
				d.action(fmt.Sprintf("m%d", msg[1]))
			}
		}
	case keySigInput:
		if ki := d.keymap.getByKey(fmt.Sprintf("m%d", msg[1])); ki != nil {
			d.handleKeySigKey(ki.PitchSrc, ki.IsMod)
		} else {
			d.handleKeySigKey(newSemiPitch(d.keymap.midimap[msg[1]]), false)
		}
	}
}

// process input in keysig mode
func (d *dialog) handleKeySigKey(pitch *pitchSrc, isMod bool) {
	// handle key
	if isMod {
		for _, v := range d.keySigNotes {
			if _, ok := d.keySig[v]; !ok {
				d.keySig[v] = newSemiPitch(0)
			}
			d.keySig[v] = d.keySig[v].add(pitch)
		}
	} else {
		note := posMod(pitch.semitones(), 12)
		for _, v := range d.keySigNotes {
			if v == note {
				return
			}
		}
		d.keySigNotes = append(d.keySigNotes, note)
	}

	// update display text
	a := make([]string, len(d.keySigNotes))
	for i, v := range d.keySigNotes {
		note := v
		if mod, ok := d.keySig[v]; ok {
			note += mod.semitones()
		}
		if a[i] = d.keymap.notatePitch(note, false); a[i] == "" {
			a[i] = fmt.Sprintf("%.2f", note)
		}
	}
	d.prompt[1] = strings.Join(a, " ")
}

// try to tab-complete an entered file path
func (d *dialog) tryPathComplete() {
	if len(d.curTargets) > 0 {
		d.input = d.curTargets[0].display
		d.updateCurTargets()
	}
}
