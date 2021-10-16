package main

import (
	"time"

	"github.com/veandco/go-sdl2/sdl"
)

const (
	inputCursorBlinkMs = 500
	border             = 2
)

// modal dialog that displays a message or prompts for input
type dialog struct {
	prompt  string
	input   string
	size    int          // text box will have room for this many chars
	action  func(string) // run if dialog is closed with K_RETURN
	shown   bool
	accept  bool // accept input
	keymode bool // convert key presses to strings instead of text input
}

// create a new dialog
func newDialog(prompt string, size int, action func(string)) *dialog {
	return &dialog{prompt: prompt, size: size, action: action, shown: true}
}

// draw the dialog
func (d *dialog) draw(p *printer, r *sdl.Renderer) {
	if !d.shown {
		return
	}

	// get displayed position and size
	viewport := r.GetViewport()
	promptWidth := p.rect.W * int32(len(d.prompt))
	inputWidth := p.rect.W * int32(d.size)
	w := promptWidth + padding*2
	if inputWidth > promptWidth {
		w = inputWidth + padding*2
	}
	h := p.rect.H + padding*2
	if d.size > 0 {
		h *= 2
	}
	rect := &sdl.Rect{viewport.W/2 - w/2, viewport.H/2 - h/2, w, h}

	// draw box and prompt
	r.SetDrawColorArray(colorFgArray...)
	r.FillRect(&sdl.Rect{rect.X - border, rect.Y - border, rect.W + border*2, rect.H + border*2})
	r.SetDrawColorArray(colorBgArray...)
	r.FillRect(rect)
	p.draw(r, d.prompt, viewport.W/2-promptWidth/2, rect.Y+padding)

	// draw input
	if d.size > 0 {
		r.SetDrawColorArray(colorHighlightArray...)
		r.FillRect(&sdl.Rect{viewport.W/2 - inputWidth/2 - padding/2, rect.Y + p.rect.H + padding*2,
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
	if d.accept && !d.keymode && len(d.input)+len(text) <= d.size {
		d.input += e.GetText()
	}
}

// respond to keyboard events
func (d *dialog) keyboardEvent(e *sdl.KeyboardEvent) {
	// ignore key release
	if e.State != sdl.PRESSED {
		return
	}

	if d.keymode {
		switch e.Keysym.Sym {
		case sdl.K_ESCAPE:
			d.shown = false
		case sdl.K_LSHIFT, sdl.K_RSHIFT, sdl.K_LCTRL, sdl.K_RCTRL, sdl.K_LALT, sdl.K_RALT,
			sdl.K_LGUI, sdl.K_RGUI:
			// don't react to modifier keys
		default:
			d.shown = false
			if d.action != nil {
				d.action(formatKeyEvent(e))
			}
		}
	} else {
		switch e.Keysym.Sym {
		case sdl.K_BACKSPACE:
			if len(d.input) > 0 {
				d.input = d.input[:len(d.input)-1]
			}
		case sdl.K_ESCAPE:
			d.shown = false
		case sdl.K_RETURN:
			d.shown = false
			if d.action != nil {
				d.action(d.input)
			}
		}
	}
}