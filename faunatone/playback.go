package main

import (
	"math"
	"sort"
	"time"

	"gitlab.com/gomidi/midi/writer"
)

// type used to signal player behavior
type playerSignal struct {
	typ   playerSignalType
	tick  int64
	world int
	event *trackEvent
}

type playerSignalType uint8

const (
	signalContinue playerSignalType = iota
	signalStart
	signalStop
	signalEvent
	signalSongChanged // TODO actually use this
	signalSendPitchRPN
	signalSendGMSystemOn
	signalResetChannels
)

const (
	defaultBPM = 120
	byteNil    = 0xff
)

// type that writes midi events over time according to a song
type player struct {
	song         *song
	realtime     bool
	lastTick     int64
	horizon      map[int]int64
	bpm          float64
	signal       chan playerSignal
	stopping     chan struct{} // player sends on this channel when stopping
	sendStopping bool          // but only if this is true
	writer       writer.ChannelWriter
	midiChannels []*channelState
	virtChannels []*channelState
	redrawChan   chan bool // send true on this when a signal is received
	noteCutCount int       // # of times polyphony limit was exceeded

	// ignore signalContinue messages with world < this.
	// increment world when signalStop and signalStart are sent.
	world int
}

// create a new player
func newPlayer(s *song, wr writer.ChannelWriter, realtime bool) *player {
	p := &player{
		song:         s,
		realtime:     realtime,
		horizon:      make(map[int]int64),
		bpm:          defaultBPM,
		signal:       make(chan playerSignal),
		stopping:     make(chan struct{}),
		writer:       wr,
		midiChannels: make([]*channelState, numMIDIChannels),
		virtChannels: make([]*channelState, numVirtualChannels),
	}
	for i := range p.midiChannels {
		p.midiChannels[i] = newChannelState()
	}
	for i := range p.virtChannels {
		p.virtChannels[i] = newChannelState()
	}
	return p
}

// start signal-handling loop
func (p *player) run() {
	p.broadcastPitchBendRPN(bendSemitones, 0)
	for sig := range p.signal {
		switch sig.typ {
		case signalStart:
			p.world++
			for _, c := range p.midiChannels {
				c.lastNoteOff = 0 // reset; all channels are fair game now
			}
			p.determineVirtualChannelStates(sig.tick)
			p.lastTick = sig.tick
			p.findHorizon()
			for i := range p.song.Tracks {
				p.playTrackEvents(i, sig.tick, sig.tick)
			}
			go func() {
				p.signal <- playerSignal{
					typ:   signalContinue,
					tick:  sig.tick,
					world: p.world,
				}
			}()
		case signalContinue:
			if sig.world < p.world {
				break
			}

			if wr, ok := p.writer.(*writer.SMF); ok {
				wr.SetDelta(uint32(sig.tick - p.lastTick))
			}

			for i := range p.song.Tracks {
				p.playTrackEvents(i, p.lastTick+1, sig.tick)
			}

			p.lastTick = sig.tick
			p.findHorizon()

			go func() {
				if tth, ok := p.ticksToHorizon(); ok {
					sig2 := playerSignal{
						typ:   signalContinue,
						tick:  p.lastTick + tth,
						world: p.world,
					}
					if p.realtime {
						time.Sleep(p.durationFromTicks(tth))
					}
					p.signal <- sig2
				} else {
					p.signal <- playerSignal{typ: signalStop}
				}
			}()
		case signalStop:
			p.world++
			for i := range p.song.Tracks {
				p.noteOff(i, p.lastTick)
			}
			if p.sendStopping {
				p.stopping <- struct{}{}
			}
		case signalEvent:
			p.playEvent(sig.event)
		case signalSendPitchRPN:
			p.broadcastPitchBendRPN(bendSemitones, 0)
		case signalSendGMSystemOn:
			writer.SysEx(p.writer, []byte{0x7e, 0x7f, 0x09, 0x01})
			for i := range p.midiChannels {
				p.virtChannels[i] = newChannelState()
			}
			for i := range p.midiChannels {
				p.midiChannels[i] = newChannelState()
			}
		case signalResetChannels:
			for i := range p.midiChannels {
				p.virtChannels[i] = newChannelState()
			}
		case signalSongChanged:
			p.findHorizon()
		}

		// if we got any signal, assume redraw is needed
		if p.redrawChan != nil {
			p.redrawChan <- true
		}
	}
}

