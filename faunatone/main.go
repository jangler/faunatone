package main

import (
	"embed"
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
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
	appName      = "Faunatone"
	appVersion   = "v0.8.0"
	fileExt      = ".faun"
	defaultFps   = 60
	configPath   = "config"
	assetsPath   = "assets"
	savesPath    = "saves"
	exportsPath  = "exports"
	errorLogFile = "error.txt"
)

const (
	midiChannelIgnore int = iota
	midiChannelOctaves
)

var midiChannelBehavior int

//go:embed config/*
var embedFS embed.FS

var (
	bendSemitones     = 24
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

	posInf = math.Inf(1)
	negInf = math.Inf(-1)
)

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.WriteFile(errorLogFile, []byte(err.Error()+"\n"), 0644)
		os.Exit(1)
	}
}

func main() {
	dia := &dialog{}

	settings := loadSettings(func(s string) { println(s) })
	bendSemitones = settings.PitchBendSemitones
	setColorArray(colorBeatArray, settings.ColorBeat)
	setColorArray(colorBg1Array, settings.ColorBg1)
	setColorArray(colorBg2Array, settings.ColorBg2)
	setColorArray(colorFgArray, settings.ColorFg)
	setColorSDL(&colorFg, settings.ColorFg)
	setColorArray(colorPlayPosArray, settings.ColorPlayPos)
	setColorArray(colorSelectArray, settings.ColorSelect)
	switch settings.MidiInputChannels {
	case "ignore":
		midiChannelBehavior = midiChannelIgnore
	case "octaves":
		midiChannelBehavior = midiChannelOctaves
	default:
		dia.message(fmt.Sprintf("Invalid MidiInputChannels setting: %q", settings.MidiInputChannels))
	}
	padding = int32(settings.FontSize) / 2

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
			dia.message(fmt.Sprintf("MIDI input port index %d out of range [%d, %d].",
				n, 0, len(ins)))
		}
	}

	var wrs []writer.ChannelWriter
	outPorts, err := settings.parsedMidiOutPortNumbers()
	if err != nil {
		dia.message("Could not parse MidiOutPortNumber setting.")
	}
	for _, port := range outPorts {
		if port >= 0 {
			outs, err := drv.Outs()
			must(err)
			if port < len(outs) {
				out := outs[port]
				must(out.Open())
				defer out.Close()
				wr := writer.New(out)
				sendSystemOn(wr, 0)
				wrs = append(wrs, wr)
			} else {
				dia.message(fmt.Sprintf("MIDI output port index %d out of range [%d, %d].",
					port, 0, len(outs)))
			}
		}
	}
	if wrs == nil {
		wrs = append(wrs, writer.New(io.Discard)) // dummy output
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
	fontPath := joinTreePath(assetsPath, settings.Font)
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

	sng := newSong(nil)
	patedit := &patternEditor{
		printer:          pr,
		song:             sng,
		division:         defaultDivision,
		velocity:         defaultVelocity,
		controller:       defaultController,
		refPitch:         defaultRefPitch,
		historyIndex:     -1,
		historySizeLimit: settings.UndoBufferSize,
		offDivAlphaMod:   uint8(settings.OffDivisionAlpha),
		shiftScrollMult:  settings.ShiftScrollMult,
	}
	pl := newPlayer(sng, wrs, true)
	pl.redrawChan = redrawChan
	go pl.run()
	defer pl.cleanup()
	sng.Keymap, err = newKeymap(settings.DefaultKeymap)
	if err != nil {
		statusf(err.Error())
	}
	patedit.updateRefPitchDisplay()
	percKeymap, err := newKeymap(settings.PercussionKeymap)
	if err != nil {
		statusf(err.Error())
	}
	percKeymap.isPerc = true

	// required for cursor blink
	go func() {
		c := time.Tick(time.Millisecond * inputCursorBlinkMs)
		for range c {
			if dia.shown && dia.size > 0 {
				redrawChan <- true
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
					{label: "Export MIDI...", action: func() { dialogExportMidi(dia, sng, pl) }},
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
						pl.stop(false)
						sng.Keymap.clearActiveNotes()
						percKeymap.clearActiveNotes()
					}},
				},
			},
			{
				label: "Select",
				items: []*menuItem{
					{label: "Previous division", action: func() { patedit.moveCursor(0, -1) },
						repeat: true},
					{label: "Next division", action: func() { patedit.moveCursor(0, 1) },
						repeat: true},
					{label: "Previous track", action: func() { patedit.moveCursor(-1, 0) },
						repeat: true},
					{label: "Next track", action: func() { patedit.moveCursor(1, 0) },
						repeat: true},
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
						mode := cursorMidiMode(patedit, pl)
						if mode >= len(instrumentTargets) {
							dia.message("Unknown MIDI mode.")
							return
						}
						dialogInsertUint8Event(dia, patedit, pl,
							"Program:", programEvent, []int64{1, 0, 0},
							instrumentTargets[mode])
					}},
					{label: "Tempo change...", action: func() {
						dialogInsertTempoChange(dia, patedit, pl)
					}},
					{label: "Controller change...", action: func() {
						dialogInsertControlChange(dia, patedit, pl)
					}},
					{label: "Aftertouch...", action: func() {
						dialogInsertUint8Event(dia, patedit, pl,
							"Channel pressure:", channelPressureEvent, []int64{0}, nil)
					}},
					/* this is not part of GM level 1
					{label: "Polyphonic aftertouch...", action: func() {
						dialogInsertUint8Event(dia, patedit, pl,
							"Insert key pressure:", keyPressureEvent, 0, nil)
					}},
					*/
					{label: "Text...", action: func() {
						dialogInsertTextEvent(dia, patedit, pl)
					}},
					{label: "Release length...", action: func() {
						dialogInsertReleaseLen(dia, patedit, pl)
					}},
					{label: "MIDI channel range...", action: func() {
						dialogInsertMidiRange(dia, patedit, pl)
					}},
					{label: "MIDI output index...", action: func() {
						dialogInsertMidiOutput(dia, patedit, pl)
					}},
					{label: "MIDI mode...", action: func() {
						dialogInsertMidiMode(dia, patedit, pl)
					}},
					{label: "MT-32 global reverb...", action: func() {
						dialogInsertMT32Reverb(dia, patedit, pl)
					}},
				},
			},
			{
				label: "Edit",
				items: []*menuItem{
					{label: "Go to beat...", action: func() { dialogGoToBeat(dia, patedit) }},
					{label: "Delete events", action: func() {
						patedit.deleteSelectedEvents()
					}},
					{label: "Undo", action: func() { dia.messageIfErr(patedit.undo()) },
						repeat: true},
					{label: "Redo", action: func() { dia.messageIfErr(patedit.redo()) },
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
						dialogSetController(dia, sng, patedit, pl)
					}},
					{label: "Set division...", action: func() { dialogSetDivision(dia, patedit) }},
					{label: "Decrease division", action: func() { patedit.addDivision(-1) },
						repeat: true},
					{label: "Increase division", action: func() { patedit.addDivision(1) },
						repeat: true},
					{label: "Halve division", action: func() { patedit.multiplyDivision(0.5) }},
					{label: "Double division", action: func() { patedit.multiplyDivision(2) }},
					{label: "Toggle song follow", action: func() {
						patedit.followSong = !patedit.followSong
					}},
				},
			},
			{
				label: "Keymap",
				items: []*menuItem{
					{label: "Load...", action: func() {
						dialogLoadKeymap(dia, sng, patedit)
					}},
					{label: "Save as...", action: func() { dialogSaveKeymap(dia, sng) }},
					{label: "Import Scala scale...", action: func() {
						dialogImportScl(dia, sng, patedit)
					}},
					{label: "Remap key...", action: func() { dialogRemapKey(dia, sng, patedit) }},
					{label: "Generate equal division...", action: func() {
						dialogMakeEdoKeymap(dia, sng, patedit)
					}},
					{label: "Generate rank-2 scale...", action: func() {
						dialogMakeRank2Keyamp(dia, sng, patedit)
					}},
					{label: "Generate isomorphic layout...", action: func() {
						dialogMakeIsoKeymap(dia, sng, patedit)
					}},
					{label: "Display as CSV", action: func() { dialogDisplayKeymap(dia, sng) }},
					{label: "Change key signature...", action: func() {
						dialogChangeKeySig(dia, sng)
					}},
				},
			},
			{
				label: "Track",
				items: []*menuItem{
					{label: "Set channel...", action: func() {
						dialogTrackSetChannel(dia, sng, patedit)
					}},
					{label: "Insert", action: func() { patedit.insertTracks() }},
					{label: "Delete", action: func() { patedit.deleteTracks() }},
					{label: "Move left", action: func() { patedit.shiftTracks(-1) },
						repeat: true},
					{label: "Move right", action: func() { patedit.shiftTracks(1) },
						repeat: true},
				},
			},
			{
				label: "MIDI",
				items: []*menuItem{
					{label: "Display available inputs", action: func() {
						dialogMidiInputs(dia, drv)
					}},
					{label: "Display available outputs", action: func() {
						dialogMidiOutputs(dia, drv)
					}},
					{label: "Send pitch bend sensitivity RPN", action: func() {
						pl.signal <- playerSignal{typ: signalSendPitchRPN}
					}},
					{label: "Send system on", action: func() {
						pl.signal <- playerSignal{typ: signalSendSystemOn}
						pl.signal <- playerSignal{typ: signalSendPitchRPN}
					}},
					{label: "Cycle mode", action: func() {
						pl.signal <- playerSignal{typ: signalCycleMIDIMode}
					}},
				},
			},
		},
	}
	mb.init(pr)

	sb := newStatusBar(settings.MessageDuration,
		func() string { return fmt.Sprintf("Root: %s", patedit.refPitchDisplay) },
		func() string { return fmt.Sprintf("Division: %d", patedit.division) },
		func() string { return fmt.Sprintf("Velocity: %d", patedit.velocity) },
		func() string { return fmt.Sprintf("Controller: %d", patedit.controller) },
		func() string { return fmt.Sprintf("Mode: %s", midiModeName(sng.MidiMode)) },
		func() string { return fmt.Sprintf("Keymap: %s", sng.Keymap.Name) },
		func() string { return conditionalString(patedit.followSong, "Follow", "") },
		func() string { return conditionalString(keyjazz, "Keyjazz", "") },
	)

	// attempt to load save file specified by first CLI arg
	if len(os.Args) > 1 {
		path := os.Args[1]
		if f, err := os.Open(path); err == nil {
			if err := sng.read(f); err == nil {
				statusf("Loaded %s.", path)
				saveAutofill = filepath.Base(path)
				exportAutofill = replaceSuffix(saveAutofill, fileExt, ".mid")
			} else {
				dia.message(err.Error())
			}
			f.Close()
		} else {
			dia.message(err.Error())
		}
	}

	for running {
		// process SDL events
	sdlEvents:
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
				break sdlEvents
			}
		}

		// process MIDI events
	midiEvents:
		for {
			select {
			case msg := <-midiIn:
				if dia.shown {
					dia.midiEvent(msg.Raw())
					redrawChan <- true
				} else {
					switch msg.Raw()[0] & 0xf0 {
					case 0x80, 0x90: // note off, note on
						sng.Keymap.midiEvent(msg.Raw(), patedit, pl, keyjazz)
					}
				}
			default:
				break midiEvents
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
			patedit.draw(renderer, &sdl.Rect{X: 0, Y: y, W: viewport.W, H: viewport.H - y - sb.rect.H},
				pl.lastTick)
			sb.draw(pr, renderer, redrawChan)
			mb.draw(pr, renderer)
			dia.draw(pr, renderer)
			renderer.Present()
		}
		sdl.Delay(uint32(1000 / fps))
	}
}

