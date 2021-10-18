package main

import (
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/veandco/go-sdl2/sdl"
	"github.com/veandco/go-sdl2/ttf"
	"gitlab.com/gomidi/midi"
	"gitlab.com/gomidi/midi/reader"
	"gitlab.com/gomidi/midi/writer"
	driver "gitlab.com/gomidi/rtmididrv"
)

const (
	windowWidth   = 1280
	windowHeight  = 720
	appName       = "Faunatone"
	fileExt       = ".fna"
	defaultFps    = 60
	bendSemitones = 24
)

var (
	colorBg             = sdl.Color{0xf0, 0xf0, 0xf0, 0xff}
	colorBgArray        = []uint8{0xf0, 0xf0, 0xf0, 0xff}
	colorHighlightArray = []uint8{0xe0, 0xe0, 0xe0, 0xff}
	colorPlayPosArray   = []uint8{0xe8, 0xe8, 0xe8, 0xff}
	colorFg             = sdl.Color{0x10, 0x10, 0x10, 0xff}
	colorFgArray        = []uint8{0x10, 0x10, 0x10, 0xff}

	configPath   = "config"
	settingsPath = filepath.Join("config", "settings.csv")
	fontPath     = filepath.Join("assets", "RobotoMono-Regular-BasicLatin.ttf")

	fontSize = int32(12)
	padding  = fontSize / 2
)

func must(err error) {
	if err != nil {
		panic(err.Error())
	}
}

