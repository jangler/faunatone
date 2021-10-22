package main

import (
	"encoding/csv"
	"fmt"
	"io"
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
	appName       = "Faunatone"
	fileExt       = ".faun"
	defaultFps    = 60
	bendSemitones = 24
	configPath    = "config"
	assetsPath    = "assets"
	savesPath     = "saves"
	exportsPath   = "exports"
)

var (
	colorBeatArray    = make([]uint8, 4)
	colorBg1Array     = make([]uint8, 4)
	colorBg2Array     = make([]uint8, 4)
	colorFgArray      = make([]uint8, 4)
	colorFg           = sdl.Color{}
	colorPlayPosArray = make([]uint8, 4)
	colorSelectArray  = make([]uint8, 4)
	padding           = int32(0)

	saveAutofill   string
	exportAutofill string
)

func must(err error) {
	if err != nil {
		panic(err.Error())
	}
}

func main() {
	settings := loadSettings(func(s string) { println(s) })
	setColorArray(colorBeatArray, settings.ColorBeat)
	setColorArray(colorBg1Array, settings.ColorBg1)
	setColorArray(colorBg2Array, settings.ColorBg2)
	setColorArray(colorFgArray, settings.ColorFg)
	setColorSDL(&colorFg, settings.ColorFg)
	setColorArray(colorPlayPosArray, settings.ColorPlayPos)
	setColorArray(colorSelectArray, settings.ColorSelect)
	padding = int32(settings.FontSize) / 2

	dia := &dialog{}

	drv, err := driver.New()
	must(err)
	defer drv.Close()

	midiIn := make(chan midi.Message, 100)
	if n := settings.MidiInPortNumber; n >= 0 {
		ins, err := drv.Ins()
		must(err)
		if n < len(ins) {
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
		} else {
			dialogMsg(dia, fmt.Sprintf("MIDI input port index %d out of range [%d, %d].",
				n, 0, len(ins)))
		}
	}

	var wr *writer.Writer
	if n := settings.MidiOutPortNumber; n >= 0 {
		outs, err := drv.Outs()
		must(err)
		if settings.MidiOutPortNumber < len(outs) {
			out := outs[settings.MidiOutPortNumber]
			must(out.Open())
			defer out.Close()
			wr = writer.New(out)
			sendGMSystemOn(wr)
		} else {
			dialogMsg(dia, fmt.Sprintf("MIDI output port index %d out of range [%d, %d].",
				n, 0, len(outs)))
		}
	}
	if wr == nil {
		wr = writer.New(io.Discard) // dummy output
	}

	err = sdl.Init(sdl.INIT_VIDEO | sdl.INIT_EVENTS)
	must(err)
	defer sdl.Quit()
	window, err := sdl.CreateWindow(appName, sdl.WINDOWPOS_UNDEFINED, sdl.WINDOWPOS_UNDEFINED,
		int32(settings.WindowWidth), int32(settings.WindowHeight),
		sdl.WINDOW_SHOWN|sdl.WINDOW_RESIZABLE|sdl.WINDOW_ALLOW_HIGHDPI)
	must(err)
	defer window.Destroy()
	renderer, err := sdl.CreateRenderer(window, -1,
		sdl.RENDERER_ACCELERATED|sdl.RENDERER_PRESENTVSYNC)
	must(err)
	defer renderer.Destroy()

	err = ttf.Init()
	must(err)
	defer ttf.Quit()
	fontPath := filepath.Join(assetsPath, settings.Font)
	font, err := ttf.OpenFont(fontPath, settings.FontSize)
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

	sng := newSong()
	patedit := &patternEditor{
		printer:          pr,
		song:             sng,
		division:         defaultDivision,
		velocity:         defaultVelocity,
		controller:       defaultController,
		refPitch:         defaultRefPitch,
		historyIndex:     -1,
		historySizeLimit: settings.UndoBufferSize,
	}
	pl := newPlayer(sng, wr, true)
	pl.redrawChan = redrawChan
	go pl.run()
	defer pl.cleanup()
	sng.Keymap, _ = newKeymap(settings.DefaultKeymap)
	percKeymap, _ := newKeymap(settings.PercussionKeymap)
	percKeymap.isPerc = true

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
	keyjazz := false

	mb := &menuBar{
		menus: []*menu{
			{
				label: "File",
				items: []*menuItem{
					{label: "New", action: func() { dialogNew(dia, sng, patedit, pl) }},
					{label: "Open...", action: func() { dialogOpen(dia, sng, patedit, pl) }},
					{label: "Save as...", action: func() { dialogSaveAs(dia, sng) }},
					{label: "Export MIDI...", action: func() { dialogExportMIDI(dia, sng, pl) }},
					{label: "Load keymap...", action: func() { dialogLoadKeymap(dia, sng) }},
					{label: "Save keymap as...", action: func() { dialogSaveKeymap(dia, sng) }},
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
					{label: "Stop", action: func() { pl.stop(false) }},
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
						patedit.writeEvent(newTrackEvent(&trackEvent{Type: noteOffEvent}, nil), pl)
					}},
					{label: "Pitch bend...", action: func() {
						dialogInsertPitchBend(dia, patedit, pl)
					}},
					{label: "Program change...", action: func() {
						dialogInsertUint8Event(dia, patedit, pl,
							"Insert program change:", programEvent, 1)
					}},
					{label: "Tempo change...", action: func() {
						dialogInsertTempoChange(dia, patedit, pl)
					}},
					{label: "Control change...", action: func() {
						dialogInsertControlChange(dia, patedit, pl)
					}},
					{label: "Aftertouch...", action: func() {
						dialogInsertUint8Event(dia, patedit, pl,
							"Insert channel pressure:", channelPressureEvent, 0)
					}},
					/* this is not part of GM level 1
					{label: "Polyphonic aftertouch...", action: func() {
						dialogInsertUint8Event(dia, patedit, pl,
							"Insert key pressure:", keyPressureEvent, 0)
					}},
					*/
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
					{label: "Insert division", action: func() { patedit.insertDivision() }},
					{label: "Delete division", action: func() { patedit.deleteDivision() }},
					{label: "Transpose...", action: func() { dialogTranpose(dia, patedit) }},
					{label: "Interpolate", action: func() { patedit.interpolateSelection() }},
					{label: "Multiply...", action: func() { dialogMultiply(dia, patedit) }},
					{label: "Vary...", action: func() { dialogVary(dia, patedit) }},
				},
			},
			{
				label: "Status",
				items: []*menuItem{
					{label: "Toggle keyjazz", action: func() { keyjazz = !keyjazz }},
					{label: "Decrease octave", action: func() { patedit.modifyRefPitch(-12) },
						repeat: true},
					{label: "Increase octave", action: func() { patedit.modifyRefPitch(12) },
						repeat: true},
					{label: "Capture root pitch", action: func() { patedit.captureRefPitch() }},
					{label: "Set velocity...", action: func() { dialogSetVelocity(dia, patedit) }},
					{label: "Set controller...", action: func() {
						dialogSetController(dia, patedit)
					}},
					{label: "Set division...", action: func() { dialogSetDivision(dia, patedit) }},
					{label: "Decrease division", action: func() { patedit.addDivision(-1) },
						repeat: true},
					{label: "Increase division", action: func() { patedit.addDivision(1) },
						repeat: true},
					{label: "Halve division", action: func() { patedit.multiplyDivision(0.5) }},
					{label: "Double division", action: func() { patedit.multiplyDivision(2) }},
					{label: "Remap key...", action: func() { dialogRemapKey(dia, sng) }},
					{label: "Make isomorphic keymap...", action: func() {
						dialogMakeIsoKeymap(dia, sng)
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

	sb := newStatusBar(settings.MessageDuration,
		func() string { return fmt.Sprintf("Root: %.2f", patedit.refPitch) },
		func() string { return fmt.Sprintf("Division: %d", patedit.division) },
		func() string { return fmt.Sprintf("Velocity: %d", patedit.velocity) },
		func() string { return fmt.Sprintf("Controller: %d", patedit.controller) },
		func() string { return fmt.Sprintf("Keymap: %s", sng.Keymap.Name) },
		func() string { return conditionalString(patedit.followSong, "Follow", "") },
		func() string { return conditionalString(keyjazz, "Keyjazz", "") },
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
					sng.Keymap.keyboardEvent(event, patedit, pl, keyjazz)
					percKeymap.keyboardEvent(event, patedit, pl, keyjazz)
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
						sng.Keymap.midiEvent(msg.Raw(), patedit, pl, keyjazz)
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
			renderer.SetDrawColorArray(colorBg1Array...)
			renderer.Clear()
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

// return a if cond, else b
func conditionalString(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

// set d to an input dialog
func dialogGoToBeat(d *dialog, pe *patternEditor) {
	*d = *newDialog("Go to beat:", 5, func(s string) {
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			pe.goToBeat(f)
		} else {
			dialogMsg(d, err.Error())
		}
	})
}

// set d to a message dialog
func dialogMsg(d *dialog, s string) {
	*d = *newDialog(s, 0, nil)
}

// set d to a message dialog if err is non-nil
func dialogIfErr(d *dialog, err error) {
	if err != nil {
		dialogMsg(d, err.Error())
	}
}

// set d to an input dialog
func dialogInsertNote(d *dialog, pe *patternEditor, p *player) {
	*d = *newDialog("Insert note:", 7, func(s string) {
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			if f >= minPitch && f <= maxPitch {
				pe.writeEvent(newTrackEvent(&trackEvent{
					Type:      noteOnEvent,
					FloatData: f,
					ByteData1: pe.velocity,
				}, pe.song.Keymap), p)
			} else {
				dialogMsg(d, fmt.Sprintf("Note must be in the range [%d, %d].",
					minPitch, maxPitch))
			}
		} else {
			dialogMsg(d, err.Error())
		}
	})
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
				}, nil), p)
			} else {
				dialogMsg(d, "Note must be in the range [0, 127].")
			}
		} else {
			dialogMsg(d, err.Error())
		}
	})
}