func cursorMidiMode(patedit *patternEditor, pl *player) int {
	i := patedit.song.Tracks[patedit.cursorTrackClick].Channel
	return pl.virtChannels[i].midiMode
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
	d.getFloat("Beat:", 1, posInf, func(f float64) {
		pe.goToBeat(f)
	})
}

// set d to an input dialog
func dialogInsertNote(d *dialog, pe *patternEditor, p *player) {
	*d = *newDialog("Interval:", 7, func(s string) {
		if ps, err := parsePitch(s, pe.song.Keymap); err == nil {
			f := math.Min(maxPitch, math.Max(minPitch, ps.semitones()+pe.refPitch))
			track, _, _, _ := pe.getSelection()
			pe.writeEvent(newTrackEvent(&trackEvent{
				Type:      noteOnEvent,
				FloatData: f,
				ByteData1: pe.velocity,
				track:     track,
			}, pe.song.Keymap), p)
		} else {
			d.message(err.Error())
		}
	})
}

// return note and pitch wheel values required to play a pitch in MIDI
func pitchToMidi(p float64, midiMode int) (uint8, int16) {
	note := uint8(math.Round(math.Max(0, math.Min(127, p))))
	bend := int16((p - float64(note)) * 8192.0 / getBendSemitones(midiMode))
	return note, bend
}

