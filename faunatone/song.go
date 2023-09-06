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
	keyPressureEvent
	channelPressureEvent
	textEvent
	releaseLenEvent
	midiRangeEvent
	midiOutputEvent
)

const (
	numMidiChannels        = 16
	numVirtualChannels     = 16
	percussionChannelIndex = 9
	numMidiModes           = 4
)

const (
	modeGM = iota
	modeGS
	modeXG
	modeMT32
)

var standardPitchNames = []string{
	"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B",
}

func midiModeName(index int) string {
	// remember to update numMidiModes when adding to this
	switch index {
	case modeGM:
		return "GM"
	case modeGS:
		return "GS"
	case modeXG:
		return "XG"
	case modeMT32:
		return "MT-32"
	}
	return "Unknown"
}

// fields in these types are exported to expose them to the JSON encoder

type song struct {
	Title    string
	Tracks   []*track
	Keymap   *keymap
	MidiMode int
}

func newSong(k *keymap) *song {
	if k == nil {
		k = newEmptyKeymap("none")
	}
	return &song{
		Tracks: []*track{
			newTrack(0, 0),
			newTrack(0, 1),
			newTrack(0, 2),
			newTrack(0, 3),
		},
		Keymap: k,
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
	if s.Keymap == nil {
		s.Keymap = newEmptyKeymap("none")
	}
	for _, ki := range s.Keymap.Items {
		if ki.PitchSrc == nil {
			ki.PitchSrc = newSemiPitch(ki.Interval)
		}
	}
	s.Keymap.setMidiPattern()
	s.Keymap.keyNotes = make(map[string]*trackEvent)
	s.Keymap.keySig = make(map[float64]*pitchSrc)
	for i, t := range s.Tracks {
		t.index = i
		for _, te := range t.Events {
			te.track = i
			te.setUiString(s.Keymap)
		}
	}
	return nil
}

// encode song data
func (s *song) write(w io.Writer) error {
	comp := zlib.NewWriter(w)
	enc := json.NewEncoder(comp)
	for _, ki := range s.Keymap.Items {
		ki.Interval = ki.PitchSrc.semitones() // for backward compatibility
	}
	if err := enc.Encode(s); err != nil {
		return err
	}
	return comp.Close()
}

// export to MIDI
func (s *song) exportSMF(path string) error {
	return writer.WriteSMF(path, 1, func(wr *writer.SMF) error {
		// TODO: make sure this doesn't crash things depdending on device mapping
		wr.ConsolidateNotes(false) // prevents timing issues with 0-velocity notes
		p := newPlayer(s, []writer.ChannelWriter{wr}, false)
		go p.run()
		p.sendStopping = true
		p.signal <- playerSignal{typ: signalStart}
		<-p.stopping
		writer.EndOfTrack(wr)
		if p.polyErrCount > 0 {
			return fmt.Errorf("Polyphony limit exceeded by %d note(s).", p.polyErrCount)
		}
		return nil
	})
}

// change UI strings for notes based on keymap
func (s *song) renameNotes() {
	for _, t := range s.Tracks {
		for _, te := range t.Events {
			if te.Type == noteOnEvent || te.Type == pitchBendEvent {
				te.setUiString(s.Keymap)
			}
		}
	}
}

type track struct {
	Channel uint8
	Events  []*trackEvent
	index   int // only used by undo/redo

	// only used by player
	activeNote  uint8
	midiChannel uint8
	pressure    uint8
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
	Tick       int64
	Type       trackEventType
	FloatData  float64 `json:",omitempty"`
	ByteData1  byte    `json:",omitempty"`
	ByteData2  byte    `json:",omitempty"`
	ByteData3  byte    `json:",omitempty"`
	TextData   string  `json:",omitempty"`
	uiString   string
	track      int
	chordIndex uint8 // for chord entry
}

func newTrackEvent(te *trackEvent, k *keymap) *trackEvent {
	te.setUiString(k)
	return te
}

var textEventLabels = []string{
	"",
	"text",
	"copy",
	"title",
	"inst",
	"lyric",
	"marker",
	"cue",
	"prog",
	"device",
}

func formatDrumPitch(b byte) string {
	return standardPitchNames[int(b)%len(standardPitchNames)] +
		fmt.Sprintf("%d", b/12)
}

func (te *trackEvent) setUiString(k *keymap) {
	switch te.Type {
	case noteOnEvent:
		if k != nil && !te.renameNote(k) {
			te.uiString = fmt.Sprintf("on %.2f %d", te.FloatData, te.ByteData1)
		}
	case drumNoteOnEvent:
		te.uiString = fmt.Sprintf(
			"dr %s %d", formatDrumPitch(te.ByteData1), te.ByteData2)
	case noteOffEvent:
		te.uiString = "off"
	case controllerEvent:
		te.uiString = fmt.Sprintf("cc %d %d", te.ByteData1, te.ByteData2)
	case pitchBendEvent:
		if k != nil && !te.renameNote(k) {
			te.uiString = fmt.Sprintf("bend %.2f", te.FloatData)
		}
	case channelPressureEvent:
		te.uiString = fmt.Sprintf("af %d", te.ByteData1)
	case keyPressureEvent:
		te.uiString = fmt.Sprintf("kp %d", te.ByteData1)
	case programEvent:
		te.uiString = fmt.Sprintf("prog %d %d %d",
			te.ByteData1+1, te.ByteData2, te.ByteData3)
	case tempoEvent:
		te.uiString = fmt.Sprintf("tempo %.2f", te.FloatData)
	case textEvent:
		label := "UNKNOWN"
		if te.ByteData1 >= 1 && int(te.ByteData1) < len(textEventLabels) {
			label = textEventLabels[te.ByteData1]
		}
		te.uiString = fmt.Sprintf("%s \"%s\"", label, te.TextData)
	case releaseLenEvent:
		te.uiString = fmt.Sprintf("@rel %.2f", te.FloatData)
	case midiRangeEvent:
		te.uiString = fmt.Sprintf("@chn %d %d", te.ByteData1+1, te.ByteData2+1)
	case midiOutputEvent:
		te.uiString = fmt.Sprintf("@out %d", te.ByteData1)
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

// reset UI string based on keymap, returning true if successful
func (te *trackEvent) renameNote(k *keymap) bool {
	if s := k.notatePitch(te.FloatData, true); s != "" {
		if te.Type == noteOnEvent {
			te.uiString = fmt.Sprintf("%s %d", s, te.ByteData1)
		} else if te.Type == pitchBendEvent {
			te.uiString = fmt.Sprintf("bend %s", s)
		}
		return true
	}
	return false
}
