package main

import (
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"time"
	"unsafe"

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

	// widest range achievable with default pitch bend
	minPitch = -24
	maxPitch = 127 + 24
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

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
	refPitchDisplay  string
	history          []*editAction
	historyIndex     int // index of action that undo will undo
	historySizeLimit int
	followSong       bool
	prevPlayPos      int64
	offDivAlphaMod   uint8
	shiftScrollMult  int
}

// used for undo/redo. the track structs have nil event slices.
type editAction struct {
	beforeTracks []*track
	afterTracks  []*track
	beforeEvents []*trackEvent
	afterEvents  []*trackEvent
	trackShift   *trackShift
	tickShift    *tickShift
	size         int
}

// return true if the action does nothing
func (ea *editAction) isNop() bool {
	return len(ea.beforeTracks) == 0 && len(ea.afterTracks) == 0 &&
		len(ea.beforeEvents) == 0 && len(ea.afterEvents) == 0 &&
		ea.trackShift == nil && ea.tickShift == nil
}

// substruct in editAction
type trackShift struct {
	min, max, offset int
}

// substruct in editAction
type tickShift struct {
	trackMin, trackMax int
	position, offset   int64
}

// return a new track shift that will undo this one
func reverseTrackShift(ts *trackShift) *trackShift {
	if ts == nil {
		return nil
	}
	return &trackShift{ts.min + ts.offset, ts.max + ts.offset, -ts.offset}
}

// return a new tick shift that will undo this one
func reverseTickShift(ts *tickShift) *tickShift {
	if ts == nil {
		return nil
	}
	return &tickShift{ts.trackMin, ts.trackMax, ts.position, -ts.offset}
}