// set to d an input dialog
func dialogInsertDrumNote(d *dialog, pe *patternEditor, p *player) {
	mode := cursorMidiMode(pe, p)
	if mode >= len(drumTargets) {
		d.message("Unknown MIDI mode.")
		return
	}
	d.getNamedInts("Pitch:", []int64{0}, drumTargets[mode],
		func(i []int64) {
			track, _, _, _ := pe.getSelection()
			pe.writeEvent(newTrackEvent(&trackEvent{
				Type:      drumNoteOnEvent,
				ByteData1: uint8(i[0]),
				ByteData2: pe.velocity,
				track:     track,
			}, nil), p)
		})
}

// set d to a key dialog
func dialogInsertPitchBend(d *dialog, pe *patternEditor, p *player) {
	*d = *newDialog("Bend to key...", 0, func(s string) {
		if f, ok := pe.song.Keymap.pitchFromString(s, pe.refPitch); ok {
			pe.writeEvent(newTrackEvent(&trackEvent{
				Type:      pitchBendEvent,
				FloatData: f,
			}, pe.song.Keymap), p)
		} else {
			d.message("Key not in keymap.")
		}
	})
	d.mode = noteInput
}

// set d to an input dialog
func dialogInsertTempoChange(d *dialog, pe *patternEditor, p *player) {
	// using 0.01 here since the error msg only displays 2 decimal places
	d.getTempo("Tempo (BPM):", 0.01, posInf, func(f float64) {
		pe.writeEvent(newTrackEvent(&trackEvent{
			Type:      tempoEvent,
			FloatData: f,
		}, nil), p)
	}, func(n, d uint64) {
		pe.writeEvent(newTrackEvent(&trackEvent{
			Type:      tempoEvent,
			ByteData1: byte(n),
			ByteData2: byte(d),
		}, nil), p)
	})
}

