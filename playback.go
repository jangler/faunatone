package main

import (
	"math"
	"time"

	"gitlab.com/gomidi/midi/writer"
)

// type used to signal player behavior
type playerSignal uint8

const (
	signalContinue playerSignal = iota
	signalStop
	signalSongChanged
)

const (
	defaultBPM = 120
	noNote     = 0xff
)

// type that writes midi events over time according to a song
type player struct {
	song         *song
	realtime     bool
	playing      bool
	lastTick     int64
	horizon      map[int]int64
	bpm          float64
	signal       chan playerSignal
	writer       writer.ChannelWriter
	midiChannels []*channelState
	virtChannels []*channelState
	trackStates  []*trackState
}

// create a new player
func newPlayer(s *song, wr writer.ChannelWriter, realtime bool) *player {
	p := &player{
		song:         s,
		realtime:     realtime,
		horizon:      make(map[int]int64),
		bpm:          defaultBPM,
		signal:       make(chan playerSignal),
		writer:       wr,
		midiChannels: make([]*channelState, numMIDIChannels),
		virtChannels: make([]*channelState, numVirtualChannels),
		trackStates:  make([]*trackState, len(s.Tracks)),
	}
	for i := range p.midiChannels {
		p.midiChannels[i] = &channelState{}
	}
	for i := range p.virtChannels {
		p.virtChannels[i] = &channelState{}
	}
	for i := range p.trackStates {
		p.trackStates[i] = &trackState{activeNote: noNote}
	}
	return p
}

// start playing from a given tick
func (p *player) playFrom(tick int64) {
	p.playing = true
	for _, c := range p.midiChannels {
		c.lastNoteOff = 0 // reset; all channels are fair game now
	}
	p.lastTick = tick
	p.findHorizon()
	for i := range p.horizon {
		p.playTrackEvents(i, 0, 0)
	}
	go func() {
		p.signal <- signalContinue
	}()
	for sig := range p.signal {
		switch sig {
		case signalContinue:
			if wr, ok := p.writer.(*writer.SMF); ok {
				wr.SetDelta(uint32(tick - p.lastTick))
			}
			println(tick)

			for i, h := range p.horizon {
				if h <= tick {
					p.playTrackEvents(i, p.lastTick+1, tick)
				}
			}

			p.lastTick = tick

			for i, h := range p.horizon {
				if h <= tick {
					p.findTrackHorizon(i)
				}
			}

			go func() {
				if tth, ok := p.ticksToHorizon(); ok {
					tick = p.lastTick + tth
					if p.realtime {
						time.Sleep(p.durationFromTicks(tth))
					}
					p.signal <- signalContinue
				} else {
					p.signal <- signalStop
				}
			}()
		case signalStop:
			println("stopping")
			p.playing = false
			return
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
	ts := p.trackStates[i]
	for _, te := range t.Events {
		if te.Tick >= tickMin && te.Tick <= tickMax {
			switch te.Type {
			case noteOnEvent:
				p.noteOff(i, te.Tick)
				ts.midiChannel = pickInactiveChannel(p.midiChannels)
				p.writer.SetChannel(ts.midiChannel)
				mcs := p.midiChannels[ts.midiChannel]
				vcs := p.virtChannels[t.Channel]
				if mcs.program != vcs.program {
					writer.ProgramChange(p.writer, vcs.program)
				}
				note, bend := pitchToMIDI(te.FloatData)
				// if mcs.bend != bend {
				writer.Pitchbend(p.writer, bend)
				mcs.bend = bend
				// }
				writer.NoteOn(p.writer, note, te.ByteData1)
				ts.activeNote = note
				mcs.lastNoteOff = -1
			case drumNoteOnEvent:
				p.noteOff(i, te.Tick)
				ts.midiChannel = percussionChannelIndex
				p.writer.SetChannel(ts.midiChannel)
				mcs := p.midiChannels[ts.midiChannel]
				vcs := p.virtChannels[t.Channel]
				if mcs.program != vcs.program {
					writer.ProgramChange(p.writer, vcs.program)
				}
				writer.NoteOn(p.writer, te.ByteData1, te.ByteData2)
				ts.activeNote = te.ByteData1
				mcs.lastNoteOff = -1
			case noteOffEvent:
				p.noteOff(i, te.Tick)
			case programEvent:
				p.virtChannels[t.Channel].program = te.ByteData1
			default:
				println("unhandled event type in player.playTrackEvents")
			}
		}
	}
}

// if a note is playing on the indexed track, play a note off
func (p *player) noteOff(i int, tick int64) {
	ts := p.trackStates[i]
	if activeNote := ts.activeNote; activeNote != 0xff {
		p.writer.SetChannel(ts.midiChannel)
		writer.NoteOff(p.writer, activeNote)
		ts.activeNote = 0xff
		p.midiChannels[ts.midiChannel].lastNoteOff = tick
	}
}

// type that tracks state of a midi or virtual channel
type channelState struct {
	lastNoteOff int64
	program     uint8
	controllers [128]uint8
	bend        int16
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

// type that tracks playing state of a track
type trackState struct {
	activeNote  uint8
	midiChannel uint8
}
