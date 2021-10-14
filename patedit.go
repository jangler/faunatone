package main

import (
	"math"
	"strconv"

	"github.com/veandco/go-sdl2/sdl"
)

const (
	ticksPerBeat = 120
	scrollTicks  = ticksPerBeat / 2
	rowsPerBeat  = 4 // used for graphical purposes only
	beatDigits   = 4
)

// user interface structure for song editing
type patternEditor struct {
	song             *song
	printer          *printer
	scrollX          int32 // pixels
	scrollY          int32 // pixels
	cursorTrackClick int
	cursorTrackDrag  int
	cursorTickClick  int64
	cursorTickDrag   int64
	division         int   // how many steps beats are divided into in the UI
	headerHeight     int32 // pixels (track headers)
	beatWidth        int32 // pixels (beat column)
	beatHeight       int32 // pixels
	trackWidth       int32 // pixels
	viewport         *sdl.Rect
}

// draw all components of the pattern editor interface
// TODO all the modification to the dst viewport rect is kind of messy
func (pe *patternEditor) draw(r *sdl.Renderer, dst *sdl.Rect) {
	pe.viewport = &sdl.Rect{dst.X, dst.Y, dst.W, dst.H}
	pe.headerHeight = pe.printer.rect.H + padding*2
	pe.beatWidth = pe.printer.rect.W*beatDigits + padding*2
	pe.beatHeight = (pe.printer.rect.H + padding) * rowsPerBeat
	pe.trackWidth = pe.printer.rect.W*int32(len("ch $ffff")) + padding

	// draw selection
	dst.X += pe.beatWidth
	dst.W -= pe.beatWidth
	dst.Y += pe.headerHeight
	dst.H -= pe.headerHeight
	trackMin, trackMax := pe.cursorTrackClick, pe.cursorTrackDrag
	if trackMin > trackMax {
		trackMin, trackMax = trackMax, trackMin
	}
	tickMin, tickMax := pe.cursorTickClick, pe.cursorTickDrag
	if tickMin > tickMax {
		tickMin, tickMax = tickMax, tickMin
	}
	x := dst.X + int32(trackMin)*pe.trackWidth - pe.scrollX
	y := dst.Y + int32(tickMin*int64(pe.beatHeight)/ticksPerBeat) - pe.scrollY
	w := int32(trackMax-trackMin+1) * pe.trackWidth
	h := int32(tickMax-tickMin)*pe.beatHeight/ticksPerBeat + pe.beatHeight/rowsPerBeat
	if x+w > dst.X && x < dst.X+dst.W &&
		y+h > dst.Y && y < dst.Y+dst.H {
		r.SetDrawColorArray(colorHighlightArray...)
		r.FillRect(&sdl.Rect{x, y, w, h})
	}
	dst.X -= pe.beatWidth
	dst.W += pe.beatWidth
	dst.Y -= pe.headerHeight
	dst.H += pe.headerHeight

	// draw track headers
	x = dst.X + pe.beatWidth - pe.scrollX
	r.SetDrawColorArray(colorBgArray...)
	r.FillRect(&sdl.Rect{x, dst.Y, dst.W, pe.headerHeight})
	for _ = range pe.song.tracks {
		if x+pe.trackWidth > dst.X && x < dst.X+dst.W {
			pe.printer.draw(r, "ch $ffff", x, dst.Y+padding)
		}
		x += pe.trackWidth
	}

	// draw beat numbers
	for i := (pe.scrollY / pe.beatHeight); i < (pe.scrollY+dst.H)/pe.beatHeight+2; i++ {
		y := dst.Y + int32(i-1)*pe.beatHeight + pe.headerHeight - pe.scrollY
		if y+pe.printer.rect.H > dst.Y && y < dst.Y+dst.H {
			s := strconv.Itoa(int(i))
			if len(s) > beatDigits {
				s = s[len(s)-beatDigits:]
			}
			pe.printer.draw(r, s, dst.X+padding, y+padding/2)
		}
	}

	// draw events
	dst.X += pe.beatWidth
	dst.W -= pe.beatWidth
	dst.Y += pe.headerHeight
	dst.H -= pe.headerHeight
	x = dst.X - pe.scrollX
	for _, t := range pe.song.tracks {
		if x+pe.trackWidth > dst.X && x < dst.X+dst.W {
			for _, e := range t.events {
				y := dst.Y + int32(e.tick*int64(pe.beatHeight)/ticksPerBeat) - pe.scrollY
				if y+pe.beatHeight > dst.Y && y < dst.Y+dst.H {
					pe.printer.draw(r, e.uiString, x+padding/2, y+padding/2)
				}
			}
		}
		x += pe.trackWidth
	}
}

// respond to mouse motion events
func (pe *patternEditor) mouseMotion(e *sdl.MouseMotionEvent) {
	// only respond to drag
	if e.State&sdl.BUTTON_LEFT == 0 {
		return
	}
	pe.cursorTrackDrag, pe.cursorTickDrag = pe.convertMouseCoords(e.X, e.Y)
}

// respond to mouse button events
func (pe *patternEditor) mouseButton(e *sdl.MouseButtonEvent) {
	// only respond to mouse down
	if e.Type != sdl.MOUSEBUTTONDOWN {
		return
	}
	pe.cursorTrackClick, pe.cursorTickClick = pe.convertMouseCoords(e.X, e.Y)
	pe.cursorTrackDrag, pe.cursorTickDrag = pe.cursorTrackClick, pe.cursorTickClick
}

// converts click/drag coords to track index and tick
func (pe *patternEditor) convertMouseCoords(x, y int32) (int, int64) {
	// x -> track
	track := int((x - pe.viewport.X - pe.beatWidth + pe.scrollX) / pe.trackWidth)
	if track < 0 {
		track = 0
	} else if track >= len(pe.song.tracks) {
		track = len(pe.song.tracks) - 1
	}

	// y -> tick
	tick := int64((y - pe.viewport.Y - pe.headerHeight + pe.scrollY -
		pe.beatHeight/rowsPerBeat/2) * ticksPerBeat / pe.beatHeight)
	if tick < 0 {
		tick = 0
	}
	tick = pe.roundTickToDivision(tick)

	return track, tick
}

func (pe *patternEditor) roundTickToDivision(t int64) int64 {
	return int64(math.Round(float64(t*int64(pe.division))/ticksPerBeat)) *
		ticksPerBeat / int64(pe.division)
}

// respond to mouse wheel events
func (pe *patternEditor) mouseWheel(e *sdl.MouseWheelEvent) {
	pe.scrollY -= e.Y * scrollTicks * pe.beatHeight / ticksPerBeat
	if pe.scrollY < 0 {
		pe.scrollY = 0
	}
}

// move the cursor and scroll to a specific beat number
func (pe *patternEditor) goToBeat(beat float64) {
	tick := pe.roundTickToDivision(int64(math.Round((beat - 1) * ticksPerBeat)))
	if tick < 0 {
		tick = 0
	}
	pe.cursorTickClick, pe.cursorTickDrag = tick, tick
	pe.scrollToCursorIfOffscreen()
}

// if the cursor is outside the viewport, center it in the viewport
func (pe *patternEditor) scrollToCursorIfOffscreen() {
	pe.scrollY = int32(pe.cursorTickDrag*int64(pe.beatHeight)/ticksPerBeat) -
		(pe.viewport.H-pe.headerHeight)/2 - pe.beatHeight/rowsPerBeat/2
	if pe.scrollY < 0 {
		pe.scrollY = 0
	}
}