// set d to an input dialog
func dialogInsertControlChange(d *dialog, pe *patternEditor, p *player) {
	d.getInt("Controller value:", 0, 127, func(i int64) {
		pe.writeEvent(newTrackEvent(&trackEvent{
			Type:      controllerEvent,
			ByteData1: pe.controller,
			ByteData2: byte(i),
		}, nil), p)
	})
}

// set d to an input dialog
func dialogInsertUint8Event(d *dialog, pe *patternEditor, p *player, prompt string,
	et trackEventType, offsets []int64, targets []*tabTarget) {
	d.getNamedInts(prompt, offsets, targets, func(ints []int64) {
		byteData := []byte{0, 0, 0}
		for i := range byteData {
			if i < len(ints) {
				if i < len(offsets) {
					byteData[i] = byte(ints[i] - offsets[i])
				} else {
					byteData[i] = byte(ints[i])
				}
			}
		}
		pe.writeEvent(newTrackEvent(&trackEvent{
			Type:      et,
			ByteData1: byteData[0],
			ByteData2: byteData[1],
			ByteData3: byteData[2],
		}, nil), p)
	})
}

// set d to an input dialog chain
func dialogInsertTextEvent(d *dialog, pe *patternEditor, p *player) {
	min, max := 1, 9
	d.getNamedInts("Meta-event type:", []int64{0}, metaEvents,
		func(i []int64) {
			if i[0] >= int64(min) && i[0] <= int64(max) {
				*d = *newDialog("Text:", 100, func(s string) {
					pe.writeEvent(newTrackEvent(&trackEvent{
						Type:      textEvent,
						ByteData1: byte(i[0]),
						TextData:  s,
					}, nil), p)
				})
			} else {
				s := fmt.Sprintf("Value must be in range [%d, %d].", min, max)
				d.message(s)
			}
		})
}

