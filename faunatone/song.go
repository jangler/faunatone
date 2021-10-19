package main

import (
	"compress/zlib"
	"encoding/json"
	"fmt"
	"io"

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
	pitchBendEvent
)

const (
	numMIDIChannels        = 16
	numVirtualChannels     = 16
	percussionChannelIndex = 9
)

// fields in these types are exported to expose them to the JSON encoder

type song struct {
	Title  string
	Tracks []*track
}

func newSong() *song {
	return &song{
		Tracks: []*track{
			newTrack(0, 0),
			newTrack(0, 1),
			newTrack(0, 2),
			newTrack(0, 3),
		},
	}
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
	for i, t := range s.Tracks {
		t.index = i
		for _, te := range t.Events {
			te.track = i
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
	return writer.WriteSMF(path, 1, func(wr *writer.SMF) error {
		p := newPlayer(s, wr, false)
		go p.run()
		p.sendStopping = true
		p.signal <- playerSignal{typ: signalStart}
		<-p.stopping
		writer.EndOfTrack(wr)
		return nil
	})
}

type track struct {
	Channel uint8
	Events  []*trackEvent
	index   int // only used by undo/redo

	// only used by player
	activeNote  uint8
	midiChannel uint8
}

// initialize a new track
func newTrack(channel uint8, index int) *track {
	return &track{
		Channel:     channel,
		index:       index,
		activeNote:  byteNil,
		midiChannel: byteNil,
	}
}

// return a copy of the track with nil playback data
func (t *track) clone() *track {
	t2 := newTrack(t.Channel, t.index)
	t2.Events = t.Events
	return t2
}

// write an event to the track, overwriting any event at the same tick and
// returning the event that was overwritten
func (t *track) writeEvent(te *trackEvent) *trackEvent {
	if te2 := t.getEventAtTick(te.Tick); te2 != nil {
		te3 := te2.clone()
		*te2 = *te
		return te3
	}
	t.Events = append(t.Events, te)
	return nil
}

// return the event at the tick in the track, if any
func (t *track) getEventAtTick(tick int64) *trackEvent {
	for _, te := range t.Events {
		if te.Tick == tick {
			return te
		}
	}
	return nil
}

type trackEvent struct {
	Tick      int64
	Type      trackEventType
	FloatData float64 `json:",omitempty"`
	ByteData1 byte    `json:",omitempty"`
	ByteData2 byte    `json:",omitempty"`
	uiString  string
	track     int // only used by undo/redo
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
	case pitchBendEvent:
		te.uiString = fmt.Sprintf("bend %.2f", te.FloatData)
	case programEvent:
		te.uiString = fmt.Sprintf("prog %d", te.ByteData1+1)
	case tempoEvent:
		te.uiString = fmt.Sprintf("tempo %.2f", te.FloatData)
	default:
		te.uiString = "UNKNOWN"
	}
}

// return a pointer to a copy of the event
func (te *trackEvent) clone() *trackEvent {
	te2 := &trackEvent{}
	*te2 = *te
	return te2
}
