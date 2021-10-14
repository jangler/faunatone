package main

import (
	"fmt"
)

type trackEventType byte

const (
	noteOffEvent trackEventType = iota
	noteOnEvent
	controllerEvent
	programEvent
	tempoEvent
)

type song struct {
	title  string
	tracks []*track
}

type track struct {
	channelMask uint16
	events      []*trackEvent
}

type trackEvent struct {
	tick      int64
	typ       trackEventType
	floatData float64
	byteData1 byte
	byteData2 byte
	uiString  string
}

func newTrackEvent(te *trackEvent) *trackEvent {
	te.setUiString()
	return te
}

func (te *trackEvent) setUiString() {
	switch te.typ {
	case noteOnEvent:
		te.uiString = fmt.Sprintf("on %.1f %d", te.floatData, te.byteData1)
	case noteOffEvent:
		te.uiString = "off"
	case controllerEvent:
		te.uiString = fmt.Sprintf("cc %d %d", te.byteData1, te.byteData2)
	case programEvent:
		te.uiString = fmt.Sprintf("pc %d", te.byteData1)
	case tempoEvent:
		te.uiString = fmt.Sprintf("tp %.1f", te.floatData)
	default:
		te.uiString = "UNKNOWN"
	}
}
