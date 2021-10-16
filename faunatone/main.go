package main

import (
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/veandco/go-sdl2/sdl"
	"github.com/veandco/go-sdl2/ttf"
	"gitlab.com/gomidi/midi/writer"
	driver "gitlab.com/gomidi/rtmididrv"
)

const (
	windowWidth  = 1280
	windowHeight = 720
	fontSize     = 14
	padding      = fontSize / 2
	appName      = "Faunatone"
	fileExt      = ".fna"
	defaultFps   = 60
)

var (
	colorBg             = sdl.Color{0xf0, 0xf0, 0xf0, 0xff}
	colorBgArray        = []uint8{0xf0, 0xf0, 0xf0, 0xff}
	colorHighlightArray = []uint8{0xe0, 0xe0, 0xe0, 0xff}
	colorPlayPosArray   = []uint8{0xe8, 0xe8, 0xe8, 0xff}
	colorFg             = sdl.Color{0x10, 0x10, 0x10, 0xff}
	colorFgArray        = []uint8{0x10, 0x10, 0x10, 0xff}

	configPath = "config"

	// TODO load font from RW instead of file
	fontPath = filepath.Join("assets", "RobotoMono-Regular-BasicLatin.ttf")

	uiScale = 1
)

func must(err error) {
	if err != nil {
		panic(err.Error())
	}
}

