package main

import (
	"fmt"
	"math"
	"strconv"
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

	// widest range achievable with pitch wheel
	minPitch = -bendSemitones
	maxPitch = 127 + bendSemitones

	historySizeLimit = 10000000 // 10 MB
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
	history          []*editAction
	historyIndex     int // index of action that undo will undo
	followSong       bool
	prevPlayPos      int64
}

// used for undo/redo. the track structs have nil event slices.
type editAction struct {
	beforeTracks []*track
	afterTracks  []*track
	beforeEvents []*trackEvent
	afterEvents  []*trackEvent
	trackShift   *trackShift
	size         int
}

// substruct in editAction
type trackShift struct {
	min, max, offset int
}

// return a new track shift that will undo this one
func reverseTrackShift(ts *trackShift) *trackShift {
	if ts == nil {
		return nil
	}
	return &trackShift{ts.min + ts.offset, ts.max + ts.offset, -ts.offset}
}

// draw all components of the pattern editor interface
// TODO all the modification to the dst viewport rect is kind of messy
func (pe *patternEditor) draw(r *sdl.Renderer, dst *sdl.Rect, playPos int64) {
	pe.viewport = &sdl.Rect{dst.X, dst.Y, dst.W, dst.H}
	pe.headerHeight = pe.printer.rect.H + padding*2
	pe.beatWidth = pe.printer.rect.W*beatDigits + padding*2
	pe.beatHeight = (pe.printer.rect.H + padding) * rowsPerBeat
	pe.trackWidth = pe.printer.rect.W*int32(len("on 123.86 100")) + padding

	// scroll to center play position if song follow is on and play pos changed
	dst.Y += pe.headerHeight
	dst.H -= pe.headerHeight
	if pe.followSong && playPos != pe.prevPlayPos {
		pe.scrollY = int32(playPos*int64(pe.beatHeight)/ticksPerBeat) -
			dst.H/2 + pe.beatHeight/rowsPerBeat/2
		if pe.scrollY < 0 {
			pe.scrollY = 0
		}
	}
	pe.prevPlayPos = playPos

	// draw play position
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
	r.SetDrawColorArray(colorBgArray...)
	r.FillRect(&sdl.Rect{dst.X, dst.Y, pe.beatWidth, dst.H})
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
	for i := trackMin; i <= trackMax; i++ {
		if te2 := pe.song.Tracks[i].getEventAtTick(te.Tick); te2 != nil {
			ea.beforeEvents = append(ea.afterEvents, te2.clone())
		}
		te3 := te.clone()
		te3.track = i
		ea.afterEvents = append(ea.afterEvents, te3)
		p.signal <- playerSignal{typ: signalEvent, event: te3}
	}
	pe.doNewEditAction(ea)
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
// in the affected area are first deleted
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
			ea.afterEvents = append(ea.afterEvents, te2)
		}
	}
	pe.doNewEditAction(ea)
}

// set the channels of selected tracks
func (pe *patternEditor) setTrackChannel(channel uint8) {
	trackMin, trackMax, _, _ := pe.getSelection()
	ea := &editAction{}
	for i := trackMin; i <= trackMax; i++ {
		ea.beforeTracks = append(ea.beforeTracks, &track{
			Channel: pe.song.Tracks[i].Channel,
			index:   i,
		})
		ea.afterTracks = append(ea.afterTracks, &track{
			Channel: channel,
			index:   i,
		})
	}
	pe.doNewEditAction(ea)
}

// add a track to the right of the selection
func (pe *patternEditor) insertTrack() {
	_, trackMax, _, _ := pe.getSelection()
	pe.doNewEditAction(&editAction{
		afterTracks: []*track{&track{
			Channel: pe.song.Tracks[trackMax].Channel,
			index:   trackMax,
		}},
	})
}

// delete selected tracks
func (pe *patternEditor) deleteTrack() {
	trackMin, trackMax, _, _ := pe.getSelection()
	ea := &editAction{}
	for i := trackMin; i <= trackMax; i++ {
		t := pe.song.Tracks[i]
		ea.beforeTracks = append(ea.beforeTracks, &track{Channel: t.Channel, index: i})
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
	ea := &editAction{}
	for i := trackMin; i <= trackMax; i++ {
		for _, te := range pe.song.Tracks[i].Events {
			if te.Type == noteOnEvent && te.Tick >= tickMin && te.Tick <= tickMax {
				f := te.FloatData + delta
				if f < minPitch {
					f = minPitch
				} else if f > maxPitch {
					f = maxPitch
				}
				ea.beforeEvents = append(ea.beforeEvents, te.clone())
				te2 := te.clone()
				te2.FloatData = f
				te2.setUiString()
				ea.afterEvents = append(ea.afterEvents, te2)
			}
		}
	}
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
			}
		}
		if startEvt != nil && endEvt != nil && startEvt.Type == endEvt.Type {
			increment := ticksPerBeat / int64(pe.division)
			for tick := startEvt.Tick + increment; tick < endEvt.Tick; tick += increment {
				te := startEvt.clone()
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
				case pitchBendEvent, tempoEvent:
					te.FloatData = interpolateValue(tick,
						startEvt.Tick, endEvt.Tick, startEvt.FloatData, endEvt.FloatData)
				case programEvent:
					te.ByteData1 = byte(math.Round(interpolateValue(tick, startEvt.Tick,
						endEvt.Tick, float64(startEvt.ByteData1), float64(endEvt.ByteData1))))
				}
				te.setUiString()
				if te2 := pe.song.Tracks[i].getEventAtTick(te.Tick); te2 != nil {
					ea.beforeEvents = append(ea.beforeEvents, te2.clone())
				}
				ea.afterEvents = append(ea.afterEvents, te)
			}
		}
	}
	pe.doNewEditAction(ea)
}

// linearly interpolate a value
func interpolateValue(pos, start, end int64, a, b float64) float64 {
	coeff := float64(pos-start) / float64(end-start)
	return a*(1-coeff) + b*coeff
}

// play note offs for selected tracks
func (pe *patternEditor) playSelectionNoteOff(p *player) {
	trackMin, trackMax, _, _ := pe.getSelection()
	for i := trackMin; i <= trackMax; i++ {
		p.signal <- playerSignal{typ: signalEvent, event: &trackEvent{
			Type:  noteOffEvent,
			track: i,
		}}
	}
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
	for _, te := range ea.afterEvents {
		pe.addEvent(te)
	}
	if ts := ea.trackShift; ts != nil {
		pe.applyTrackShift(ts.min, ts.max, ts.offset)
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
	t2 := &track{}
	*t2 = *t
	pe.song.Tracks[t.index] = t2
}

// do a new edit action and insert it at the history index, clearing the
// history beyond that index
func (pe *patternEditor) doNewEditAction(ea *editAction) {
	pe.historyIndex++
	pe.history = pe.history[:pe.historyIndex]
	pe.history = append(pe.history, ea)
	pe.doEditAction(ea)
	size := pe.getHistorySize()
	for size > historySizeLimit {
		size -= pe.history[0].size
		pe.history = pe.history[1:]
		pe.historyIndex--
	}
}

// returns the approximate size of the undo buffer in bytes
func (pe *patternEditor) getHistorySize() int {
	size := 0
	for _, ea := range pe.history {
		if ea.size == 0 {
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
