package main

import (
	"fmt"
	"reflect"
	"strconv"
)

var (
	settingsPath = joinTreePath(configPath, "settings.csv")
)

type settings struct {
	ColorBeat         uint32
	ColorBg1          uint32
	ColorBg2          uint32
	ColorFg           uint32
	ColorPlayPos      uint32
	ColorSelect       uint32
	DefaultKeymap     string
	PercussionKeymap  string
	Font              string
	FontSize          int
	MessageDuration   int
	MidiInPortNumber  int
	MidiOutPortNumber int
	OffDivisionAlpha  int
	ShiftScrollMult   int
	UndoBufferSize    int
	WindowHeight      int
	WindowWidth       int
}

// load settings from config file
func loadSettings(warn func(string)) *settings {
	s := &settings{}
	if records, err := readCSV("config/settings.csv", true); err == nil {
		s.applyRecords(records, warn)
	} else {
		warn(err.Error())
	}
	if records, err := readCSV(settingsPath, false); err == nil {
		s.applyRecords(records, warn)
	} else {
		warn(err.Error())
	}
	return s
}

// apply CSV records
func (s *settings) applyRecords(records [][]string, warn func(string)) {
	v := reflect.ValueOf(s).Elem()
	for _, rec := range records {
		success := false
		if len(rec) == 2 {
			if field := v.FieldByName(rec[0]); field.IsValid() {
				switch field.Kind() {
				case reflect.Uint32:
					if len(rec[1]) > 1 {
						if i, err := strconv.ParseUint(rec[1][1:], 16, 32); err == nil {
							field.SetUint(uint64(i))
							success = true
						}
					}
				case reflect.Int:
					if i, err := strconv.Atoi(rec[1]); err == nil {
						field.SetInt(int64(i))
						success = true
					}
				case reflect.String:
					field.SetString(rec[1])
					success = true
				}
			}
		}
		if !success {
			warn(fmt.Sprintf("bad settings record: %v", rec))
		}
	}
}