func main() {
	settings := loadSettings()
	if v, ok := settings["fontSize"]; ok {
		fontSize = int32(v)
		padding = fontSize / 2
	}

	drv, err := driver.New()
	must(err)
	defer drv.Close()

	midiIn := make(chan midi.Message, 100)
	if n, ok := settings["midiInPortNumber"]; ok {
		ins, err := drv.Ins()
		must(err)
		in := ins[n]
		must(in.Open())
		defer in.Close()
		rd := reader.New(reader.NoLogger(),
			reader.Each(func(pos *reader.Position, msg midi.Message) {
				select {
				case midiIn <- msg:
				default:
				}
			}),
		)
		err = rd.ListenTo(in)
		must(err)
	}

	outs, err := drv.Outs()
	must(err)
	out := outs[settings["midiOutPortNumber"]]
	must(out.Open())
	defer out.Close()
	wr := writer.New(out)

	err = sdl.Init(sdl.INIT_VIDEO | sdl.INIT_EVENTS)
	must(err)
	defer sdl.Quit()
	window, err := sdl.CreateWindow(appName, sdl.WINDOWPOS_UNDEFINED, sdl.WINDOWPOS_UNDEFINED,
		windowWidth, windowHeight, sdl.WINDOW_SHOWN|sdl.WINDOW_RESIZABLE|sdl.WINDOW_ALLOW_HIGHDPI)
	must(err)
	defer window.Destroy()
	renderer, err := sdl.CreateRenderer(window, -1,
		sdl.RENDERER_ACCELERATED|sdl.RENDERER_PRESENTVSYNC)
	must(err)
	defer renderer.Destroy()

	err = ttf.Init()
	must(err)
	defer ttf.Quit()
	font, err := ttf.OpenFont(fontPath, int(fontSize))
	must(err)
	defer font.Close()
	pr, err := newPrinter(font)
	must(err)
	defer pr.destroy()

	redraw := false
	redrawChan := make(chan bool)
	go func() {
		for v := range redrawChan {
			redraw = v
		}
	}()
	fps := getRefreshRate()

	sng := &song{
		Tracks: []*track{
			&track{},
			&track{},
			&track{},
			&track{},
		},
	}
	patedit := &patternEditor{
		printer:      pr,
		song:         sng,
		division:     defaultDivision,
		velocity:     defaultVelocity,
		controller:   defaultController,
		refPitch:     defaultRefPitch,
		historyIndex: -1,
	}
	pl := newPlayer(sng, wr, true)
	pl.redrawChan = redrawChan
	go pl.run()
	defer pl.cleanup()
	km, _ := newKeymap(defaultKeymapPath)
	dia := &dialog{}

	// required for cursor blink
	go func() {
		for {
			if dia.shown && dia.size > 0 {
				redrawChan <- true
				time.Sleep(time.Millisecond * inputCursorBlinkMs)
			}
		}
	}()

	running := true

	mb := &menuBar{
		menus: []*menu{
			{
				label: "File",
				items: []*menuItem{
					{label: "Open...", action: func() { dialogOpen(dia, sng, patedit) }},
					{label: "Save as...", action: func() { dialogSaveAs(dia, sng) }},
					{label: "Export MIDI...", action: func() { dialogExportMIDI(dia, sng, pl) }},
					{label: "Quit", action: func() { running = false }},
				},
			},
			{
				label: "Play",
				items: []*menuItem{
					{label: "From start", action: func() {
						pl.signal <- playerSignal{typ: signalStart}
					}},
					{label: "From top of screen", action: func() {
						pl.signal <- playerSignal{typ: signalStart,
							tick: patedit.firstTickOnScreen()}
					}},
					{label: "From cursor", action: func() {
						_, _, minTick, _ := patedit.getSelection()
						pl.signal <- playerSignal{typ: signalStart, tick: minTick}
					}},
					{label: "Stop", action: func() {
						pl.signal <- playerSignal{typ: signalStop}
					}},
				},
			},
			{
				label: "Cursor",
				items: []*menuItem{
					{label: "Previous division", action: func() { patedit.moveCursor(0, -1) },
						repeat: true},
					{label: "Next division", action: func() { patedit.moveCursor(0, 1) },
						repeat: true},
					{label: "Previous track", action: func() { patedit.moveCursor(-1, 0) },
						repeat: true},
					{label: "Next track", action: func() { patedit.moveCursor(1, 0) },
						repeat: true},
					{label: "Go to beat...", action: func() { dialogGoToBeat(dia, patedit) }},
				},
			},
			{
				label: "Insert",
				items: []*menuItem{
					{label: "Note...", action: func() {
						dialogInsertNote(dia, patedit, pl)
					}},
					{label: "Drum note...", action: func() {
						dialogInsertDrumNote(dia, patedit, pl)
					}},
					{label: "Note off", action: func() {
						patedit.writeEvent(newTrackEvent(&trackEvent{Type: noteOffEvent}), pl)
					}},
					{label: "Pitch bend...", action: func() {
						dialogInsertPitchBend(dia, km, patedit, pl)
					}},
					{label: "Program change...", action: func() {
						dialogInsertProgramChange(dia, patedit, pl)
					}},
					{label: "Tempo change...", action: func() {
						dialogInsertTempoChange(dia, patedit, pl)
					}},
					{label: "Control change...", action: func() {
						dialogInsertControlChange(dia, patedit, pl)
					}},
				},
			},
			{
				label: "Edit",
				items: []*menuItem{
					{label: "Delete events", action: func() {
						patedit.deleteSelectedEvents()
					}},
					{label: "Undo", action: func() { dialogIfErr(dia, patedit.undo()) },
						repeat: true},
					{label: "Redo", action: func() { dialogIfErr(dia, patedit.redo()) },
						repeat: true},
					{label: "Cut", action: func() { patedit.cut() }},
					{label: "Copy", action: func() { patedit.copy() }},
					{label: "Paste", action: func() { patedit.paste(false) }},
					{label: "Mix paste", action: func() { patedit.paste(true) }},
					{label: "Transpose...", action: func() { dialogTranpose(dia, patedit, km) }},
					{label: "Interpolate", action: func() { patedit.interpolateSelection() }},
				},
			},
			{
				label: "Status",
				items: []*menuItem{
					{label: "Decrease octave", action: func() { patedit.modifyRefPitch(-12) },
						repeat: true},
					{label: "Increase octave", action: func() { patedit.modifyRefPitch(12) },
						repeat: true},
					{label: "Capture root pitch", action: func() { patedit.captureRefPitch() }},
					{label: "Set velocity...", action: func() { dialogSetVelocity(dia, patedit) }},
					{label: "Set controller...", action: func() {
						dialogSetController(dia, patedit)
					}},
					{label: "Decrease division", action: func() { patedit.addDivision(-1) },
						repeat: true},
					{label: "Increase division", action: func() { patedit.addDivision(1) },
						repeat: true},
					{label: "Halve division", action: func() { patedit.multiplyDivision(0.5) }},
					{label: "Double division", action: func() { patedit.multiplyDivision(2) }},
					{label: "Remap key...", action: func() { dialogRemapKey(dia, km) }},
					{label: "Load keymap...", action: func() { dialogLoadKeymap(dia, km) }},
					{label: "Make isomorphic keymap...", action: func() {
						dialogMakeIsoKeymap(dia, km)
					}},
					{label: "Toggle song follow", action: func() {
						patedit.followSong = !patedit.followSong
					}},
				},
			},
			{
				label: "Track",
				items: []*menuItem{
					{label: "Set channel...", action: func() {
						dialogTrackSetChannel(dia, sng, patedit)
					}},
					{label: "Insert", action: func() { patedit.insertTrack() }},
					{label: "Delete", action: func() { patedit.deleteTrack() }},
					{label: "Move left", action: func() { patedit.shiftTracks(-1) },
						repeat: true},
					{label: "Move right", action: func() { patedit.shiftTracks(1) },
						repeat: true},
				},
			},
		},
	}
	mb.init(pr)

	sb := newStatusBar(
		func() string { return fmt.Sprintf("Root: %.2f", patedit.refPitch) },
		func() string { return fmt.Sprintf("Division: %d", patedit.division) },
		func() string { return fmt.Sprintf("Velocity: %d", patedit.velocity) },
		func() string { return fmt.Sprintf("Controller: %d", patedit.controller) },
		func() string { return fmt.Sprintf("Keymap: %s", km.name) },
		func() string {
			if patedit.followSong {
				return "Follow"
			}
			return ""
		},
	)

	// attempt to load save file specified by first CLI arg
	if len(os.Args) > 1 {
		if f, err := os.Open(os.Args[1]); err == nil {
			if err := sng.read(f); err == nil {
				sb.showMessage(fmt.Sprintf("Loaded %s.", os.Args[1]), redrawChan)
			} else {
				sb.showMessage(err.Error(), redrawChan)
			}
			f.Close()
		} else {
			sb.showMessage(err.Error(), redrawChan)
		}
	}

	for running {
		// process SDL events
		for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
			// if we got any event, assume redraw is needed
			redrawChan <- true

			switch event := event.(type) {
			case *sdl.MouseMotionEvent:
				if !dia.shown {
					mb.mouseMotion(event)
					patedit.mouseMotion(event)
				}
			case *sdl.MouseButtonEvent:
				if dia.shown {
					if event.State == sdl.PRESSED {
						dia.shown = false
					}
				} else {
					if !mb.shown() {
						patedit.mouseButton(event)
					}
					mb.mouseButton(event)
				}
			case *sdl.KeyboardEvent:
				if dia.shown {
					dia.keyboardEvent(event)
				} else if !mb.keyboardEvent(event) {
					km.keyboardEvent(event, patedit, pl)
				}
			case *sdl.TextInputEvent:
				if dia.shown {
					dia.textInput(event)
				}
			case *sdl.MouseWheelEvent:
				if !dia.shown {
					patedit.mouseWheel(event)
				}
			case *sdl.QuitEvent:
				running = false
				break
			}
		}

		// process MIDI events
	outer:
		for {
			select {
			case msg := <-midiIn:
				if !dia.shown {
					switch msg.Raw()[0] & 0xf0 {
					case 0x80, 0x90: // note off, note on
						km.midiEvent(msg.Raw(), patedit, pl)
					}
				}
			default:
				break outer
			}
		}

		// hack to prevent Alt+<letter> from typing <letter> into dialog
		dia.accept = dia.shown

		if redraw {
			redrawChan <- false
			renderer.SetDrawColorArray(colorBgArray...)
			renderer.Clear()
			renderer.SetDrawColorArray(colorFgArray...)
			viewport := renderer.GetViewport()
			y := mb.menus[0].rect.H
			patedit.draw(renderer, &sdl.Rect{0, y, viewport.W, viewport.H - y - sb.rect.H},
				pl.lastTick)
			sb.draw(pr, renderer)
			mb.draw(pr, renderer)
			dia.draw(pr, renderer)
			renderer.Present()
		}
		sdl.Delay(uint32(1000 / fps))
	}
}