// set d to an input dialog
func dialogInsertReleaseLen(d *dialog, pe *patternEditor, p *player) {
	d.getFloat("Release length in beats:", negInf, posInf, func(f float64) {
		pe.writeEvent(newTrackEvent(&trackEvent{
			Type:      releaseLenEvent,
			FloatData: f,
		}, nil), p)
	})
}

// set d to an input dialog chain
func dialogInsertMidiRange(d *dialog, pe *patternEditor, p *player) {
	d.getInt("Minimum MIDI channel:", 1, 16, func(min int64) {
		d.getInt("Maximum MIDI channel:", min, 16, func(max int64) {
			pe.writeEvent(newTrackEvent(&trackEvent{
				Type:      midiRangeEvent,
				ByteData1: byte(min - 1),
				ByteData2: byte(max - 1),
			}, nil), p)
		})
	})
}

// set d to an input dialog
func dialogInsertMidiOutput(d *dialog, pe *patternEditor, p *player) {
	d.getInt("Index of MIDI output in settings.csv list:", 0, 127, func(i int64) {
		pe.writeEvent(newTrackEvent(&trackEvent{
			Type:      midiOutputEvent,
			ByteData1: byte(i),
		}, nil), p)
	})
}

// set d to an input dialog
func dialogInsertMidiMode(d *dialog, pe *patternEditor, p *player) {
	d.getNamedInts("MIDI mode:", []int64{0}, midiModeTargets(), func(xs []int64) {
		pe.writeEvent(newTrackEvent(&trackEvent{
			Type:      midiModeEvent,
			ByteData1: byte(xs[0]),
		}, nil), p)
	})
}

// set d to an input dialog chain
func dialogInsertMT32Reverb(d *dialog, pe *patternEditor, p *player) {
	d.getInt("Mode (0-3 = room, hall, plate, tap delay):", 0, 3, func(mode int64) {
		d.getInt("Time (0-7):", 0, 7, func(time int64) {
			d.getInt("Level (0-7):", 0, 7, func(level int64) {
				pe.writeEvent(newTrackEvent(&trackEvent{
					Type:      mt32ReverbEvent,
					ByteData1: byte(mode),
					ByteData2: byte(time),
					ByteData3: byte(level),
				}, nil), p)
			})
		})
	})
}

// set d to an input dialog
func dialogSetController(d *dialog, s *song, pe *patternEditor, pl *player) {
	mode := cursorMidiMode(pe, pl)
	if mode >= len(ccTargets) {
		d.message("Unknown MIDI mode.")
		return
	}
	d.getNamedInts("Controller index:", []int64{0}, ccTargets[mode],
		func(i []int64) {
			pe.controller = uint8(i[0])
		})
}

// set d to an input dialog
func dialogSetDivision(d *dialog, pe *patternEditor) {
	d.getInt("Division:", 1, ticksPerBeat, func(i int64) {
		pe.division = int(i)
		pe.fixCursor()
	})
}