// clean up active notes and reset pitch bend sensitivity to default
func (p *player) cleanup() {
	for i := range p.song.Tracks {
		p.noteOff(i, p.lastTick)
	}
	p.broadcastPitchBendRPN(2, 0)
}

// send the "pitch bend sensitivity" RPN to every channel
func (p *player) broadcastPitchBendRPN(semitones, cents uint8) {
	for i := uint8(0); i < numMIDIChannels; i++ {
		p.writer.SetChannel(i)
		writer.RPN(p.writer, 0, 0, semitones, cents)
	}
}

// find the next tick when something happens in each track
func (p *player) findHorizon() {
	for i := range p.song.Tracks {
		p.findTrackHorizon(i)
	}
}

// find last horizon only for a specific track
func (p *player) findTrackHorizon(i int) {
	p.horizon[i] = math.MaxInt64
	if i >= len(p.song.Tracks) {
		return
	}
	t := p.song.Tracks[i]
	for _, te := range t.Events {
		if te.Tick > p.lastTick && te.Tick < p.horizon[i] {
			p.horizon[i] = te.Tick
		}
	}
}

// returnt he ticks until the next event
func (p *player) ticksToHorizon() (int64, bool) {
	horizon, ok := int64(math.MaxInt64), false
	for _, tick := range p.horizon {
		if tick > p.lastTick && tick < horizon {
			horizon, ok = tick, true
		}
	}
	return horizon - p.lastTick, ok
}

// return the time until the next event
func (p *player) timeToHorizon() (time.Duration, bool) {
	ticks, ok := p.ticksToHorizon()
	return p.durationFromTicks(ticks), ok
}

// convert a tick count to a time.Duration
func (p *player) durationFromTicks(t int64) time.Duration {
	return time.Duration(int64(float64(int64(time.Minute)*t/ticksPerBeat) / p.bpm))
}

// play events on track i in the tick range [tickMin, tickMax]
func (p *player) playTrackEvents(i int, tickMin, tickMax int64) {
	for _, te := range p.song.Tracks[i].Events {
		if te.Tick >= tickMin && te.Tick <= tickMax {
			p.playEvent(te)
		}
	}
}

