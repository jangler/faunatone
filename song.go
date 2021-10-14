package main

import (
	"compress/zlib"
	"encoding/json"
	"fmt"
	"io"
)

type trackEventType byte

const (
	noteOffEvent trackEventType = iota
	noteOnEvent
	controllerEvent
	programEvent
	tempoEvent
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

type track struct {
	ChannelMask uint16
	Events      []*trackEvent
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
	Tick      int64
	Type      trackEventType
	FloatData float64 `json:",omitempty"`
	ByteData1 byte    `json:",omitempty"`
	ByteData2 byte    `json:",omitempty"`
	uiString  string
}

func newTrackEvent(te *trackEvent) *trackEvent {
	te.setUiString()
	return te
}

func (te *trackEvent) setUiString() {
	switch te.Type {
	case noteOnEvent:
		te.uiString = fmt.Sprintf("on %.2f %d", te.FloatData, te.ByteData1)
	case noteOffEvent:
		te.uiString = "off"
	case controllerEvent:
		te.uiString = fmt.Sprintf("cc %d %d", te.ByteData1, te.ByteData2)
	case programEvent:
		te.uiString = fmt.Sprintf("pc %d", te.ByteData1)
	case tempoEvent:
		te.uiString = fmt.Sprintf("tp %.2f", te.FloatData)
	default:
		te.uiString = "UNKNOWN"
	}
}