// set d to an input dialog
func dialogTranpose(d *dialog, pe *patternEditor) {
	*d = *newDialog("Transpose selection by key...", 0, func(s string) {
		if f, ok := pe.song.Keymap.pitchFromString(s, 0); ok {
			pe.transposeSelection(f)
		} else {
			d.message("Key not in keymap.")
		}
	})
	d.mode = noteInput
}

// set d to an input dialog
func dialogMultiply(d *dialog, pe *patternEditor) {
	d.getFloat("Multiply selection by:", 0, posInf, func(f float64) {
		pe.multiplySelection(f)
	})
}

// set d to an input dialog
func dialogVary(d *dialog, pe *patternEditor) {
	d.getFloat("Vary selection by:", 0, posInf, func(f float64) {
		pe.varySelection(f)
	})
}

// set d to an input dialog
func dialogSetVelocity(d *dialog, pe *patternEditor) {
	d.getInt("Velocity:", 0, 127, func(i int64) {
		pe.velocity = uint8(i)
	})
}

// set d to a key dialog, then input dialog
func dialogRemapKey(d *dialog, s *song, pe *patternEditor) {
	*d = *newDialog("Remap key...", 0, func(s1 string) {
		*d = *newDialog("Interval:", 7, func(s2 string) {
			if ps, err := parsePitch(s2, s.Keymap); err == nil {
				ki := newKeyInfo(s1, strings.HasPrefix(s2, "*"), "", ps)
				if existing := s.Keymap.getByKey(s1); existing == nil {
					s.Keymap.Items = append(s.Keymap.Items, ki)
				} else {
					*existing = *ki
				}
				s.Keymap.Name = addSuffixIfMissing(s.Keymap.Name, "*")
				s.Keymap.setMidiPattern()
				s.renameNotes()
				pe.updateRefPitchDisplay()
				statusf("Remapped %s.", s1)
			} else {
				d.message(err.Error())
			}
		})
	})
	d.mode = noteInput
}

// set d to an input dialog
func dialogLoadKeymap(d *dialog, sng *song, pe *patternEditor) {
	d.getPath("Load keymap:", keymapPath, ".csv", true, func(s string) {
		s = addSuffixIfMissing(s, ".csv")
		if k, err := newKeymap(s); err == nil {
			sng.Keymap = k
			sng.renameNotes()
			pe.updateRefPitchDisplay()
		} else {
			d.message(err.Error())
		}
	})
}

// set d to an input dialog
func dialogSaveKeymap(d *dialog, sng *song) {
	d.getPath("Save keymap as:", keymapPath, ".csv", false, func(s string) {
		s = addSuffixIfMissing(s, ".csv")
		if err := sng.Keymap.write(s); err != nil {
			d.message(err.Error())
		} else {
			statusf("Wrote %s.", s)
		}
	})
	d.input = addSuffixIfMissing(sng.Keymap.Name, ".csv")
	d.updateCurTargets()
}

// set d to a message dialog
func dialogDisplayKeymap(d *dialog, sng *song) {
	d.message(sng.Keymap.String())
}

// set d to an input dialog
func dialogImportScl(d *dialog, sng *song, pe *patternEditor) {
	d.getPath("Import Scala scale:", keymapPath, ".scl", true, func(s string) {
		s = addSuffixIfMissing(s, ".scl")
		if k, err := keymapFromSclFile(s); err == nil {
			sng.Keymap = k
			sng.renameNotes()
			pe.updateRefPitchDisplay()
		} else {
			d.message(err.Error())
		}
	})
}

// set d to an input dialog chain
func dialogMakeEdoKeymap(d *dialog, sng *song, pe *patternEditor) {
	d.getInterval("Interval to divide:", sng.Keymap, func(ps *pitchSrc) {
		d.getInt("Number of divisions:", 1, 127, func(i int64) {
			sng.Keymap = genEqualDivisionKeymap(ps.semitones(), int(i))
			sng.renameNotes()
			pe.updateRefPitchDisplay()
		})
	})
}