// play a single event; i is track index
func (p *player) playEvent(te *trackEvent) {
	i := te.track
	t := p.song.Tracks[i]
	switch te.Type {
	case noteOnEvent:
		p.noteOff(i, te.Tick)
		vcs := p.virtChannels[t.Channel]
		var stolen bool
		t.midiChannel, stolen = pickInactiveChannel(p.midiChannels, vcs.midiMin, vcs.midiMax)
		if stolen {
			p.noteCutCount++
		}
		p.writer.SetChannel(t.midiChannel)
		mcs := p.midiChannels[t.midiChannel]
		for i, v := range vcs.controllers {
			if mcs.controllers[i] != v {
				writer.ControlChange(p.writer, uint8(i), v)
				mcs.controllers[i] = v
			}
		}
		if mcs.program != vcs.program {
			writer.ProgramChange(p.writer, vcs.program)
			mcs.program = vcs.program
		}
		if mcs.pressure != vcs.pressure {
			writer.Aftertouch(p.writer, vcs.pressure)
			mcs.pressure = vcs.pressure
		}
		note, bend := pitchToMIDI(te.FloatData)
		vcs.bend = bend
		if mcs.bend != bend {
			writer.Pitchbend(p.writer, bend)
			mcs.bend = bend
		}
		if mcs.keyPressure[note] != t.pressure {
			writer.PolyAftertouch(p.writer, note, t.pressure)
			mcs.keyPressure[note] = t.pressure
		}
		writer.NoteOn(p.writer, note, te.ByteData1)
		t.activeNote = note
		mcs.lastNoteOff = -1
	case drumNoteOnEvent:
		p.noteOff(i, te.Tick)
		t.midiChannel = percussionChannelIndex
		p.writer.SetChannel(t.midiChannel)
		mcs := p.midiChannels[t.midiChannel]
		vcs := p.virtChannels[t.Channel]
		if mcs.program != vcs.program {
			writer.ProgramChange(p.writer, vcs.program)
		}
		writer.NoteOn(p.writer, te.ByteData1, te.ByteData2)
		t.activeNote = te.ByteData1
		mcs.lastNoteOff = -1
	case noteOffEvent:
		p.noteOff(i, te.Tick)
	case controllerEvent:
		p.virtChannels[t.Channel].controllers[te.ByteData1] = te.ByteData2
		for _, t2 := range p.song.Tracks {
			if t2.Channel == t.Channel && t2.midiChannel != byteNil {
				p.writer.SetChannel(t2.midiChannel)
				writer.ControlChange(p.writer, te.ByteData1, te.ByteData2)
				p.midiChannels[t2.midiChannel].controllers[te.ByteData1] = te.ByteData2
			}
		}
	case pitchBendEvent:
		if note := t.activeNote; note != byteNil {
			bend := int16((te.FloatData - float64(note)) * 8192.0 / bendSemitones)
			p.virtChannels[t.Channel].bend = bend
			p.writer.SetChannel(t.midiChannel)
			writer.Pitchbend(p.writer, bend)
			p.midiChannels[t.midiChannel].bend = bend
		}
	case channelPressureEvent:
		p.virtChannels[t.Channel].pressure = te.ByteData1
		for _, t2 := range p.song.Tracks {
			if t2.Channel == t.Channel && t2.midiChannel != byteNil {
				p.writer.SetChannel(t2.midiChannel)
				writer.Aftertouch(p.writer, te.ByteData1)
				p.midiChannels[t2.midiChannel].pressure = te.ByteData1
			}
		}
	case keyPressureEvent:
		t.pressure = te.ByteData1
		if t.activeNote != byteNil {
			p.writer.SetChannel(t.midiChannel)
			writer.PolyAftertouch(p.writer, t.activeNote, t.pressure)
			p.midiChannels[t.midiChannel].keyPressure[t.activeNote] = t.pressure
		}
	case programEvent:
		p.virtChannels[t.Channel].program = te.ByteData1
		for _, t2 := range p.song.Tracks {
			if t2.Channel == t.Channel && t2.midiChannel != byteNil {
				p.writer.SetChannel(t2.midiChannel)
				writer.ProgramChange(p.writer, te.ByteData1)
				p.midiChannels[t2.midiChannel].program = te.ByteData1
			}
		}
	case tempoEvent:
		p.bpm = te.FloatData
		if wr, ok := p.writer.(*writer.SMF); ok {
			writer.TempoBPM(wr, te.FloatData)
		}
	case textEvent:
		if wr, ok := p.writer.(*writer.SMF); ok {
			switch te.ByteData1 {
			case 1:
				writer.Text(wr, te.TextData)
			case 2:
				writer.Copyright(wr, te.TextData)
			case 3:
				writer.TrackSequenceName(wr, te.TextData)
			case 4:
				writer.Instrument(wr, te.TextData)
			case 5:
				writer.Lyric(wr, te.TextData)
			case 6:
				writer.Marker(wr, te.TextData)
			case 7:
				writer.Cuepoint(wr, te.TextData)
			case 8:
				writer.Program(wr, te.TextData)
			case 9:
				writer.Device(wr, te.TextData)
			default:
				println("unhandled text event type in player.playTrackEvents")
			}
		}
	case releaseLenEvent:
		p.virtChannels[t.Channel].releaseLen = int64(math.Round(te.FloatData * ticksPerBeat))
	case midiRangeEvent:
		vcs := p.virtChannels[t.Channel]
		vcs.midiMin, vcs.midiMax = te.ByteData1, te.ByteData2
		for _, t2 := range p.song.Tracks {
			if t2.Channel == t.Channel &&
				(t2.midiChannel < vcs.midiMin || t2.midiChannel > vcs.midiMax) {
				t2.midiChannel = vcs.midiMin
			}
		}
	default:
		println("unhandled event type in player.playTrackEvents")
	}
}