// set d to an input dialog
func dialogGoToBeat(d *dialog, pe *patternEditor) {
	*d = dialog{
		prompt: "Go to beat:",
		size:   5,
		action: func(s string) {
			if f, err := strconv.ParseFloat(s, 64); err == nil {
				pe.goToBeat(f)
			} else {
				dialogMsg(d, err.Error())
			}
		},
		shown: true,
	}
}

// set d to a message dialog
func dialogMsg(d *dialog, s string) {
	*d = dialog{
		prompt: s,
		size:   0,
		shown:  true,
	}
}

// set d to a message dialog if err is non-nil
func dialogIfErr(d *dialog, err error) {
	if err != nil {
		dialogMsg(d, err.Error())
	}
}

// set d to an input dialog
func dialogInsertNote(d *dialog, pe *patternEditor, p *player) {
	*d = dialog{
		prompt: "Insert note:",
		size:   7,
		action: func(s string) {
			if f, err := strconv.ParseFloat(s, 64); err == nil {
				if f >= minPitch && f <= maxPitch {
					pe.writeEvent(newTrackEvent(&trackEvent{
						Type:      noteOnEvent,
						FloatData: f,
						ByteData1: pe.velocity,
					}), p)
				} else {
					dialogMsg(d, fmt.Sprintf("Note must be in the range [%d, %d].",
						minPitch, maxPitch))
				}
			} else {
				dialogMsg(d, err.Error())
			}
		},
		shown: true,
	}
}

