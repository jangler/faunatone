package main

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strconv"
)

var (
	settingsPath = filepath.Join(configPath, "settings.csv")
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
	UndoBufferSize    int
	WindowHeight      int
	WindowWidth       int
}

// load settings from config file
func loadSettings(warn func(string)) *settings {
	s := &settings{}
	if records, err := readCSV(settingsPath); err == nil {
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
	return s
}
