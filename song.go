package main

import (
	"compress/zlib"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"

	"gitlab.com/gomidi/midi/writer"
)

type trackEventType byte

const (
	noteOffEvent trackEventType = iota
	noteOnEvent
	controllerEvent
	programEvent
	tempoEvent
	drumNoteOnEvent
)

const (
	numMIDIChannels        = 16
	percussionChannelIndex = 9
)

// fields in these types are exported to expose them to the JSON encoder

type song struct {
	Title  string
	Tracks []*track
}

// decode song data; if successful, the current song data is replaced
func (s *song) read(r io.Reader) error {
	comp, err := zlib.NewReader(r)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(comp)
	newSong := &song{}
	if err := dec.Decode(newSong); err != nil {
		return err
	}
	if err := comp.Close(); err != nil {
		return err
	}
	*s = *newSong
	for _, t := range s.Tracks {
		for _, te := range t.Events {
			te.setUiString()
		}
	}
	return nil
}

// encode song data
func (s *song) write(w io.Writer) error {
	comp := zlib.NewWriter(w)
	enc := json.NewEncoder(comp)
	if err := enc.Encode(s); err != nil {
		return err
	}
	return comp.Close()
}

// export to MIDI
func (s *song) exportSMF(path string) error {
	// first collate all events and sort
	events := []*trackEvent{}
	for i, t := range s.Tracks {
		t.activeNote = 0xff
		for _, te := range t.Events {
			te.trackIndex = i
			te.track = t
			events = append(events, te)
		}
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].Tick < events[j].Tick {
			return true
		} else if events[i].Tick == events[j].Tick {
			return events[i].trackIndex < events[j].trackIndex
		}
		return false
	})

	// set up data structures for tracking channels
	channelStates := make([]*channelState, 16)
	for i := range channelStates {
		channelStates[i] = &channelState{}
	}
	vcPrograms := make(map[uint8]uint8)

	// then write file
	return writer.WriteSMF(path, 1, func(wr *writer.SMF) error {
		if s.Title != "" {
			writer.TrackSequenceName(wr, s.Title)
		}
		prevTick := int64(0)
		for _, te := range events {
			wr.SetDelta(uint32(te.Tick - prevTick))
			prevTick = te.Tick
			switch te.Type {
			case noteOnEvent:
				if activeNote := te.track.activeNote; activeNote != 0xff {
					wr.SetChannel(te.track.midiChannel)
					writer.NoteOff(wr, activeNote)
					te.track.activeNote = 0xff
					cs := channelStates[te.track.midiChannel]
					cs.lastNoteOff = prevTick
				}
				te.track.midiChannel = pickInactiveChannel(channelStates)
				cs := channelStates[te.track.midiChannel]
				wr.SetChannel(te.track.midiChannel)
				if cs.program != vcPrograms[te.track.Channel] {
					writer.ProgramChange(wr, vcPrograms[te.track.Channel])
				}
				note, bend := pitchToMIDI(te.FloatData)
				if cs.bend != bend {
					writer.Pitchbend(wr, bend)
				}
				writer.NoteOn(wr, note, te.ByteData1)
				te.track.activeNote = note
				cs.lastNoteOff = -1
				cs.bend = bend
			case drumNoteOnEvent:
				if activeNote := te.track.activeNote; activeNote != 0xff {
					wr.SetChannel(te.track.midiChannel)
					writer.NoteOff(wr, activeNote)
					te.track.activeNote = 0xff
					cs := channelStates[te.track.midiChannel]
					cs.lastNoteOff = prevTick
				}
				te.track.midiChannel = percussionChannelIndex
				cs := channelStates[te.track.midiChannel]
				wr.SetChannel(te.track.midiChannel)
				if cs.program != vcPrograms[te.track.Channel] {
					writer.ProgramChange(wr, vcPrograms[te.track.Channel])
				}
				writer.NoteOn(wr, te.ByteData1, te.ByteData2)
				te.track.activeNote = te.ByteData1
				cs.lastNoteOff = -1
			case noteOffEvent:
				if activeNote := te.track.activeNote; activeNote != 0xff {
					wr.SetChannel(te.track.midiChannel)
					writer.NoteOff(wr, activeNote)
					te.track.activeNote = 0xff
					cs := channelStates[te.track.midiChannel]
					cs.lastNoteOff = prevTick
				}
			case programEvent:
				vcPrograms[te.track.Channel] = te.ByteData1
			default:
				println("unhandled event type in song.exportSMF")
			}
		}
		writer.EndOfTrack(wr)
		return nil
	})
}

type channelState struct {
	eventIndex  int
	lastNoteOff int64
	program     uint8
	controllers [128]uint8
	bend        int16
}

// returns the index of the channel which has had no active notes for the
// longest time, aside from the percussion channel
func pickInactiveChannel(a []*channelState) uint8 {
	bestScore, bestIndex := int64(math.MaxInt64), 0
	for i, cs := range a {
		if i != percussionChannelIndex && cs.lastNoteOff < bestScore {
			bestScore, bestIndex = cs.lastNoteOff, i
		}
	}
	return uint8(bestIndex)
}

type track struct {
	Channel     uint8
	Events      []*trackEvent
	activeNote  uint8
	midiChannel uint8
}

// write an event to the track, overwriting any event at the same tick
func (t *track) writeEvent(te *trackEvent) {
	for _, te2 := range t.Events {
		if te2.Tick == te.Tick {
			*te2 = *te
			return
		}
	}
	t.Events = append(t.Events, te)
}

type trackEvent struct {
	Tick       int64
	Type       trackEventType
	FloatData  float64 `json:",omitempty"`
	ByteData1  byte    `json:",omitempty"`
	ByteData2  byte    `json:",omitempty"`
	uiString   string
	trackIndex int
	track      *track // used by export
}

func newTrackEvent(te *trackEvent) *trackEvent {
	te.setUiString()
	return te
}

func (te *trackEvent) setUiString() {
	switch te.Type {
	case noteOnEvent:
		te.uiString = fmt.Sprintf("on %.2f %d", te.FloatData, te.ByteData1)
	case drumNoteOnEvent:
		te.uiString = fmt.Sprintf("dr %d %d", te.ByteData1, te.ByteData2)
	case noteOffEvent:
		te.uiString = "off"
	case controllerEvent:
		te.uiString = fmt.Sprintf("cc %d %d", te.ByteData1, te.ByteData2)
	case programEvent:
		te.uiString = fmt.Sprintf("prog %d", te.ByteData1+1)
	case tempoEvent:
		te.uiString = fmt.Sprintf("tp %.2f", te.FloatData)
	default:
		te.uiString = "UNKNOWN"
	}
}