func main() {
	drv, err := driver.New()
	must(err)
	defer drv.Close()

	outs, err := drv.Outs()
	must(err)

	if len(os.Args) == 2 && os.Args[1] == "list" {
		for _, port := range outs {
			fmt.Printf("[%v] %s\n", port.Number(), port)
		}
	}

	out := outs[0]
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
	font, err := ttf.OpenFont(fontPath, fontSize)
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
	println(fps)

	sng := &song{
		Tracks: []*track{
			&track{},
			&track{},
			&track{},
			&track{},
		},
	}
	patedit := &patternEditor{
		printer:    pr,
		song:       sng,
		division:   defaultDivision,
		velocity:   defaultVelocity,
		controller: defaultController,
		refPitch:   defaultRefPitch,
	}
	pl := newPlayer(sng, wr, true)
	pl.redrawChan = redrawChan
	go pl.run()
	km, _ := newKeymap(defaultKeymapPath)
	dia := &dialog{}

	running := true

	mb := &menuBar{
		menus: []*menu{
			{
				label: "File",
				items: []*menuItem{
					{label: "Open...", action: func() { dialogOpen(dia, sng, patedit) }},
					{label: "Save as...", action: func() { dialogSaveAs(dia, sng) }},
					{label: "Export MIDI...", action: func() { dialogExportMIDI(dia, sng) }},
					{label: "Quit", action: func() { running = false }},
				},
			},
			{
				label: "Play",
				items: []*menuItem{
					{label: "Song", action: func() {
						go func() {
							pl.signal <- playerSignal{typ: signalStart}
						}()
					}},
					{label: "From cursor", action: func() {
						go func() {
							_, _, minTick, _ := patedit.getSelection()
							pl.signal <- playerSignal{typ: signalStart, tick: minTick}
						}()
					}},
					{label: "Stop", action: func() {
						go func() {
							pl.signal <- playerSignal{typ: signalStop}
						}()
					}},
				},
			},
			{
				label: "Cursor",
				items: []*menuItem{
					{label: "Previous division", action: func() { patedit.moveCursor(0, -1) }},
					{label: "Next division", action: func() { patedit.moveCursor(0, 1) }},
					{label: "Previous track", action: func() { patedit.moveCursor(-1, 0) }},
					{label: "Next track", action: func() { patedit.moveCursor(1, 0) }},
					{label: "Go to beat...", action: func() { dialogGoToBeat(dia, patedit) }},
				},
			},
			{
				label: "Edit",
				items: []*menuItem{
					{label: "Insert note...", action: func() {
						dialogInsertNote(dia, patedit, wr)
					}},
					{label: "Insert drum note...", action: func() {
						dialogInsertDrumNote(dia, patedit, wr)
					}},
					{label: "Insert note off", action: func() {
						patedit.writeEvent(newTrackEvent(&trackEvent{Type: noteOffEvent}))
					}},
					{label: "Insert program change...", action: func() {
						dialogInsertProgramChange(dia, patedit, wr)
					}},
					{label: "Insert tempo change...", action: func() {
						dialogInsertTempoChange(dia, patedit)
					}},
					{label: "Insert control change...", action: func() {
						dialogInsertControlChange(dia, patedit)
					}},
					{label: "Delete events", action: func() {
						patedit.deleteSelectedEvents()
					}},
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
					{label: "Decrease octave", action: func() { patedit.modifyRefPitch(-12) }},
					{label: "Increase octave", action: func() { patedit.modifyRefPitch(12) }},
					{label: "Capture root pitch", action: func() { patedit.captureRefPitch() }},
					{label: "Set velocity...", action: func() { dialogSetVelocity(dia, patedit) }},
					{label: "Set controller...", action: func() {
						dialogSetController(dia, patedit)
					}},
					{label: "Decrease division", action: func() { patedit.addDivision(-1) }},
					{label: "Increase division", action: func() { patedit.addDivision(1) }},
					{label: "Halve division", action: func() { patedit.multiplyDivision(0.5) }},
					{label: "Double division", action: func() { patedit.multiplyDivision(2) }},
					{label: "Remap key...", action: func() { dialogRemapKey(dia, km) }},
					{label: "Load keymap...", action: func() { dialogLoadKeymap(dia, km) }},
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
					{label: "Move left", action: func() { patedit.shiftTracks(-1) }},
					{label: "Move right", action: func() { patedit.shiftTracks(1) }},
				},
			},
		},
	}
	mb.init(pr)

	sb := statusBar{
		rect: &sdl.Rect{},
		funcs: []func() string{
			func() string { return fmt.Sprintf("Root: %.2f", patedit.refPitch) },
			func() string { return fmt.Sprintf("Division: %d", patedit.division) },
			func() string { return fmt.Sprintf("Velocity: %d", patedit.velocity) },
			func() string { return fmt.Sprintf("Controller: %d", patedit.controller) },
			func() string { return fmt.Sprintf("Keymap: %s", km.name) },
		},
	}

	for running {
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
					km.keyboardEvent(event, patedit, wr)
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

		// hack to prevent Alt+<letter> from typing <letter> into dialog
		dia.accept = dia.shown

		if redraw {
			redrawChan <- false
			renderer.SetDrawColorArray(colorBgArray...)
			renderer.Clear()
			renderer.SetDrawColorArray(colorFgArray...)
			viewport := renderer.GetViewport()
			y := mb.menus[0].rect.H
			patedit.draw(renderer, &sdl.Rect{0, y, viewport.W, viewport.H - y - sb.rect.H}, pl.lastTick)
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

// set d to an input dialog
func dialogInsertNote(d *dialog, pe *patternEditor, wr *writer.Writer) {
	*d = dialog{
		prompt: "Insert note:",
		size:   7,
		action: func(s string) {
			if f, err := strconv.ParseFloat(s, 64); err == nil {
				if f >= minPitch && f <= maxPitch {
					note, bend := pitchToMIDI(f)
					pe.writeEvent(newTrackEvent(&trackEvent{
						Type:      noteOnEvent,
						FloatData: f,
						ByteData1: pe.velocity,
					}))
					wr.SetChannel(0)
					writer.Pitchbend(wr, bend)
					writer.NoteOn(wr, note, pe.velocity)
					writer.NoteOff(wr, note)
				} else {
					dialogMsg(d, fmt.Sprintf("Note must be in the range [-%.f, %.f].",
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
	note := uint8(math.Max(0, math.Min(127, p)))
	bend := int16((p - float64(note)) * 4096)
	return note, bend
}

// set to d an input dialog
func dialogInsertDrumNote(d *dialog, pe *patternEditor, wr *writer.Writer) {
	*d = *newDialog("Insert drum note:", 3, func(s string) {
		if i, err := strconv.ParseUint(s, 10, 8); err == nil {
			if i < 128 {
				pe.writeEvent(newTrackEvent(&trackEvent{
					Type:      drumNoteOnEvent,
					ByteData1: uint8(i),
					ByteData2: pe.velocity,
				}))
				wr.SetChannel(percussionChannelIndex)
				writer.NoteOn(wr, uint8(i), pe.velocity)
				writer.NoteOff(wr, uint8(i))
			} else {
				dialogMsg(d, "Note must be in the range [0, 127].")
			}
		} else {
			dialogMsg(d, err.Error())
		}
	})
}

// set d to an input dialog
func dialogInsertProgramChange(d *dialog, pe *patternEditor, wr *writer.Writer) {
	*d = *newDialog("Insert program change:", 3, func(s string) {
		if i, err := strconv.ParseUint(s, 10, 8); err == nil {
			if i >= 1 && i <= 128 {
				pe.writeEvent(newTrackEvent(&trackEvent{
					Type:      programEvent,
					ByteData1: byte(i - 1),
				}))
				wr.SetChannel(0)
				writer.ProgramChange(wr, uint8(i-1))
			} else {
				dialogMsg(d, "Program must be in the range [1, 128].")
			}
		} else {
			dialogMsg(d, err.Error())
		}
	})
}

// set d to an input dialog
func dialogInsertTempoChange(d *dialog, pe *patternEditor) {
	*d = *newDialog("Insert tempo change:", 7, func(s string) {
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			if f > 0 {
				pe.writeEvent(newTrackEvent(&trackEvent{
					Type:      tempoEvent,
					FloatData: f,
				}))
			} else {
				dialogMsg(d, "Tempo must be above zero.")
			}
		} else {
			dialogMsg(d, err.Error())
		}
	})
}

// set d to an input dialog
func dialogInsertControlChange(d *dialog, pe *patternEditor) {
	*d = *newDialog("Controller value:", 3, func(s string) {
		if i, err := strconv.ParseUint(s, 10, 8); err == nil {
			pe.writeEvent(newTrackEvent(&trackEvent{
				Type:      controllerEvent,
				ByteData1: pe.controller,
				ByteData2: byte(i),
			}))
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
		s = addSuffixIfMissing(s, ".tsv")
		if k2, err := newKeymap(s); err == nil {
			*k = *k2
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
func dialogExportMIDI(d *dialog, sng *song) {
	*d = *newDialog("Export as:", 50, func(s string) {
		s = addSuffixIfMissing(s, ".mid")
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

// read records from a TSV file
func readTSV(path string) ([][]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
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