// return note and pitch wheel values required to play a pitch in MIDI,
// assuming a 2-semitone pitch bend range
func pitchToMIDI(p float64) (uint8, int16) {
	note := uint8(math.Round(math.Max(0, math.Min(127, p))))
	bend := int16((p - float64(note)) * 8192.0 / bendSemitones)
	return note, bend
}

// set to d an input dialog
func dialogInsertDrumNote(d *dialog, pe *patternEditor, p *player) {
	*d = *newDialog("Insert drum note:", 3, func(s string) {
		if i, err := strconv.ParseUint(s, 10, 8); err == nil {
			if i < 128 {
				pe.writeEvent(newTrackEvent(&trackEvent{
					Type:      drumNoteOnEvent,
					ByteData1: uint8(i),
					ByteData2: pe.velocity,
				}), p)
			} else {
				dialogMsg(d, "Note must be in the range [0, 127].")
			}
		} else {
			dialogMsg(d, err.Error())
		}
	})
}

// set d to a key dialog
func dialogInsertPitchBend(d *dialog, k *keymap, pe *patternEditor, p *player) {
	*d = *newDialog("Insert pitch bend...", 0, func(s string) {
		if f, ok := k.pitchFromString(s, pe.refPitch); ok {
			pe.writeEvent(newTrackEvent(&trackEvent{
				Type:      pitchBendEvent,
				FloatData: f,
			}), p)
		} else {
			dialogMsg(d, "Key not in keymap.")
		}
	})
	d.keymode = true
}

// set d to an input dialog
func dialogInsertProgramChange(d *dialog, pe *patternEditor, p *player) {
	*d = *newDialog("Insert program change:", 3, func(s string) {
		if i, err := strconv.ParseUint(s, 10, 8); err == nil {
			if i >= 1 && i <= 128 {
				pe.writeEvent(newTrackEvent(&trackEvent{
					Type:      programEvent,
					ByteData1: byte(i - 1),
				}), p)
			} else {
				dialogMsg(d, "Program must be in the range [1, 128].")
			}
		} else {
			dialogMsg(d, err.Error())
		}
	})
}

// set d to an input dialog
func dialogInsertTempoChange(d *dialog, pe *patternEditor, p *player) {
	*d = *newDialog("Insert tempo change:", 7, func(s string) {
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			if f > 0 {
				pe.writeEvent(newTrackEvent(&trackEvent{
					Type:      tempoEvent,
					FloatData: f,
				}), p)
			} else {
				dialogMsg(d, "Tempo must be above zero.")
			}
		} else {
			dialogMsg(d, err.Error())
		}
	})
}

// set d to an input dialog
func dialogInsertControlChange(d *dialog, pe *patternEditor, p *player) {
	*d = *newDialog("Controller value:", 3, func(s string) {
		if i, err := strconv.ParseUint(s, 10, 8); err == nil {
			pe.writeEvent(newTrackEvent(&trackEvent{
				Type:      controllerEvent,
				ByteData1: pe.controller,
				ByteData2: byte(i),
			}), p)
		} else {
			dialogMsg(d, err.Error())
		}
	})
}

// set d to an input dialog
func dialogSetController(d *dialog, pe *patternEditor) {
	*d = *newDialog("Controller index:", 3, func(s string) {
		if i, err := strconv.ParseUint(s, 10, 8); err == nil {
			if i < 128 {
				pe.controller = uint8(i)
			} else {
				dialogMsg(d, "Controller must be in the range [0, 127].")
			}
		} else {
			dialogMsg(d, err.Error())
		}
	})
}

// set d to an input dialog
func dialogTranpose(d *dialog, pe *patternEditor, k *keymap) {
	*d = *newDialog("Transpose selection by:", 7, func(s string) {
		if f, err := parsePitch(s, k); err == nil {
			pe.transposeSelection(f)
		} else {
			dialogMsg(d, err.Error())
		}
	})
}

// set d to an input dialog
func dialogSetVelocity(d *dialog, pe *patternEditor) {
	*d = *newDialog("Set velocity:", 3, func(s string) {
		if i, err := strconv.ParseUint(s, 10, 8); err == nil {
			if i < 128 {
				pe.velocity = uint8(i)
			} else {
				dialogMsg(d, "Velocity must be in the range [0, 127].")
			}
		} else {
			dialogMsg(d, err.Error())
		}
	})
}