// if a note is playing on the indexed track, play a note off
func (p *player) noteOff(i int, tick int64) {
	t := p.song.Tracks[i]
	if activeNote := t.activeNote; activeNote != byteNil {
		p.writer.SetChannel(t.midiChannel)
		writer.NoteOff(p.writer, activeNote)
		t.activeNote = byteNil
		p.midiChannels[t.midiChannel].lastNoteOff = tick + p.virtChannels[t.Channel].releaseLen
	}
}

// set virtual channel states based on everything that happens from the start
// of the song up to (but not including) a given tick
// TODO this seems expensive to do every time play needs to happen; it's
// probably worth looking into keeping events sorted in the first place. it
// would also be cheaper to have a different slice for each virtual channel,
// since channels can't affect each other's states and (n log n + m log m) <
// (n+m log n+m)
func (p *player) determineVirtualChannelStates(tick int64) {
	events := []*trackEvent{}
	for _, t := range p.song.Tracks {
		for _, te := range t.Events {
			if te.Tick < tick {
				events = append(events, te)
			}
		}
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].Tick < events[j].Tick {
			return true
		} else if events[i].Tick == events[j].Tick && events[i].track < events[j].track {
			return true
		}
		return false
	})
	for _, te := range events {
		t := p.song.Tracks[te.track]
		switch te.Type {
		case controllerEvent:
			p.virtChannels[t.Channel].controllers[te.ByteData1] = te.ByteData2
		case programEvent:
			p.virtChannels[t.Channel].program = te.ByteData1
		case tempoEvent:
			p.bpm = te.FloatData
		}
	}
}

// send a stop signal to the player, waiting if await is true
func (p *player) stop(await bool) {
	p.sendStopping = true
	p.signal <- playerSignal{typ: signalStop}
	<-p.stopping
	p.sendStopping = false
}

// type that tracks state of a midi or virtual channel
type channelState struct {
	lastNoteOff int64
	program     uint8
	controllers [128]uint8
	bend        int16
	pressure    uint8
	keyPressure [128]uint8
	releaseLen  int64
	midiMin     uint8 // only used by virtual channels
	midiMax     uint8 // ^
}

// return an initialized channelState, using the default controller values from
// "GM level 1 developer guidelines - second revision"
func newChannelState() *channelState {
	cs := &channelState{
		midiMin: 0,
		midiMax: numMIDIChannels - 1,
	}
	cs.controllers[7] = 100    // volume
	cs.controllers[10] = 64    // pan
	cs.controllers[11] = 127   // expression
	cs.controllers[100] = 0x7f // RPN LSB
	cs.controllers[101] = 0x7f // RPN MSB
	return cs
}

// return the index of the channel which has had no active notes for the
// longest time, aside from the percussion channel; return true if voice was
// stolen
func pickInactiveChannel(a []*channelState, min, max uint8) (uint8, bool) {
	bestScore, bestIndex := int64(math.MaxInt64), min
	for i, cs := range a {
		if i >= int(min) && i <= int(max) &&
			i != percussionChannelIndex && cs.lastNoteOff != -1 && cs.lastNoteOff < bestScore {
			bestScore, bestIndex = cs.lastNoteOff, uint8(i)
		}
	}
	return bestIndex, bestScore == int64(math.MaxInt64)
}
