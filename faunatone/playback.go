package main

import (
	"math"
	"sort"
	"sync"
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
	signalSongChanged
	signalSendPitchRPN
	signalSendSystemOn
	signalResetChannels
	signalCycleMIDIMode
)

const (
	defaultBPM = 120
	byteNil    = 0xff

	ccBankMSB = 0
	ccBankLSB = 32

	mt32MinChannel = 1
	mt32MaxChannel = 8
)

var systemOnBytes = [][]byte{
	{0x7e, 0x7f, 0x09, 0x01},                               // GM
	{0x41, 0x10, 0x42, 0x12, 0x40, 0x00, 0x7f, 0x00, 0x41}, // GS
	{0x43, 0x10, 0x4c, 0x00, 0x00, 0x7e, 0x00},             // XG
	{0x41, 0x10, 0x16, 0x12, 0x7f, 0x01},                   // MT-32
}

// type that writes midi events over time according to a song
type player struct {
	song         *song
	realtime     bool
	lastTick     int64
	lastEvtTick  int64         // tick of last event which was written
	horizon      map[int]int64 // map of tracks to ticks
	horizonMutex sync.Mutex
	bpm          float64
	signal       chan playerSignal
	stopping     chan struct{} // player sends on this channel when stopping
	sendStopping bool          // but only if this is true
	outputs      []*midiOutput
	virtChannels []*channelState
	redrawChan   chan bool // send true on this when a signal is received
	polyErrCount int       // # of times polyphony limit was exceeded

	// ignore signalContinue messages with world < this.
	// increment world when signalStop and signalStart are sent.
	world int
}

type midiOutput struct {
	writer   writer.ChannelWriter
	channels []*channelState
}

// create a new player
func newPlayer(s *song, wrs []writer.ChannelWriter, realtime bool) *player {
	p := &player{
		song:         s,
		realtime:     realtime,
		horizon:      make(map[int]int64),
		bpm:          defaultBPM,
		signal:       make(chan playerSignal),
		stopping:     make(chan struct{}),
		outputs:      make([]*midiOutput, len(wrs)),
		virtChannels: make([]*channelState, numVirtualChannels),
	}
	for i, wr := range wrs {
		out := &midiOutput{
			writer:   wr,
			channels: make([]*channelState, numMidiChannels),
		}
		for i := range out.channels {
			out.channels[i] = newChannelState(s.MidiMode, i, false)
		}
		p.outputs[i] = out
	}
	for i := range p.virtChannels {
		p.virtChannels[i] = newChannelState(s.MidiMode, i, true)
	}
	return p
}