// set d to an input dialog chain
func dialogMakeRank2Keyamp(d *dialog, sng *song, pe *patternEditor) {
	d.getInterval("Period:", sng.Keymap, func(per *pitchSrc) {
		d.getInterval("Generator:", sng.Keymap, func(gen *pitchSrc) {
			d.getInt("Number of notes:", 1, 127, func(i int64) {
				if k, err := genRank2Keymap(per, gen, int(i)); err == nil {
					sng.Keymap = k
					sng.renameNotes()
					pe.updateRefPitchDisplay()
				} else {
					d.message(err.Error())
				}
			})
		})
	})
}

// set d to an input dialog chain
func dialogMakeIsoKeymap(d *dialog, sng *song, pe *patternEditor) {
	d.getInterval("First interval:", sng.Keymap, func(ps1 *pitchSrc) {
		d.getInterval("Second interval:", sng.Keymap, func(ps2 *pitchSrc) {
			sng.Keymap = genIsoKeymap(ps1, ps2)
			sng.renameNotes()
			pe.updateRefPitchDisplay()
		})
	})
}

// set d to an input dialog chain
func dialogChangeKeySig(d *dialog, sng *song) {
	*d = *newDialog("Input keys and accidentals, then press Enter:\n...", 0, func(s1 string) {
		sng.Keymap.keySig = d.keySig
	})
	d.mode, d.keymap, d.keySig = keySigInput, sng.Keymap, copyKeySig(sng.Keymap.keySig)
}

// set d to a y/n dialog
func dialogNew(d *dialog, sng *song, pe *patternEditor, p *player) {
	*d = *newDialog("Create new song? (y/n)", 0, func(s string) {
		p.stop(true)
		p.signal <- playerSignal{typ: signalResetChannels}
		*sng = *newSong(sng.Keymap)
		pe.reset()
		saveAutofill = ""
		exportAutofill = ""
	})
	d.mode = yesNoInput
}

// set d to an input dialog
func dialogOpen(d *dialog, sng *song, pe *patternEditor, p *player) {
	d.getPath("Load song:", savesPath, ".faun", true, func(s string) {
		s = addSuffixIfMissing(s, fileExt)
		if f, err := os.Open(joinTreePath(savesPath, s)); err == nil {
			defer f.Close()
			p.stop(true)
			p.signal <- playerSignal{typ: signalResetChannels}
			if err := sng.read(f); err == nil {
				pe.reset()

				// needed when loading a file in a different midi mode
				p.signal <- playerSignal{typ: signalSendSystemOn}
				p.signal <- playerSignal{typ: signalSendPitchRPN}

				saveAutofill = s
				exportAutofill = replaceSuffix(s, fileExt, ".mid")
			} else {
				d.message(err.Error())
			}
		} else {
			d.message(err.Error())
		}
	})
}

// set d to an input dialog
func dialogSaveAs(d *dialog, sng *song) {
	d.getPath("Save song as:", savesPath, ".faun", false, func(s string) {
		s = addSuffixIfMissing(s, fileExt)
		saveAutofill = s
		if exportAutofill == "" {
			exportAutofill = replaceSuffix(s, fileExt, ".mid")
		}
		os.MkdirAll(joinTreePath(savesPath), 0755)
		if f, err := os.Create(joinTreePath(savesPath, s)); err == nil {
			defer f.Close()
			if err := sng.write(f); err != nil {
				d.message(err.Error())
			} else {
				statusf("Wrote %s.", s)
			}
		} else {
			d.message(err.Error())
		}
	})
	d.input = saveAutofill
	d.updateCurTargets()
}

