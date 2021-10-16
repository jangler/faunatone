package main

import (
	"math"
	"strconv"

	"github.com/veandco/go-sdl2/sdl"
)

const (
	ticksPerBeat      = 960
	scrollTicks       = ticksPerBeat / 2
	rowsPerBeat       = 4 // used for graphical purposes only
	beatDigits        = 4
	defaultDivision   = 4
	defaultVelocity   = 100
	defaultController = 1
	defaultRefPitch   = 60

	// widest range achievable with 2-semitone pitch wheel range
	minPitch = -2
	maxPitch = 129
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
	copyTicks        int64
	copiedEvents     [][]*trackEvent // ticks are relative to start of copy area
	velocity         uint8
	controller       uint8
	refPitch         float64
}

// draw all components of the pattern editor interface
// TODO all the modification to the dst viewport rect is kind of messy
func (pe *patternEditor) draw(r *sdl.Renderer, dst *sdl.Rect, playPos int64) {
	pe.viewport = &sdl.Rect{dst.X, dst.Y, dst.W, dst.H}
	pe.headerHeight = pe.printer.rect.H + padding*2
	pe.beatWidth = pe.printer.rect.W*beatDigits + padding*2
	pe.beatHeight = (pe.printer.rect.H + padding) * rowsPerBeat
	pe.trackWidth = pe.printer.rect.W*int32(len("on 123.86 100")) + padding

	// draw play position
	dst.Y += pe.headerHeight
	dst.H -= pe.headerHeight
	y := dst.Y + int32(playPos*int64(pe.beatHeight)/ticksPerBeat) - pe.scrollY
	h := pe.beatHeight / rowsPerBeat
	if y+h > dst.Y && y < dst.Y+dst.H {
		r.SetDrawColorArray(colorPlayPosArray...)
		r.FillRect(&sdl.Rect{0, y, pe.viewport.W, h})
	}

	// draw selection
	dst.X += pe.beatWidth
	dst.W -= pe.beatWidth
	trackMin, trackMax, tickMin, tickMax := pe.getSelection()
	x := dst.X + int32(trackMin)*pe.trackWidth - pe.scrollX
	y = dst.Y + int32(tickMin*int64(pe.beatHeight)/ticksPerBeat) - pe.scrollY
	w := int32(trackMax-trackMin+1) * pe.trackWidth
	h = int32(tickMax-tickMin)*pe.beatHeight/ticksPerBeat + pe.beatHeight/rowsPerBeat
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
	for _, t := range pe.song.Tracks {
		if x+pe.trackWidth > dst.X && x < dst.X+dst.W {
			pe.printer.draw(r, "channel "+strconv.Itoa(int(t.Channel)+1), x, dst.Y+padding)
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
	for _, t := range pe.song.Tracks {
		if x+pe.trackWidth > dst.X && x < dst.X+dst.W {
			for _, e := range t.Events {
				y := dst.Y + int32(e.Tick*int64(pe.beatHeight)/ticksPerBeat) - pe.scrollY
				if y >= dst.Y && y < dst.Y+dst.H {
					pe.printer.draw(r, e.uiString, x+padding/2, y+padding/2)
				}
			}
		}
		x += pe.trackWidth
	}
}

// return ranges of selected tracks and ticks
func (pe *patternEditor) getSelection() (int, int, int64, int64) {
	trackMin, trackMax := pe.cursorTrackClick, pe.cursorTrackDrag
	if trackMin > trackMax {
		trackMin, trackMax = trackMax, trackMin
	}
	tickMin, tickMax := pe.cursorTickClick, pe.cursorTickDrag
	if tickMin > tickMax {
		tickMin, tickMax = tickMax, tickMin
	}
	return trackMin, trackMax, tickMin, tickMax
}

// respond to mouse motion events
func (pe *patternEditor) mouseMotion(e *sdl.MouseMotionEvent) {
	// only respond to drag
	if e.State&sdl.BUTTON_LEFT == 0 {
		return
	}
	if !(&sdl.Point{e.X, e.Y}).InRect(pe.viewport) {
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
	if !(&sdl.Point{e.X, e.Y}).InRect(pe.viewport) {
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
	} else if track >= len(pe.song.Tracks) {
		track = len(pe.song.Tracks) - 1
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
	y := int32(pe.cursorTickDrag*int64(pe.beatHeight)/ticksPerBeat) - pe.scrollY
	if y < 0 || y+pe.beatHeight/rowsPerBeat > pe.viewport.H-pe.headerHeight {
		pe.scrollY += y - (pe.viewport.H-pe.headerHeight)/2 + pe.beatHeight/rowsPerBeat/2
		if pe.scrollY < 0 {
			pe.scrollY = 0
		}
	}
}

// write an event to the cursor position(s)
func (pe *patternEditor) writeEvent(te *trackEvent) {
	trackMin, trackMax, tickMin, _ := pe.getSelection()
	te.Tick = tickMin
	for i := trackMin; i <= trackMax; i++ {
		// need to make a separate struct for each track
		te2 := &trackEvent{}
		*te2 = *te
		pe.song.Tracks[i].writeEvent(te2)
	}
}

// delete selected track events
func (pe *patternEditor) deleteSelectedEvents() {
	pe.deleteArea(pe.getSelection())
}

// delete track events in a given area
func (pe *patternEditor) deleteArea(trackMin, trackMax int, tickMin, tickMax int64) {
	for i, t := range pe.song.Tracks {
		if i >= trackMin && i <= trackMax {
			for j := 0; j < len(t.Events); j++ {
				te := t.Events[j]
				if te.Tick >= tickMin && te.Tick <= tickMax {
					t.Events = append(t.Events[:j], t.Events[j+1:]...)
					j--
				}
			}
		}
	}
}

// reset scroll and cursor state
func (pe *patternEditor) reset() {
	pe.cursorTrackClick, pe.cursorTrackDrag = 0, 0
	pe.cursorTickClick, pe.cursorTickDrag = 0, 0
	pe.scrollX, pe.scrollY = 0, 0
}

// copy selected events to a buffer
func (pe *patternEditor) copy() {
	trackMin, trackMax, tickMin, tickMax := pe.getSelection()
	pe.copyTicks = tickMax - tickMin
	pe.copiedEvents = make([][]*trackEvent, trackMax-trackMin+1)
	for i := range pe.copiedEvents {
		for _, te := range pe.song.Tracks[trackMin+i].Events {
			if te.Tick >= tickMin && te.Tick <= tickMax {
				te2 := &trackEvent{}
				*te2 = *te
				te2.Tick -= tickMin
				pe.copiedEvents[i] = append(pe.copiedEvents[i], te2)
			}
		}
	}
}

// copy selected events to a buffer, then delete the selection
func (pe *patternEditor) cut() {
	pe.copy()
	pe.deleteSelectedEvents()
}

// paste selected events to a buffer; if mix is false then all existing events
// in the affected area are first deleted
func (pe *patternEditor) paste(mix bool) {
	trackMin, _, tickMin, _ := pe.getSelection()
	if !mix {
		pe.deleteArea(trackMin, trackMin+len(pe.copiedEvents)-1, tickMin, tickMin+pe.copyTicks)
	}
	for i := range pe.copiedEvents {
		if i+trackMin >= len(pe.song.Tracks) {
			break
		}
		for _, te := range pe.copiedEvents[i] {
			te2 := &trackEvent{}
			*te2 = *te
			te2.Tick += tickMin
			pe.song.Tracks[i+trackMin].writeEvent(te2)
		}
	}
}

// set the channels of selected tracks
func (pe *patternEditor) setTrackChannel(channel uint8) {
	trackMin, trackMax, _, _ := pe.getSelection()
	for i := trackMin; i <= trackMax; i++ {
		pe.song.Tracks[i].Channel = channel
	}
}

// add a track to the right of the selection
func (pe *patternEditor) insertTrack() {
	_, trackMax, _, _ := pe.getSelection()
	pe.song.Tracks = append(pe.song.Tracks[:trackMax+1], pe.song.Tracks[trackMax:]...)
	pe.song.Tracks[trackMax+1] = &track{
		Channel: pe.song.Tracks[trackMax].Channel,
	}
}

// delete selected tracks
func (pe *patternEditor) deleteTrack() {
	trackMin, trackMax, _, _ := pe.getSelection()
	for i := trackMax; i >= trackMin && len(pe.song.Tracks) > 1; i-- {
		pe.song.Tracks = append(pe.song.Tracks[:i], pe.song.Tracks[i+1:]...)
	}
	pe.fixCursor()
}

// keep the cursor in bounds
func (pe *patternEditor) fixCursor() {
	if pe.cursorTrackClick < 0 {
		pe.cursorTrackClick = 0
	}
	if pe.cursorTrackDrag < 0 {
		pe.cursorTrackDrag = 0
	}
	if pe.cursorTrackClick >= len(pe.song.Tracks) {
		pe.cursorTrackClick = len(pe.song.Tracks) - 1
	}
	if pe.cursorTrackDrag >= len(pe.song.Tracks) {
		pe.cursorTrackDrag = len(pe.song.Tracks) - 1
	}
	if pe.cursorTickClick < 0 {
		pe.cursorTickClick = 0
	}
	if pe.cursorTickDrag < 0 {
		pe.cursorTickDrag = 0
	}
	pe.cursorTickClick = pe.roundTickToDivision(pe.cursorTickClick)
	pe.cursorTickDrag = pe.roundTickToDivision(pe.cursorTickDrag)
}

// move selected tracks left or right
func (pe *patternEditor) shiftTracks(offset int) {
	trackMin, trackMax, _, _ := pe.getSelection()
	if offset < 0 && trackMin > 0 {
		for i := trackMin - 1; i < trackMax; i++ {
			pe.song.Tracks[i], pe.song.Tracks[i+1] = pe.song.Tracks[i+1], pe.song.Tracks[i]
		}
		pe.cursorTrackClick -= 1
		pe.cursorTrackDrag -= 1
		pe.shiftTracks(offset + 1)
	} else if offset > 0 && trackMax < len(pe.song.Tracks)-1 {
		for i := trackMax + 1; i > trackMin; i-- {
			pe.song.Tracks[i], pe.song.Tracks[i-1] = pe.song.Tracks[i-1], pe.song.Tracks[i]
		}
		pe.cursorTrackClick += 1
		pe.cursorTrackDrag += 1
		pe.shiftTracks(offset - 1)
	}
}

// shift cursor
func (pe *patternEditor) moveCursor(tracks, divisions int) {
	pe.cursorTrackClick += tracks
	pe.cursorTrackDrag += tracks
	pe.cursorTickClick += int64(divisions) * ticksPerBeat / int64(pe.division)
	pe.cursorTickDrag += int64(divisions) * ticksPerBeat / int64(pe.division)
	pe.fixCursor()
	pe.scrollToCursorIfOffscreen()
}

// change beat division via addition
func (pe *patternEditor) addDivision(delta int) {
	pe.division += delta
	if pe.division < 1 {
		pe.division = 1
	}
	pe.fixCursor()
}

// change beat division via multiplication
func (pe *patternEditor) multiplyDivision(factor float64) {
	pe.division = int(math.Round(float64(pe.division) * factor))
	if pe.division < 1 {
		pe.division = 1
	}
	pe.fixCursor()
}

// change reference pitch via addition
func (pe *patternEditor) modifyRefPitch(delta float64) {
	pe.refPitch += delta
	if pe.refPitch < minPitch {
		pe.refPitch = minPitch
	} else if pe.refPitch > maxPitch {
		pe.refPitch = maxPitch
	}
}

// set ref pitch to the top-left corner of selection
func (pe *patternEditor) captureRefPitch() {
	trackMin, _, tickMin, _ := pe.getSelection()
	for _, te := range pe.song.Tracks[trackMin].Events {
		if te.Type == noteOnEvent && te.Tick == tickMin {
			pe.refPitch = te.FloatData
			return
		}
	}
}

// add to pitch of selected notes
func (pe *patternEditor) transposeSelection(delta float64) {
	trackMin, trackMax, tickMin, tickMax := pe.getSelection()
	for i := trackMin; i <= trackMax; i++ {
		for _, te := range pe.song.Tracks[i].Events {
			if te.Type == noteOnEvent && te.Tick >= tickMin && te.Tick <= tickMax {
				f := te.FloatData + delta
				if f < minPitch {
					f = minPitch
				} else if f > maxPitch {
					f = maxPitch
				}
				te.FloatData = f
				te.setUiString()
			}
		}
	}
}

// insert interpolated events at each division between events of same type at
// each end of selection
func (pe *patternEditor) interpolateSelection() {
	trackMin, trackMax, tickMin, tickMax := pe.getSelection()
	for i := trackMin; i <= trackMax; i++ {
		var startEvt, endEvt *trackEvent
		for _, te := range pe.song.Tracks[i].Events {
			if te.Tick == tickMin {
				startEvt = te
			} else if te.Tick == tickMax {
				endEvt = te
			}
		}
		if startEvt != nil && endEvt != nil && startEvt.Type == endEvt.Type {
			increment := ticksPerBeat / int64(pe.division)
			for tick := startEvt.Tick + increment; tick < endEvt.Tick; tick += increment {
				te := &trackEvent{}
				*te = *startEvt
				te.Tick = tick
				switch te.Type {
				case controllerEvent:
					te.ByteData2 = byte(math.Round(interpolateValue(tick, startEvt.Tick,
						endEvt.Tick, float64(startEvt.ByteData2), float64(endEvt.ByteData2))))
				case noteOnEvent:
					te.FloatData = interpolateValue(tick,
						startEvt.Tick, endEvt.Tick, startEvt.FloatData, endEvt.FloatData)
					te.ByteData1 = byte(math.Round(interpolateValue(tick, startEvt.Tick,
						endEvt.Tick, float64(startEvt.ByteData1), float64(endEvt.ByteData1))))
				case tempoEvent:
					te.FloatData = interpolateValue(tick,
						startEvt.Tick, endEvt.Tick, startEvt.FloatData, endEvt.FloatData)
				case programEvent:
					te.ByteData1 = byte(math.Round(interpolateValue(tick, startEvt.Tick,
						endEvt.Tick, float64(startEvt.ByteData1), float64(endEvt.ByteData1))))
				}
				te.setUiString()
				pe.song.Tracks[i].writeEvent(te)
			}
		}
	}
}

// linearly interpolate a value
func interpolateValue(pos, start, end int64, a, b float64) float64 {
	coeff := float64(pos-start) / float64(end-start)
	return a*(1-coeff) + b*coeff
}