// start signal-handling loop
func (p *player) run() {
	p.broadcastPitchBendRPN(uint8(bendSemitones), 0)
	for sig := range p.signal {
		switch sig.typ {
		case signalStart:
			p.world++
			for _, out := range p.outputs {
				for _, c := range out.channels {
					c.lastNoteOff = 0 // reset; all channels are fair game now
				}
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

			for _, out := range p.outputs {
				if wr, ok := out.writer.(*writer.SMF); ok {
					wr.SetDelta(uint32(sig.tick - p.lastEvtTick))
				}
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
			p.broadcastPitchBendRPN(uint8(bendSemitones), 0)
		case signalSendSystemOn:
			for _, out := range p.outputs {
				sendGMSystemOn(out.writer, p.song.MidiMode)
				for i := range out.channels {
					out.channels[i] = newChannelState(p.song.MidiMode, i, false)
				}
			}
			for i := range p.virtChannels {
				p.virtChannels[i] = newChannelState(p.song.MidiMode, i, true)
			}
		case signalResetChannels:
			for i := range p.virtChannels {
				p.virtChannels[i] = newChannelState(p.song.MidiMode, i, true)
			}
		case signalSongChanged:
			p.findHorizon()
		case signalCycleMIDIMode:
			p.song.MidiMode = (p.song.MidiMode + 1) % numMidiModes
			go func() {
				p.signal <- playerSignal{typ: signalSendSystemOn}
				p.signal <- playerSignal{typ: signalSendPitchRPN}
			}()
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
	for _, out := range p.outputs {
		for i := uint8(0); i < numMidiChannels; i++ {
			out.writer.SetChannel(i)
			writer.RPN(out.writer, 0, 0, semitones, cents)
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
	p.horizonMutex.Lock()
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
	p.horizonMutex.Unlock()
}

// return the ticks until the next event
func (p *player) ticksToHorizon() (int64, bool) {
	horizon, ok := int64(math.MaxInt64), false
	p.horizonMutex.Lock()
	for _, tick := range p.horizon {
		if tick > p.lastTick && tick < horizon {
			horizon, ok = tick, true
		}
	}
	p.horizonMutex.Unlock()
	return horizon - p.lastTick, ok
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

// return the midi output for a track.
// always returns non-nil if the player has at least one output.
func (p *player) trackOutput(t *track) *midiOutput {
	i := p.virtChannels[t.Channel].output
	if i >= len(p.outputs) {
		i = len(p.outputs) - 1
	}
	return p.outputs[i]
}

// play a single event; i is track index
func (p *player) playEvent(te *trackEvent) {
	i := te.track
	t := p.song.Tracks[i]
	out := p.trackOutput(t)
	switch te.Type {
	case noteOnEvent:
		p.lastEvtTick = te.Tick
		p.noteOff(i, te.Tick)
		vcs := p.virtChannels[t.Channel]
		var stolen bool
		t.midiChannel, stolen = pickInactiveChannel(out.channels, vcs.midiMin, vcs.midiMax, p.song.MidiMode)
		for j, t2 := range p.song.Tracks {
			vcs2 := p.virtChannels[t2.Channel]
			if t2.Channel != t.Channel &&
				vcs2.output == vcs.output &&
				t2.midiChannel == t.midiChannel {
				if t2.activeNote != byteNil {
					p.noteOff(j, te.Tick)
				}
				t2.midiChannel = byteNil
			}
		}
		if stolen && !vcs.isPercussionChannel() {
			p.polyErrCount++
		}
		out.writer.SetChannel(t.midiChannel)
		mcs := out.channels[t.midiChannel]
		for i, v := range vcs.controllers {
			if mcs.controllers[i] != v {
				writer.ControlChange(out.writer, uint8(i), v)
				mcs.controllers[i] = v
			}
		}
		if mcs.program != vcs.program {
			writer.ControlChange(out.writer, ccBankMSB, uint8(vcs.program>>8))
			writer.ControlChange(out.writer, ccBankLSB, uint8(vcs.program>>16))
			writer.ProgramChange(out.writer, uint8(vcs.program))
			mcs.program = vcs.program
		}
		if mcs.pressure != vcs.pressure {
			writer.Aftertouch(out.writer, vcs.pressure)
			mcs.pressure = vcs.pressure
		}
		note, bend := pitchToMidi(te.FloatData, p.song.MidiMode)
		vcs.bend = bend
		if mcs.bend != bend {
			writer.Pitchbend(out.writer, bend)
			mcs.bend = bend
		}
		if mcs.keyPressure[note] != t.pressure {
			writer.PolyAftertouch(out.writer, note, t.pressure)
			mcs.keyPressure[note] = t.pressure
		}
		writer.NoteOn(out.writer, note, te.ByteData1)
		t.activeNote = note
		mcs.lastNoteOff = -1
	case drumNoteOnEvent:
		p.lastEvtTick = te.Tick
		p.noteOff(i, te.Tick)
		t.midiChannel = percussionChannelIndex
		vcs := p.virtChannels[t.Channel]
		out.writer.SetChannel(t.midiChannel)
		mcs := out.channels[t.midiChannel]
		if mcs.program != vcs.program {
			writer.ProgramChange(out.writer, uint8(vcs.program))
		}
		writer.NoteOn(out.writer, te.ByteData1, te.ByteData2)
		t.activeNote = te.ByteData1
		mcs.lastNoteOff = -1
	case noteOffEvent:
		p.noteOff(i, te.Tick)
	case controllerEvent:
		p.virtChannels[t.Channel].controllers[te.ByteData1] = te.ByteData2
		for _, t2 := range p.song.Tracks {
			if t2.Channel == t.Channel && t2.midiChannel != byteNil {
				p.lastEvtTick = te.Tick
				out.writer.SetChannel(t2.midiChannel)
				writer.ControlChange(out.writer, te.ByteData1, te.ByteData2)
				out.channels[t2.midiChannel].controllers[te.ByteData1] = te.ByteData2
			}
		}
	case pitchBendEvent:
		if note := t.activeNote; note != byteNil {
			p.lastEvtTick = te.Tick
			bend := int16((te.FloatData - float64(note)) * 8192.0 /
				getBendSemitones(p.song.MidiMode))
			p.virtChannels[t.Channel].bend = bend
			out.writer.SetChannel(t.midiChannel)
			writer.Pitchbend(out.writer, bend)
			out.channels[t.midiChannel].bend = bend
		}
	case channelPressureEvent:
		p.virtChannels[t.Channel].pressure = te.ByteData1
		for _, t2 := range p.song.Tracks {
			if t2.Channel == t.Channel && t2.midiChannel != byteNil {
				p.lastEvtTick = te.Tick
				out.writer.SetChannel(t2.midiChannel)
				writer.Aftertouch(out.writer, te.ByteData1)
				out.channels[t2.midiChannel].pressure = te.ByteData1
			}
		}
	case keyPressureEvent:
		t.pressure = te.ByteData1
		if t.activeNote != byteNil {
			p.lastEvtTick = te.Tick
			out.writer.SetChannel(t.midiChannel)
			writer.PolyAftertouch(out.writer, t.activeNote, t.pressure)
			out.channels[t.midiChannel].keyPressure[t.activeNote] = t.pressure
		}
	case programEvent:
		p.virtChannels[t.Channel].program = uint32(te.ByteData1) +
			uint32(te.ByteData2)<<8 +
			uint32(te.ByteData3)<<16
		for _, t2 := range p.song.Tracks {
			if t2.Channel == t.Channel && t2.midiChannel != byteNil {
				p.lastEvtTick = te.Tick
				out.writer.SetChannel(t2.midiChannel)
				writer.ControlChange(out.writer, ccBankMSB, te.ByteData2)
				writer.ControlChange(out.writer, ccBankLSB, te.ByteData3)
				writer.ProgramChange(out.writer, te.ByteData1)
				out.channels[t2.midiChannel].program = p.virtChannels[t.Channel].program
			}
		}
	case tempoEvent:
		if te.FloatData != 0 {
			p.bpm = te.FloatData
		} else {
			p.bpm *= float64(te.ByteData1) / float64(te.ByteData2)
		}
		if wr, ok := out.writer.(*writer.SMF); ok {
			p.lastEvtTick = te.Tick
			writer.TempoBPM(wr, p.bpm)
		}
	case textEvent:
		if wr, ok := out.writer.(*writer.SMF); ok {
			p.lastEvtTick = te.Tick
			switch te.ByteData1 {
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
				writer.Text(wr, te.TextData)
			}
		}
	case releaseLenEvent:
		p.virtChannels[t.Channel].releaseLen = int64(math.Round(te.FloatData * ticksPerBeat))
	case midiRangeEvent:
		vcs := p.virtChannels[t.Channel]
		vcs.midiMin, vcs.midiMax = te.ByteData1, te.ByteData2
		if vcs.midiMin == vcs.midiMax {
			for _, t2 := range p.song.Tracks {
				if t2.Channel == t.Channel {
					t2.midiChannel = vcs.midiMin
				}
			}
		}
	case midiOutputEvent:
		vcs := p.virtChannels[t.Channel]
		vcs.output = int(te.ByteData1)
	case mt32ReverbEvent:
		p.lastEvtTick = te.Tick
		// mode, time, level
		sysex([]byte{0x41, 0x10, 0x16, 0x12, 0x10, 0x00, 0x01, te.ByteData1},
			out.writer, p.song.MidiMode)
		sysex([]byte{0x41, 0x10, 0x16, 0x12, 0x10, 0x00, 0x02, te.ByteData2},
			out.writer, p.song.MidiMode)
		sysex([]byte{0x41, 0x10, 0x16, 0x12, 0x10, 0x00, 0x03, te.ByteData3},
			out.writer, p.song.MidiMode)
	default:
		println("unhandled event type in player.playTrackEvents")
	}
}

func sysex(data []byte, wr writer.ChannelWriter, midiMode int) {
	b := make([]byte, len(data))
	copy(b, data)
	// MT-32 sysexes require checksum
	if midiMode == modeMT32 && len(b) >= 5 {
		b = append(b, calcRolandChecksum(b))
	}
	if err := writer.SysEx(wr, b); err != nil {
		println(err.Error())
	}
}

// calculate the checksum byte for a MT-32 sysex message
func calcRolandChecksum(b []byte) byte {
	sum := 0
	for _, n := range b[4:] {
		sum += int(n)
	}
	sum = (0x80 - (sum % 0x80)) % 0x80 // https://github.com/shingo45endo/sysex-checksum/blob/main/sysex_parser.js
	return byte(sum)
}

// if a note is playing on the indexed track, play a note off
func (p *player) noteOff(i int, tick int64) {
	t := p.song.Tracks[i]
	if activeNote := t.activeNote; activeNote != byteNil {
		p.lastEvtTick = tick
		out := p.trackOutput(t)
		out.writer.SetChannel(t.midiChannel)
		writer.NoteOff(out.writer, activeNote)
		t.activeNote = byteNil
		out.channels[t.midiChannel].lastNoteOff = tick + p.virtChannels[t.Channel].releaseLen
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
			p.virtChannels[t.Channel].program = uint32(te.ByteData1) +
				uint32(te.ByteData2)<<8 +
				uint32(te.ByteData3)<<16
		case tempoEvent:
			if te.FloatData != 0 {
				p.bpm = te.FloatData
			} else {
				p.bpm *= float64(te.ByteData1) / float64(te.ByteData2)
			}
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
	program     uint32
	controllers [128]uint8
	bend        int16
	pressure    uint8
	keyPressure [128]uint8
	releaseLen  int64
	midiMin     uint8 // only used by virtual channels
	midiMax     uint8 // ^
	output      int   // device index
}

var (
	// panning is a guess
	mt32DefaultPrograms = []uint32{0, 68, 48, 95, 78, 41, 3, 110, 122, 0, 0, 0, 0, 0, 0, 0}
	mt32DefaultPanning  = []uint8{64, 48, 80, 32, 96, 16, 112, 0, 127, 64, 64, 64, 64, 64, 64, 64}
)

// return an initialized channelState, using the default controller values from
// "GM level 1 developer guidelines - second revision"
func newChannelState(midiMode, index int, virtual bool) *channelState {
	cs := &channelState{
		midiMin: 0,
		midiMax: numMidiChannels - 1,
	}
	cs.controllers[7] = 100    // volume
	cs.controllers[10] = 64    // pan
	cs.controllers[11] = 127   // expression
	cs.controllers[100] = 0x7f // RPN LSB
	cs.controllers[101] = 0x7f // RPN MSB
	if midiMode == modeXG {
		cs.controllers[71] = 0x40  // harmonic content
		cs.controllers[72] = 0x40  // release time
		cs.controllers[73] = 0x40  // attack time
		cs.controllers[74] = 0x40  // brightness
		cs.controllers[91] = 0x28  // reverb send level
		cs.controllers[100] = 0x7f // RPN LSB
		cs.controllers[101] = 0x7f // RPN MSB
	} else if midiMode == modeMT32 {
		cs.program = mt32DefaultPrograms[index]
		cs.controllers[10] = mt32DefaultPanning[index]
	}
	return cs
}

func (cs *channelState) isPercussionChannel() bool {
	return cs.midiMin == percussionChannelIndex && cs.midiMax == percussionChannelIndex
}

// return the index of the channel which has had no active notes for the
// longest time, aside from the percussion channel; return true if voice was
// stolen
func pickInactiveChannel(a []*channelState, min, max uint8, midiMode int) (uint8, bool) {
	if midiMode == modeMT32 {
		min = clamp(min, mt32MinChannel, mt32MaxChannel)
		max = clamp(max, mt32MinChannel, mt32MaxChannel)
	}
	bestScore, bestIndex := int64(math.MaxInt64), min
	for i, cs := range a {
		if i >= int(min) && i <= int(max) &&
			i != percussionChannelIndex && cs.lastNoteOff != -1 && cs.lastNoteOff < bestScore {
			bestScore, bestIndex = cs.lastNoteOff, uint8(i)
		}
	}
	return bestIndex, bestScore == int64(math.MaxInt64)
}

// clamp x between min and max inclusive
func clamp(x, min, max uint8) uint8 {
	if x < min {
		return min
	} else if x > max {
		return max
	}
	return x
}