// set d to a key dialog, then input dialog
func dialogRemapKey(d *dialog, k *keymap) {
	*d = *newDialog("Press key to remap...", 0, func(s1 string) {
		*d = *newDialog("Remap to interval:", 7, func(s2 string) {
			if f, err := parsePitch(s2, k); err == nil {
				k.keymap[s1] = f
				k.name = addSuffixIfMissing(k.name, "*")
			} else {
				dialogMsg(d, err.Error())
			}
		})
	})
	d.keymode = true
}

// set d to an input dialog
func dialogLoadKeymap(d *dialog, k *keymap) {
	*d = *newDialog("Load keymap:", 50, func(s string) {
		s = addSuffixIfMissing(s, ".csv")
		if k2, err := newKeymap(s); err == nil {
			*k = *k2
		} else {
			dialogMsg(d, err.Error())
		}
	})
}

// set d to an input dialog chain
func dialogMakeIsoKeymap(d *dialog, k *keymap) {
	*d = *newDialog("Enter first interval:", 7, func(s string) {
		if f1, err := parsePitch(s, k); err == nil {
			*d = *newDialog("Enter second interval:", 7, func(s string) {
				if f2, err := parsePitch(s, k); err == nil {
					*k = *genIsoKeymap(f1, f2)
				} else {
					dialogMsg(d, err.Error())
				}
			})
		} else {
			dialogMsg(d, err.Error())
		}
	})
}

// set d to an input dialog
func dialogOpen(d *dialog, sng *song, pe *patternEditor) {
	*d = *newDialog("Open:", 50, func(s string) {
		s = addSuffixIfMissing(s, fileExt)
		if f, err := os.Open(s); err == nil {
			defer f.Close()
			if err := sng.read(f); err == nil {
				pe.reset()
			} else {
				dialogMsg(d, err.Error())
			}
		} else {
			dialogMsg(d, err.Error())
		}
	})
}

// set d to an input dialog
func dialogSaveAs(d *dialog, sng *song) {
	*d = *newDialog("Save as:", 50, func(s string) {
		s = addSuffixIfMissing(s, fileExt)
		if f, err := os.Create(s); err == nil {
			defer f.Close()
			if err := sng.write(f); err != nil {
				dialogMsg(d, err.Error())
			}
		} else {
			dialogMsg(d, err.Error())
		}
	})
}

// set d to an input dialog
func dialogExportMIDI(d *dialog, sng *song, p *player) {
	*d = *newDialog("Export as:", 50, func(s string) {
		s = addSuffixIfMissing(s, ".mid")
		p.signal <- playerSignal{typ: signalStop} // avoid race conditions
		if err := sng.exportSMF(s); err != nil {
			dialogMsg(d, err.Error())
		}
	})
}

// set d to an input dialog
func dialogTrackSetChannel(d *dialog, sng *song, pe *patternEditor) {
	*d = *newDialog("Set channel:", 3, func(s string) {
		if i, err := strconv.ParseUint(s, 10, 8); err == nil {
			if i >= 1 && i <= numVirtualChannels {
				pe.setTrackChannel(uint8(i - 1))
			} else {
				dialogMsg(d, fmt.Sprintf("Channel must be in the range [1, %d].",
					numVirtualChannels))
			}
		} else {
			dialogMsg(d, err.Error())
		}
	})
}

// read records from a CSV file
func readCSV(path string) ([][]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.TrimLeadingSpace = true
	r.Comment = '#'
	return r.ReadAll()
}

// return base+suffix if base does not already end with suffix, otherwise
// return base. NOT case-sensitive.
func addSuffixIfMissing(base, suffix string) string {
	if !strings.HasSuffix(strings.ToLower(base), strings.ToLower(suffix)) {
		return base + suffix
	}
	return base
}

// return the refresh rate of the display, according to SDL, or default FPS if
// it's not available
func getRefreshRate() int {
	if dm, err := sdl.GetCurrentDisplayMode(0); err == nil {
		return int(dm.RefreshRate)
	}
	return defaultFps
}

// load settings from config/settings.csv
func loadSettings() map[string]int {
	m := make(map[string]int)
	if records, err := readCSV(settingsPath); err == nil {
		for _, rec := range records {
			if len(rec) == 2 {
				if i, err := strconv.Atoi(rec[1]); err == nil {
					m[rec[0]] = i
				}
			}
		}
	}
	return m
}