// set d to a key dialog
func dialogInsertPitchBend(d *dialog, pe *patternEditor, p *player) {
	*d = *newDialog("Insert pitch bend...", 0, func(s string) {
		if f, ok := pe.song.Keymap.pitchFromString(s, pe.refPitch); ok {
			pe.writeEvent(newTrackEvent(&trackEvent{
				Type:      pitchBendEvent,
				FloatData: f,
			}, pe.song.Keymap), p)
		} else {
			dialogMsg(d, "Key not in keymap.")
		}
	})
	d.mode = noteInput
}

// set d to an input dialog
func dialogInsertTempoChange(d *dialog, pe *patternEditor, p *player) {
	*d = *newDialog("Insert tempo change:", 7, func(s string) {
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			if f > 0 {
				pe.writeEvent(newTrackEvent(&trackEvent{
					Type:      tempoEvent,
					FloatData: f,
				}, nil), p)
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
			}, nil), p)
		} else {
			dialogMsg(d, err.Error())
		}
	})
}

// set d to an input dialog
func dialogInsertUint8Event(d *dialog, pe *patternEditor, p *player, prompt string,
	et trackEventType, offset int) {
	*d = *newDialog(prompt, 3, func(s string) {
		if i, err := strconv.ParseUint(s, 10, 8); err == nil {
			if int(i) >= offset && int(i) <= 127+offset {
				pe.writeEvent(newTrackEvent(&trackEvent{
					Type:      et,
					ByteData1: byte(int(i) - offset),
				}, nil), p)
			} else {
				dialogMsg(d, fmt.Sprintf("Value must be in the range [%d, %d].",
					offset, 127+offset))
			}
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
func dialogSetDivision(d *dialog, pe *patternEditor) {
	*d = *newDialog("Division:", 3, func(s string) {
		if i, err := strconv.ParseUint(s, 10, 16); err == nil {
			if i > ticksPerBeat {
				i = ticksPerBeat
			} else if i < 1 {
				i = 1
			}
			pe.division = int(i)
		} else {
			dialogMsg(d, err.Error())
		}
	})
}

// set d to an input dialog
func dialogTranpose(d *dialog, pe *patternEditor) {
	*d = *newDialog("Transpose selection by...", 0, func(s string) {
		if f, ok := pe.song.Keymap.pitchFromString(s, 0); ok {
			pe.transposeSelection(f)
		} else {
			dialogMsg(d, "Key not in keymap.")
		}
	})
	d.mode = noteInput
}

// set d to an input dialog
func dialogMultiply(d *dialog, pe *patternEditor) {
	*d = *newDialog("Multiply selection by:", 5, func(s string) {
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			pe.multiplySelection(f)
		} else {
			dialogMsg(d, err.Error())
		}
	})
}

// set d to an input dialog
func dialogVary(d *dialog, pe *patternEditor) {
	*d = *newDialog("Vary selection by:", 5, func(s string) {
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			pe.varySelection(f)
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
func dialogRemapKey(d *dialog, s *song) {
	*d = *newDialog("Press key to remap...", 0, func(s1 string) {
		*d = *newDialog("Remap to interval:", 7, func(s2 string) {
			if f, err := parsePitch(s2, s.Keymap); err == nil {
				ki := newKeyInfo(s1, strings.HasPrefix(s2, "*"), f, "", s2)
				s.Keymap.Items = append(s.Keymap.Items, ki)
				s.Keymap.Name = addSuffixIfMissing(s.Keymap.Name, "*")
			} else {
				dialogMsg(d, err.Error())
			}
		})
	})
	d.mode = noteInput
}

// set d to an input dialog
func dialogLoadKeymap(d *dialog, sng *song) {
	*d = *newDialog("Load keymap:", 50, func(s string) {
		s = addSuffixIfMissing(s, ".csv")
		if k, err := newKeymap(s); err == nil {
			sng.Keymap = k
			sng.renameNotes()
		} else {
			dialogMsg(d, err.Error())
		}
	})
}

// set d to an input dialog
func dialogSaveKeymap(d *dialog, sng *song) {
	*d = *newDialog("Save keymap as:", 50, func(s string) {
		s = addSuffixIfMissing(s, ".csv")
		if err := sng.Keymap.write(s); err != nil {
			dialogMsg(d, err.Error())
		}
	})
	d.input = addSuffixIfMissing(sng.Keymap.Name, ".csv")
}

// set d to an input dialog chain
func dialogMakeIsoKeymap(d *dialog, sng *song) {
	*d = *newDialog("Enter first interval:", 7, func(s string) {
		if f1, err := parsePitch(s, sng.Keymap); err == nil {
			*d = *newDialog("Enter second interval:", 7, func(s string) {
				if f2, err := parsePitch(s, sng.Keymap); err == nil {
					sng.Keymap = genIsoKeymap(f1, f2)
					sng.renameNotes()
				} else {
					dialogMsg(d, err.Error())
				}
			})
		} else {
			dialogMsg(d, err.Error())
		}
	})
}

// set d to a y/n dialog
func dialogNew(d *dialog, sng *song, pe *patternEditor, p *player) {
	*d = *newDialog("Create new song? (y/n)", 0, func(s string) {
		p.stop(true)
		*sng = *newSong()
		pe.reset()
		saveAutofill = ""
		exportAutofill = ""
	})
	d.mode = yesNoInput
}

// set d to an input dialog
func dialogOpen(d *dialog, sng *song, pe *patternEditor, p *player) {
	*d = *newDialog("Open:", 50, func(s string) {
		s = addSuffixIfMissing(s, fileExt)
		if f, err := os.Open(filepath.Join(savesPath, s)); err == nil {
			defer f.Close()
			p.stop(true)
			if err := sng.read(f); err == nil {
				pe.reset()
				saveAutofill = s
				exportAutofill = replaceSuffix(s, fileExt, ".mid")
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
		saveAutofill = s
		if exportAutofill == "" {
			exportAutofill = replaceSuffix(s, fileExt, ".mid")
		}
		os.MkdirAll(savesPath, 0755)
		if f, err := os.Create(filepath.Join(savesPath, s)); err == nil {
			defer f.Close()
			if err := sng.write(f); err != nil {
				dialogMsg(d, err.Error())
			}
		} else {
			dialogMsg(d, err.Error())
		}
	})
	d.input = saveAutofill
}

// set d to an input dialog
func dialogExportMIDI(d *dialog, sng *song, p *player) {
	*d = *newDialog("Export as:", 50, func(s string) {
		s = addSuffixIfMissing(s, ".mid")
		exportAutofill = s
		if saveAutofill == "" {
			saveAutofill = replaceSuffix(s, ".mid", fileExt)
		}
		p.stop(true) // avoid race condition
		os.MkdirAll(exportsPath, 0755)
		if err := sng.exportSMF(filepath.Join(exportsPath, s)); err != nil {
			dialogMsg(d, err.Error())
		}
	})
	d.input = exportAutofill
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

// write records to a CSV file
func writeCSV(path string, records [][]string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return csv.NewWriter(f).WriteAll(records)
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

// set an array to the bytes of an int, MSB to LSB
func setColorArray(a []uint8, v uint32) {
	for i := range a {
		a[i] = uint8(v >> ((len(a) - i - 1) * 8))
	}
}

// same idea as setColorArray
func setColorSDL(c *sdl.Color, v uint32) {
	a := make([]uint8, 4)
	setColorArray(a, v)
	*c = sdl.Color{a[0], a[1], a[2], a[3]}
}

// send the "GM system on" sysex message
func sendGMSystemOn(wr *writer.Writer) {
	writer.SysEx(wr, []byte{0x7e, 0x7f, 0x09, 0x01})
}

// replaces the suffix of a string, if present
func replaceSuffix(s, old, new_ string) string {
	if strings.HasSuffix(s, old) {
		return s[:len(s)-len(old)] + new_
	}
	return s
}
