package main

import (
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/veandco/go-sdl2/sdl"
)

const (
	maxDirNames        = 1000
	inputCursorBlinkMs = 500
	border             = 2
)

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
	return &dialog{prompt: strings.Split(prompt, "\n"), size: size, action: action, shown: true}
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

// return the larger of two integers
func intMax(a, b int) int {
	if a > b {
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

// set d to a file path dialog that allows for tab completion
func (d *dialog) getPath(prompt, dir, ext string, action func(string)) {
	*d = *newDialog(prompt, 50, action)
	d.dir, d.ext = joinTreePath(dir), ext
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
}

// respond to text input events
func (d *dialog) textInput(e *sdl.TextInputEvent) {
	text := e.GetText()
	if d.accept && d.mode == textInput && len(d.input)+len(text) <= d.size {
		d.input += e.GetText()
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
			} else if len(d.input) > 0 {
				_, size := utf8.DecodeLastRuneInString(d.input)
				d.input = d.input[:len(d.input)-size]
			}
		case sdl.K_ESCAPE:
			d.shown = false
		case sdl.K_RETURN:
			d.shown = false
			if d.action != nil {
				d.action(d.input)
			}
		case sdl.K_TAB:
			if d.dir != "" {
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
			d.handleKeySigKey(formatKeyEvent(e, true))
		}
	}
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
		d.handleKeySigKey(fmt.Sprintf("m%d", msg[1]))
	}
}

// process input in keysig mode
func (d *dialog) handleKeySigKey(key string) {
	if ki := d.keymap.getByKey(key); ki != nil {
		// handle key
		if ki.IsMod {
			for _, v := range d.keySigNotes {
				if _, ok := d.keySig[v]; !ok {
					d.keySig[v] = newSemiPitch(0)
				}
				d.keySig[v] = d.keySig[v].add(ki.PitchSrc)
			}
		} else {
			note := posMod(ki.PitchSrc.semitones(), 12)
			for _, v := range d.keySigNotes {
				if v == note {
					return
				}
			}
			d.keySigNotes = append(d.keySigNotes, note)
		}

		// update display text
		// TODO don't include octave numbers
		// TODO figure out what to display if notation fails
		a := make([]string, len(d.keySigNotes))
		for i, v := range d.keySigNotes {
			note := v
			if mod, ok := d.keySig[v]; ok {
				note += mod.semitones()
			}
			a[i] = d.keymap.notatePitch(posMod(note, 12))
		}
		d.prompt[1] = strings.Join(a, " ")
	}
}

// try to tab-complete an entered file path
func (d *dialog) tryPathComplete() {
	if f, err := os.Open(d.dir); err == nil {
		candidate := ""
		if names, err := f.Readdirnames(maxDirNames); err == nil {
			for _, name := range names {
				if d.ext != "" && !strings.HasSuffix(name, d.ext) {
					continue
				}
				if strings.HasPrefix(name, d.input) {
					if candidate == "" {
						candidate = name
					} else {
						candidate = commonPrefix(candidate, name)
					}
				}
			}
		}
		if candidate != "" {
			d.input = candidate
		}
	}
}

// return the longest common prefix of two strings
func commonPrefix(a, b string) string {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return a[:i]
		}
	}
	if len(a) < len(b) {
		return a
	}
	return b
}