// set d to an input dialog
func dialogExportMidi(d *dialog, sng *song, p *player) {
	d.getPath("Export song as:", exportsPath, ".mid", false, func(s string) {
		s = addSuffixIfMissing(s, ".mid")
		exportAutofill = s
		if saveAutofill == "" {
			saveAutofill = replaceSuffix(s, ".mid", fileExt)
		}
		p.stop(true) // avoid race condition
		os.MkdirAll(joinTreePath(exportsPath), 0755)
		if err := sng.exportSMF(joinTreePath(exportsPath, s)); err != nil {
			d.message(err.Error())
		} else {
			statusf("Wrote %s.", s)
		}
	})
	d.input = exportAutofill
	d.updateCurTargets()
}

// set d to a message dialog
func dialogMidiInputs(d *dialog, drv *driver.Driver) {
	if ins, err := drv.Ins(); err == nil {
		a := make([]string, len(ins)+1)
		a[0] = "Available MIDI inputs:"
		for i, v := range ins {
			a[i+1] = fmt.Sprintf("[%d] %s", i, v)
		}
		d.message(strings.Join(a, "\n"))
	} else {
		d.message(err.Error())
	}
}

// set d to a message dialog
func dialogMidiOutputs(d *dialog, drv *driver.Driver) {
	if outs, err := drv.Outs(); err == nil {
		a := make([]string, len(outs)+1)
		a[0] = "Available MIDI outputs:"
		for i, v := range outs {
			a[i+1] = fmt.Sprintf("[%d] %s", i, v)
		}
		d.message(strings.Join(a, "\n"))
	} else {
		d.message(err.Error())
	}
}

// set d to an input dialog
func dialogTrackSetChannel(d *dialog, sng *song, pe *patternEditor) {
	d.getInt("Channel:", 1, numVirtualChannels, func(i int64) {
		pe.setTrackChannel(uint8(i - 1))
	})
}

// read records from a CSV file
func readCSV(path string, embed bool) ([][]string, error) {
	var f io.ReadCloser
	var err error
	if embed {
		f, err = embedFS.Open(path)
	} else {
		f, err = os.Open(path)
	}
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
	w := csv.NewWriter(f)
	w.UseCRLF = runtime.GOOS == "windows"
	return w.WriteAll(records)
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
	*c = sdl.Color{R: a[0], G: a[1], B: a[2], A: a[3]}
}

// send the "GM system on" sysex message
func sendSystemOn(wr writer.ChannelWriter, midiMode int) {
	if midiMode < len(systemOnBytes) {
		writer.SysEx(wr, systemOnBytes[midiMode])
	}
	switch midiMode {
	case modeMT32:
		// set partial reserves for dynamic allocation
		sysex([]byte{
			0x41, 0x10, 0x16, 0x12, 0x10, 0x00, 0x04,
			0x03, 0x03, 0x03, 0x03, 0x03, 0x03, 0x03, 0x03, 0x08,
		}, wr, midiMode)
	case modeMPE:
		// send MPE configuration message
		// assign all channels to lower zone
		wr.SetChannel(0)
		writer.RPN(wr, 0x00, 0x06, 0xf, 0)
	}
}

// replaces the suffix of a string, if present
func replaceSuffix(s, old, new_ string) string {
	if strings.HasSuffix(s, old) {
		return s[:len(s)-len(old)] + new_
	}
	return s
}

// cached for joinTreePath
var (
	exePath    string
	useExePath bool
)

// like filepath.Join, but relative to the executable's dir. falls back onto
// the working dir if the exe-relative path doesn't have config dir
func joinTreePath(elem ...string) string {
	if exePath == "" {
		var err error
		if exePath, err = os.Executable(); err == nil {
			if exePath, err = filepath.EvalSymlinks(exePath); err == nil {
				_, err := os.Stat(filepath.Join(filepath.Dir(exePath), configPath))
				useExePath = err == nil
			} else {
				statusf(err.Error())
			}
		} else {
			statusf(err.Error())
		}
	}
	if useExePath {
		return filepath.Join(append([]string{filepath.Dir(exePath)}, elem...)...)
	}
	return filepath.Join(elem...)
}

func getBendSemitones(midiMode int) float64 {
	if midiMode == modeMT32 {
		return 12
	}
	return float64(bendSemitones)
}