// draw all components of the pattern editor interface
// TODO all the modification to the dst viewport rect is kind of messy
func (pe *patternEditor) draw(r *sdl.Renderer, dst *sdl.Rect, playPos int64) {
	pe.viewport = &sdl.Rect{X: dst.X, Y: dst.Y, W: dst.W, H: dst.H}
	pe.headerHeight = pe.printer.rect.H + padding*2
	pe.beatWidth = pe.printer.rect.W*beatDigits + padding*2
	pe.beatHeight = (pe.printer.rect.H + padding) * rowsPerBeat
	pe.trackWidth = pe.printer.rect.W*int32(len("on 123.86 100")) + padding

	// scroll to center play position if song follow is on and play pos changed
	dst.Y += pe.headerHeight
	dst.H -= pe.headerHeight
	if pe.followSong && playPos != pe.prevPlayPos {
		pe.scrollToTick(playPos)
	}
	pe.prevPlayPos = playPos

	// draw vertical beat sidebar
	dst.Y -= pe.headerHeight
	dst.H += pe.headerHeight
	r.SetDrawColorArray(colorBg1Array...)
	r.FillRect(&sdl.Rect{X: dst.X, Y: dst.Y, W: pe.beatWidth, H: dst.H})

	// draw play position
	dst.Y += pe.headerHeight
	dst.H -= pe.headerHeight
	y := dst.Y + int32(playPos*int64(pe.beatHeight)/ticksPerBeat) - pe.scrollY
	h := pe.beatHeight / rowsPerBeat
	if y+h > dst.Y && y < dst.Y+dst.H {
		r.SetDrawBlendMode(sdl.BLENDMODE_BLEND)
		r.SetDrawColorArray(colorPlayPosArray...)
		r.FillRect(&sdl.Rect{X: 0, Y: y, W: pe.viewport.W, H: h})
		r.SetDrawBlendMode(sdl.BLENDMODE_NONE)
	}
	dst.Y -= pe.headerHeight
	dst.H += pe.headerHeight

	// draw beat numbers and lines
	r.SetDrawColorArray(colorBeatArray...)
	for i := (pe.scrollY / pe.beatHeight); i < (pe.scrollY+dst.H)/pe.beatHeight+2; i++ {
		y := dst.Y + int32(i-1)*pe.beatHeight + pe.headerHeight - pe.scrollY
		if y+pe.printer.rect.H > dst.Y && y < dst.Y+dst.H {
			s := strconv.Itoa(int(i))
			if len(s) > beatDigits {
				s = s[len(s)-beatDigits:]
			}
			lineY := y + padding/2 + pe.printer.rect.H/2
			r.DrawLine(dst.X, lineY, dst.X+dst.W, lineY)
			pe.printer.draw(r, s, dst.X+padding, y+padding/2)
		}
	}

	// draw selection
	dst.X += pe.beatWidth
	dst.W -= pe.beatWidth
	dst.Y += pe.headerHeight
	dst.H -= pe.headerHeight
	trackMin, trackMax, tickMin, tickMax := pe.getSelection()
	x := dst.X + int32(trackMin)*pe.trackWidth - pe.scrollX
	y = dst.Y + int32(tickMin*int64(pe.beatHeight)/ticksPerBeat) - pe.scrollY
	w := int32(trackMax-trackMin+1) * pe.trackWidth
	h = int32(tickMax-tickMin)*pe.beatHeight/ticksPerBeat + pe.beatHeight/rowsPerBeat
	if x+w > dst.X && x < dst.X+dst.W &&
		y+h > dst.Y && y < dst.Y+dst.H {
		r.SetDrawBlendMode(sdl.BLENDMODE_BLEND)
		r.SetDrawColorArray(colorSelectArray...)
		r.FillRect(&sdl.Rect{X: x, Y: y, W: w, H: h})
		r.SetDrawBlendMode(sdl.BLENDMODE_NONE)
	}
	dst.Y -= pe.headerHeight
	dst.H += pe.headerHeight
	dst.X -= pe.beatWidth
	dst.W += pe.beatWidth

	// draw track headers
	x = dst.X + pe.beatWidth - pe.scrollX
	r.SetDrawColorArray(colorBg1Array...)
	r.FillRect(&sdl.Rect{X: dst.X, Y: dst.Y, W: dst.W, H: pe.headerHeight})
	for _, t := range pe.song.Tracks {
		if x+pe.trackWidth > dst.X && x < dst.X+dst.W {
			pe.printer.draw(r, "channel "+strconv.Itoa(int(t.Channel)+1), x, dst.Y+padding)
		}
		x += pe.trackWidth
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
					alpha := uint8(255)
					if e.Tick != pe.roundTickToDivision(e.Tick) {
						alpha = pe.offDivAlphaMod
					}
					pe.printer.drawAlpha(r, e.uiString, x+padding/2, y+padding/2, alpha)
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
	if !(&sdl.Point{X: e.X, Y: e.Y}).InRect(pe.viewport) {
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
	if !(&sdl.Point{X: e.X, Y: e.Y}).InRect(pe.viewport) {
		return
	}
	x, y := pe.convertMouseCoords(e.X, e.Y)
	if sdl.GetModState()&sdl.KMOD_SHIFT == 0 {
		pe.cursorTrackClick, pe.cursorTickClick = x, y
	}
	pe.cursorTrackDrag, pe.cursorTickDrag = x, y
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
	st := scrollTicks
	if sdl.GetModState()&sdl.KMOD_SHIFT != 0 {
		st *= pe.shiftScrollMult
	}
	pe.scrollY -= e.Y * int32(st) * pe.beatHeight / ticksPerBeat
	if pe.scrollY < 0 {
		pe.scrollY = 0
	}
}

// move scroll to a specific beat number
func (pe *patternEditor) goToBeat(beat float64) {
	tick := pe.roundTickToDivision(int64(math.Round((beat - 1) * ticksPerBeat)))
	if tick < 0 {
		tick = 0
	}
	pe.scrollToTick(tick)
}

// scroll to a tick
func (pe *patternEditor) scrollToTick(tick int64) {
	pe.scrollY = int32(tick*int64(pe.beatHeight)/ticksPerBeat) -
		pe.viewport.H/2 + pe.beatHeight/rowsPerBeat
	if pe.scrollY < 0 {
		pe.scrollY = 0
	}
}

// if cursor y is outside the viewport, center it in the viewport
// if cursor x is outside the viewport, adjust until it's not
func (pe *patternEditor) scrollToCursorIfOffscreen() {
	y := int32(pe.cursorTickDrag*int64(pe.beatHeight)/ticksPerBeat) - pe.scrollY
	if y < 0 || y+pe.beatHeight/rowsPerBeat > pe.viewport.H-pe.headerHeight {
		pe.scrollY += y - (pe.viewport.H-pe.headerHeight)/2 + pe.beatHeight/rowsPerBeat/2
		if pe.scrollY < 0 {
			pe.scrollY = 0
		}
	}
	x := int32(pe.cursorTrackDrag)*pe.trackWidth - pe.scrollX
	for x < pe.beatWidth {
		pe.scrollX -= pe.trackWidth
		x += pe.trackWidth
	}
	for x+pe.trackWidth > pe.viewport.W {
		pe.scrollX += pe.trackWidth
		x -= pe.trackWidth
	}
	if pe.scrollX < 0 {
		pe.scrollX = 0 // this is sloppy
	}
}

// write an event to the cursor position(s) and play it
func (pe *patternEditor) writeEvent(te *trackEvent, p *player) {
	trackMin, trackMax, tickMin, _ := pe.getSelection()
	te.Tick = tickMin
	ea := &editAction{}
	if te.Type == noteOnEvent || te.Type == drumNoteOnEvent {
		pe.writeSingleEvent(te, te.track, ea, p)
	} else {
		for i := trackMin; i <= trackMax; i++ {
			pe.writeSingleEvent(te, i, ea, p)
		}
	}
	pe.doNewEditAction(ea)
}

// write an event at just one position
func (pe *patternEditor) writeSingleEvent(te *trackEvent, i int, ea *editAction, p *player) {
	if te2 := pe.song.Tracks[i].getEventAtTick(te.Tick); te2 != nil {
		ea.beforeEvents = append(ea.afterEvents, te2.clone())
	}
	te3 := te.clone()
	te3.track = i
	ea.afterEvents = append(ea.afterEvents, te3)
	p.signal <- playerSignal{typ: signalEvent, event: te3}
}

// delete selected track events
func (pe *patternEditor) deleteSelectedEvents() {
	ea := pe.deleteArea(pe.getSelection())
	pe.doNewEditAction(ea)
}

// return an editAction that would delete track events in a given area
func (pe *patternEditor) deleteArea(trackMin, trackMax int, tickMin, tickMax int64) *editAction {
	ea := &editAction{}
	for i, t := range pe.song.Tracks {
		if i >= trackMin && i <= trackMax {
			for _, te := range t.Events {
				if te.Tick >= tickMin && te.Tick <= tickMax {
					ea.beforeEvents = append(ea.beforeEvents, te.clone())
				}
			}
		}
	}
	return ea
}

// reset scroll, cursor, and history state
func (pe *patternEditor) reset() {
	pe.cursorTrackClick, pe.cursorTrackDrag = 0, 0
	pe.cursorTickClick, pe.cursorTickDrag = 0, 0
	pe.scrollX, pe.scrollY = 0, 0
	pe.history = pe.history[:0]
	pe.historyIndex = -1
}

// copy selected events to a buffer
func (pe *patternEditor) copy() {
	trackMin, trackMax, tickMin, tickMax := pe.getSelection()
	pe.copyTicks = tickMax - tickMin
	pe.copiedEvents = make([][]*trackEvent, trackMax-trackMin+1)
	for i := range pe.copiedEvents {
		for _, te := range pe.song.Tracks[trackMin+i].Events {
			if te.Tick >= tickMin && te.Tick <= tickMax {
				te2 := te.clone()
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
// in the affected area are first deleted; if mix is true then existing events
// take precedence
func (pe *patternEditor) paste(mix bool) {
	trackMin, _, tickMin, _ := pe.getSelection()
	ea := &editAction{}
	if !mix {
		ea = pe.deleteArea(trackMin, trackMin+len(pe.copiedEvents)-1, tickMin, tickMin+pe.copyTicks)
	}
	for i := range pe.copiedEvents {
		if i+trackMin >= len(pe.song.Tracks) {
			break
		}
		for _, te := range pe.copiedEvents[i] {
			te2 := te.clone()
			te2.Tick += tickMin
			te2.track = i + trackMin
			if !mix || pe.song.Tracks[te2.track].getEventAtTick(te2.Tick) == nil {
				ea.afterEvents = append(ea.afterEvents, te2)
			}
		}
	}
	pe.doNewEditAction(ea)
}

// set the channels of selected tracks
func (pe *patternEditor) setTrackChannel(channel uint8) {
	trackMin, trackMax, _, _ := pe.getSelection()
	ea := &editAction{}
	for i := trackMin; i <= trackMax; i++ {
		ea.beforeTracks = append(ea.beforeTracks, newTrack(pe.song.Tracks[i].Channel, i))
		ea.afterTracks = append(ea.afterTracks, newTrack(channel, i))
	}
	pe.doNewEditAction(ea)
}

// add a new track for each track in the selection
func (pe *patternEditor) insertTracks() {
	trackMin, trackMax, _, _ := pe.getSelection()
	ea := &editAction{}
	for i := trackMin; i <= trackMax; i++ {
		ea.afterTracks = append(ea.afterTracks, newTrack(pe.song.Tracks[i].Channel, i))
	}
	pe.doNewEditAction(ea)
}

// delete selected tracks
func (pe *patternEditor) deleteTracks() {
	trackMin, trackMax, _, _ := pe.getSelection()
	ea := &editAction{}
	for i := trackMin; i <= trackMax && len(pe.song.Tracks)-len(ea.beforeTracks) > 1; i++ {
		t := pe.song.Tracks[i]
		ea.beforeTracks = append(ea.beforeTracks, newTrack(t.Channel, i))
		for _, te := range t.Events {
			ea.beforeEvents = append(ea.beforeEvents, te.clone())
		}
	}
	pe.doNewEditAction(ea)
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
	pe.doNewEditAction(&editAction{trackShift: &trackShift{trackMin, trackMax, offset}})
}

// actually implement track shifting
func (pe *patternEditor) applyTrackShift(trackMin, trackMax, offset int) {
	if offset < 0 && trackMin > 0 {
		for i := trackMin - 1; i < trackMax; i++ {
			pe.song.Tracks[i], pe.song.Tracks[i+1] = pe.song.Tracks[i+1], pe.song.Tracks[i]
		}
		if pe.cursorTrackClick >= trackMin && pe.cursorTrackClick <= trackMax {
			pe.cursorTrackClick -= 1
		}
		if pe.cursorTrackDrag >= trackMin && pe.cursorTrackDrag <= trackMax {
			pe.cursorTrackDrag -= 1
		}
		pe.applyTrackShift(trackMin, trackMax, offset+1)
	} else if offset > 0 && trackMax < len(pe.song.Tracks)-1 {
		for i := trackMax + 1; i > trackMin; i-- {
			pe.song.Tracks[i], pe.song.Tracks[i-1] = pe.song.Tracks[i-1], pe.song.Tracks[i]
		}
		if pe.cursorTrackClick >= trackMin && pe.cursorTrackClick <= trackMax {
			pe.cursorTrackClick += 1
		}
		if pe.cursorTrackDrag >= trackMin && pe.cursorTrackDrag <= trackMax {
			pe.cursorTrackDrag += 1
		}
		pe.applyTrackShift(trackMin, trackMax, offset-1)
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
	pe.updateRefPitchDisplay()
}

// set ref pitch to the top-left corner of selection
func (pe *patternEditor) captureRefPitch() {
	trackMin, _, tickMin, _ := pe.getSelection()
	for _, te := range pe.song.Tracks[trackMin].Events {
		if (te.Type == noteOnEvent || te.Type == pitchBendEvent) && te.Tick == tickMin {
			pe.refPitch = te.FloatData
			pe.updateRefPitchDisplay()
			return
		}
	}
}

// update the displayed notation for the reference pitch
func (pe *patternEditor) updateRefPitchDisplay() {
	s := pe.song.Keymap.notatePitch(pe.refPitch, true)
	if s == "" {
		s = fmt.Sprintf("%.2f", pe.refPitch)
	}
	pe.refPitchDisplay = s
}

// add to pitch of selected notes
func (pe *patternEditor) transposeSelection(delta float64) {
	ea := &editAction{}
	pe.forEventsInSelection(func(t *track, te *trackEvent) {
		if te.Type == noteOnEvent || te.Type == pitchBendEvent {
			f := te.FloatData + delta
			if f < minPitch {
				f = minPitch
			} else if f > maxPitch {
				f = maxPitch
			}
			ea.beforeEvents = append(ea.beforeEvents, te.clone())
			te2 := te.clone()
			te2.FloatData = f
			te2.setUiString(pe.song.Keymap)
			ea.afterEvents = append(ea.afterEvents, te2)
		}
	})
	pe.doNewEditAction(ea)
}

// insert interpolated events at each division between events of same type at
// each end of selection
func (pe *patternEditor) interpolateSelection() {
	trackMin, trackMax, tickMin, tickMax := pe.getSelection()
	ea := &editAction{}
	for i := trackMin; i <= trackMax; i++ {
		var startEvt, endEvt *trackEvent
		for _, te := range pe.song.Tracks[i].Events {
			if te.Tick == tickMin {
				startEvt = te
			} else if te.Tick == tickMax {
				endEvt = te
			} else if te.Tick > tickMin && te.Tick < tickMax {
				ea.beforeEvents = append(ea.beforeEvents, te.clone())
			}
		}
		prevEvent := startEvt
		if startEvt != nil && endEvt != nil && (startEvt.Type == endEvt.Type ||
			(startEvt.Type == noteOnEvent && endEvt.Type == pitchBendEvent) ||
			(startEvt.Type == pitchBendEvent && endEvt.Type == noteOnEvent)) {
			if endEvt.Type == noteOnEvent {
				// convert end note to pitch bend
				ea.beforeEvents = append(ea.beforeEvents, endEvt.clone())
				te := endEvt.clone()
				te.Type, te.ByteData1 = pitchBendEvent, 0
				te.setUiString(pe.song.Keymap)
				ea.afterEvents = append(ea.afterEvents, te)
				endEvt = te
			}
			increment := ticksPerBeat / int64(pe.division)
			for tick := startEvt.Tick + increment; tick < endEvt.Tick; tick += increment {
				te := endEvt.clone()
				te.Tick = tick
				switch te.Type {
				case controllerEvent:
					te.ByteData2 = byte(interpolateValue(tick, startEvt.Tick, endEvt.Tick,
						float64(startEvt.ByteData2), float64(endEvt.ByteData2), true))
				case noteOnEvent:
					te.FloatData = interpolateValue(tick, startEvt.Tick, endEvt.Tick,
						startEvt.FloatData, endEvt.FloatData, false)
					te.ByteData1 = byte(interpolateValue(tick, startEvt.Tick, endEvt.Tick,
						float64(startEvt.ByteData1), float64(endEvt.ByteData1), true))
				case pitchBendEvent, tempoEvent, releaseLenEvent:
					te.FloatData = interpolateValue(tick, startEvt.Tick, endEvt.Tick,
						startEvt.FloatData, endEvt.FloatData, false)
				case programEvent, channelPressureEvent, keyPressureEvent:
					te.ByteData1 = byte(interpolateValue(tick, startEvt.Tick, endEvt.Tick,
						float64(startEvt.ByteData1), float64(endEvt.ByteData1), true))
				}
				te.setUiString(pe.song.Keymap)
				if !eventDataEqual(te, prevEvent) && !eventDataEqual(te, endEvt) {
					ea.afterEvents = append(ea.afterEvents, te)
					prevEvent = te
				}
			}
		}
	}
	pe.doNewEditAction(ea)
}

// linearly interpolate a value
func interpolateValue(pos, start, end int64, a, b float64, round bool) float64 {
	coeff := float64(pos-start) / float64(end-start)
	result := a*(1-coeff) + b*coeff
	if round {
		if a < b {
			result = math.Floor(result)
		} else {
			result = math.Ceil(result)
		}
	}
	return result
}

// return true if data for two events have equal data
func eventDataEqual(e1, e2 *trackEvent) bool {
	return e1.FloatData == e2.FloatData && e1.ByteData1 == e2.ByteData1 &&
		e1.ByteData2 == e2.ByteData2
}

// return the first tick in the current view
func (pe *patternEditor) firstTickOnScreen() int64 {
	return int64(pe.scrollY) * ticksPerBeat / int64(pe.beatHeight)
}

// undo the last edit action
func (pe *patternEditor) undo() error {
	if pe.historyIndex >= 0 {
		ea := pe.history[pe.historyIndex]
		pe.historyIndex--
		pe.doEditAction(&editAction{
			beforeTracks: ea.afterTracks,
			afterTracks:  ea.beforeTracks,
			beforeEvents: ea.afterEvents,
			afterEvents:  ea.beforeEvents,
			trackShift:   reverseTrackShift(ea.trackShift),
			tickShift:    reverseTickShift(ea.tickShift),
		})
		return nil
	}
	return fmt.Errorf("nothing to undo")
}

// redo the last undone edit action
func (pe *patternEditor) redo() error {
	if pe.historyIndex+1 < len(pe.history) {
		pe.historyIndex++
		pe.doEditAction(pe.history[pe.historyIndex])
		return nil
	}
	return fmt.Errorf("nothing to redo")
}

// do an edit action in the "forward" order, without modifying the history
func (pe *patternEditor) doEditAction(ea *editAction) {
	for _, te := range ea.beforeEvents {
		pe.removeEvent(te)
	}
	removedTracks := 0
	for _, t := range ea.beforeTracks {
		stillExists := false
		for _, t2 := range ea.afterTracks {
			if t2.index == t.index {
				stillExists = true
				break
			}
		}
		if !stillExists {
			pe.removeTrack(t, -removedTracks)
			removedTracks++
		}
	}
	for _, t := range ea.afterTracks {
		alreadyExists := false
		for _, t2 := range ea.beforeTracks {
			if t2.index == t.index {
				alreadyExists = true
				break
			}
		}
		if alreadyExists {
			pe.song.Tracks[t.index].Channel = t.Channel
		} else {
			pe.addTrack(t)
		}
	}
	if ts := ea.tickShift; ts != nil {
		pe.applyTickShift(ts)
	}
	for _, te := range ea.afterEvents {
		pe.addEvent(te)
	}
	if ts := ea.trackShift; ts != nil {
		pe.applyTrackShift(ts.min, ts.max, ts.offset)
	}
	if ea.trackShift != nil || len(ea.beforeTracks) > 0 || len(ea.afterTracks) > 0 {
		for i, t := range pe.song.Tracks {
			for _, te := range t.Events {
				te.track = i
			}
		}
	}
}

// remove a matching event from the song (based on track and tick only)
func (pe *patternEditor) removeEvent(te *trackEvent) {
	t := pe.song.Tracks[te.track]
	for i, te2 := range t.Events {
		if te2.Tick == te.Tick {
			t.Events = append(t.Events[:i], t.Events[i+1:]...)
			break
		}
	}
}

// add a copy of the event to the song
func (pe *patternEditor) addEvent(te *trackEvent) {
	te2 := te.clone()
	t := pe.song.Tracks[te.track]
	t.Events = append(t.Events, te2)
}

// remove a matching track from the song (based on index only)
func (pe *patternEditor) removeTrack(t *track, offset int) {
	pe.song.Tracks = append(pe.song.Tracks[:t.index+offset], pe.song.Tracks[t.index+1+offset:]...)
	pe.fixCursor()
}

// add a copy of the track to the song
func (pe *patternEditor) addTrack(t *track) {
	pe.song.Tracks = append(pe.song.Tracks[:t.index+1], pe.song.Tracks[t.index:]...)
	pe.song.Tracks[t.index] = t.clone()
}

// do a new edit action and insert it at the history index, clearing the
// history beyond that index. ignores nop actions
func (pe *patternEditor) doNewEditAction(ea *editAction) {
	if ea.isNop() {
		return // nop action
	}
	pe.historyIndex++
	pe.history = pe.history[:pe.historyIndex]
	pe.history = append(pe.history, ea)
	pe.doEditAction(ea)
	size := pe.getHistorySize()
	for size > pe.historySizeLimit {
		size -= pe.history[0].size
		pe.history = pe.history[1:]
		pe.historyIndex--
	}
}

// return the approximate size of the undo buffer in bytes
func (pe *patternEditor) getHistorySize() int {
	size := 0
	for _, ea := range pe.history {
		if ea.size == 0 {
			ea.size += int(unsafe.Sizeof(ea))
			for _, t := range ea.beforeTracks {
				ea.size += int(unsafe.Sizeof(t))
			}
			for _, t := range ea.afterTracks {
				ea.size += int(unsafe.Sizeof(t))
			}
			for _, te := range ea.beforeEvents {
				ea.size += int(unsafe.Sizeof(te))
			}
			for _, te := range ea.afterEvents {
				ea.size += int(unsafe.Sizeof(te))
			}
			ea.size += int(unsafe.Sizeof(ea.trackShift))
		}
		size += ea.size
	}
	return size
}

// insert time at the cursor
func (pe *patternEditor) insertDivision() {
	pe.doNewTickShift(1)
}

// delete time at the cursor
func (pe *patternEditor) deleteDivision() {
	pe.doNewTickShift(-1)
}

// insert/delete time at the cursor
func (pe *patternEditor) doNewTickShift(factor int64) {
	trackMin, trackMax, tickMin, tickMax := pe.getSelection()
	offset := ticksPerBeat / int64(pe.division)
	if tickMax-tickMin > offset {
		offset = tickMax - tickMin
	}
	offset *= factor
	beforeEvents := []*trackEvent{}
	if factor < 0 {
		for i := trackMin; i <= trackMax; i++ {
			for _, te := range pe.song.Tracks[i].Events {
				if te.Tick >= tickMin && te.Tick+offset < tickMin {
					beforeEvents = append(beforeEvents, te)
				}
			}
		}
	}
	pe.doNewEditAction(&editAction{
		beforeEvents: beforeEvents,
		tickShift: &tickShift{
			trackMin: trackMin,
			trackMax: trackMax,
			position: tickMin,
			offset:   offset,
		},
	})
}

// apply a tick shift edit action
func (pe *patternEditor) applyTickShift(ts *tickShift) {
	for i := ts.trackMin; i <= ts.trackMax; i++ {
		for _, te := range pe.song.Tracks[i].Events {
			if te.Tick >= ts.position {
				te.Tick += ts.offset
			}
		}
	}
}

// multiply the last data value of selected events by a constant factor
func (pe *patternEditor) multiplySelection(f float64) {
	ea := &editAction{}
	pe.forEventsInSelection(func(t *track, te *trackEvent) {
		switch te.Type {
		case noteOnEvent, drumNoteOnEvent, controllerEvent,
			channelPressureEvent, keyPressureEvent, releaseLenEvent:
			ea.beforeEvents = append(ea.beforeEvents, te.clone())
			te2 := te.clone()
			switch te2.Type {
			case noteOnEvent, channelPressureEvent, keyPressureEvent:
				te2.ByteData1 = byte(math.Min(127, math.Max(0,
					math.Round(float64(te.ByteData1)*f))))
			case drumNoteOnEvent, controllerEvent:
				te2.ByteData2 = byte(math.Min(127, math.Max(0,
					math.Round(float64(te.ByteData2)*f))))
			case releaseLenEvent:
				te2.FloatData = te.FloatData * f
			}
			te2.setUiString(pe.song.Keymap)
			ea.afterEvents = append(ea.afterEvents, te2)
		}
	})
	pe.doNewEditAction(ea)
}

// vary the last data value of selected events by a random amount up to the
// given magnitude
func (pe *patternEditor) varySelection(magnitude float64) {
	ea := &editAction{}
	pe.forEventsInSelection(func(t *track, te *trackEvent) {
		switch te.Type {
		case noteOnEvent, drumNoteOnEvent, controllerEvent,
			channelPressureEvent, keyPressureEvent, releaseLenEvent:
			ea.beforeEvents = append(ea.beforeEvents, te.clone())
			te2 := te.clone()
			f := rand.Float64()*magnitude*2 - magnitude
			switch te2.Type {
			case noteOnEvent, channelPressureEvent, keyPressureEvent:
				te2.ByteData1 = byte(math.Min(127, math.Max(0,
					math.Round(float64(te.ByteData1)+f))))
			case drumNoteOnEvent, controllerEvent:
				te2.ByteData2 = byte(math.Min(127, math.Max(0,
					math.Round(float64(te.ByteData2)+f))))
			case releaseLenEvent:
				te2.FloatData = te.FloatData + f
			}
			te2.setUiString(pe.song.Keymap)
			ea.afterEvents = append(ea.afterEvents, te2)
		}
	})
	pe.doNewEditAction(ea)
}

// call a function on every event in the current selection
func (pe *patternEditor) forEventsInSelection(fn func(*track, *trackEvent)) {
	trackMin, trackMax, tickMin, tickMax := pe.getSelection()
	for i := trackMin; i <= trackMax; i++ {
		t := pe.song.Tracks[i]
		for _, te := range t.Events {
			if te.Tick >= tickMin && te.Tick <= tickMax {
				fn(t, te)
			}
		}
	}
}
