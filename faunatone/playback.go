package main

import (
	"math"
	"time"

	"gitlab.com/gomidi/midi/writer"
)

// type used to signal player behavior
type playerSignal struct {
	typ   playerSignalType
	tick  int64
	world int
}

type playerSignalType uint8

const (
	signalContinue playerSignalType = iota
	signalStart
	signalStop
	signalSongChanged // TODO actually use this
)

const (
	defaultBPM = 120
	noNote     = 0xff
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
	writer       writer.ChannelWriter
	midiChannels []*channelState
	virtChannels []*channelState

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
			select {
			case p.stopping <- struct{}{}:
				// do nothing
			default:
				// also do nothing
			}
		case signalSongChanged:
			p.findHorizon()
		}
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
	t := p.song.Tracks[i]
	for _, te := range t.Events {
		if te.Tick >= tickMin && te.Tick <= tickMax {
			switch te.Type {
			case noteOnEvent:
				p.noteOff(i, te.Tick)
				t.midiChannel = pickInactiveChannel(p.midiChannels)
				p.writer.SetChannel(t.midiChannel)
				mcs := p.midiChannels[t.midiChannel]
				vcs := p.virtChannels[t.Channel]
				for i, v := range vcs.controllers {
					if mcs.controllers[i] != v {
						writer.ControlChange(p.writer, uint8(i), v)
						mcs.controllers[i] = v
					}
				}
				// if mcs.program != vcs.program {
				writer.ProgramChange(p.writer, vcs.program)
				mcs.program = vcs.program
				// }
				note, bend := pitchToMIDI(te.FloatData)
				// if mcs.bend != bend {
				writer.Pitchbend(p.writer, bend)
				mcs.bend = bend
				// }
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
					if t2.Channel == t.Channel {
						p.writer.SetChannel(t2.midiChannel)
						writer.ControlChange(p.writer, te.ByteData1, te.ByteData2)
					}
				}
			case programEvent:
				// need to write an nop event here for timing reasons
				vcs := p.virtChannels[t.Channel]
				p.writer.SetChannel(t.midiChannel)
				writer.ProgramChange(p.writer, vcs.program)
				vcs.program = te.ByteData1
			case tempoEvent:
				p.bpm = te.FloatData
				if wr, ok := p.writer.(*writer.SMF); ok {
					writer.TempoBPM(wr, te.FloatData)
				}
			default:
				println("unhandled event type in player.playTrackEvents")
			}
		}
	}
}

// if a note is playing on the indexed track, play a note off
func (p *player) noteOff(i int, tick int64) {
	t := p.song.Tracks[i]
	if activeNote := t.activeNote; activeNote != 0xff {
		p.writer.SetChannel(t.midiChannel)
		writer.NoteOff(p.writer, activeNote)
		t.activeNote = 0xff
		p.midiChannels[t.midiChannel].lastNoteOff = tick
	}
}

// set virtual channel states based on everything that happens from the start
// of the song up to (but not including) a given tick
func (p *player) determineVirtualChannelStates(tick int64) {
	for _, t := range p.song.Tracks {
		for _, te := range t.Events {
			if te.Tick < tick {
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
	}
}

// type that tracks state of a midi or virtual channel
type channelState struct {
	lastNoteOff int64
	program     uint8
	controllers [128]uint8
	bend        int16
}

// return an initialized channelState, using the default controller values from
// "GM level 1 developer guidelines - second revision"
func newChannelState() *channelState {
	cs := &channelState{}
	for i := range cs.controllers {
		cs.controllers[i] = 0
	}
	cs.controllers[7] = 100    // volume
	cs.controllers[10] = 64    // pan
	cs.controllers[11] = 127   // expression
	cs.controllers[100] = 0x7f // RPN LSB
	cs.controllers[101] = 0x7f // RPN MSB
	return cs
}

// return the index of the channel which has had no active notes for the
// longest time, aside from the percussion channel
func pickInactiveChannel(a []*channelState) uint8 {
	bestScore, bestIndex := int64(math.MaxInt64), 0
	for i, cs := range a {
		if i != percussionChannelIndex && cs.lastNoteOff != -1 && cs.lastNoteOff < bestScore {
			bestScore, bestIndex = cs.lastNoteOff, i
		}
	}
	return uint8(bestIndex)
}
