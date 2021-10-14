package main

import (
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"

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
	fps          = 30
)

var (
	colorBg             = sdl.Color{0xf0, 0xf0, 0xf0, 0xff}
	colorBgArray        = []uint8{0xf0, 0xf0, 0xf0, 0xff}
	colorHighlightArray = []uint8{0xe0, 0xe0, 0xe0, 0xff}
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
	window, err := sdl.CreateWindow("Polyfauna", sdl.WINDOWPOS_UNDEFINED, sdl.WINDOWPOS_UNDEFINED,
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

	sng := &song{
		Tracks: []*track{
			&track{},
			&track{},
			&track{},
			&track{},
		},
	}
	patedit := &patternEditor{
		printer:  pr,
		song:     sng,
		division: 4,
	}

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
				label: "Cursor",
				items: []*menuItem{
					{label: "Go to beat...", action: func() { dialogGoToBeat(dia, patedit) }},
				},
			},
			{
				label: "Edit",
				items: []*menuItem{
					{label: "Insert note...", action: func() {
						dialogInsertNote(dia, patedit, wr)
					}},
					{label: "Insert note off", action: func() {
						patedit.writeEvent(newTrackEvent(&trackEvent{Type: noteOffEvent}))
					}},
					{label: "Delete events", action: func() {
						patedit.deleteSelectedEvents()
					}},
					{label: "Cut", action: func() { patedit.cut() }},
					{label: "Copy", action: func() { patedit.copy() }},
					{label: "Paste", action: func() { patedit.paste(false) }},
					{label: "Mix paste", action: func() { patedit.paste(true) }},
				},
			},
		},
	}
	mb.init(pr)

	for running {
		for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
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
					if event.Repeat == 0 {
						switch event.Keysym.Sym {
						case sdl.K_z:
							if event.State == sdl.PRESSED {
							}
						}
					}
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

		renderer.SetDrawColorArray(colorBgArray...)
		renderer.Clear()
		renderer.SetDrawColorArray(colorFgArray...)
		viewport := renderer.GetViewport()
		y := mb.menus[0].rect.H
		patedit.draw(renderer, &sdl.Rect{0, y, viewport.W, viewport.H - y})
		mb.draw(pr, renderer)
		dia.draw(pr, renderer)
		renderer.Present()
		sdl.Delay(1000 / fps)
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
				dialogMsg(d, "Invalid input.")
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
				if f >= -2 && f <= 129 {
					note, bend := pitchToMIDI(f)
					velocity := uint8(100)
					pe.writeEvent(newTrackEvent(&trackEvent{
						Type:      noteOnEvent,
						FloatData: f,
						ByteData1: velocity,
					}))
					writer.Pitchbend(wr, bend)
					writer.NoteOn(wr, note, velocity)
					writer.NoteOff(wr, note)
				} else {
					dialogMsg(d, "Note must be in the range [-2, 129].")
				}
			} else {
				dialogMsg(d, "Invalid syntax.")
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

// set d to an input dialog
func dialogOpen(d *dialog, sng *song, pe *patternEditor) {
	*d = *newDialog("Open:", 50, func(s string) {
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
		if err := sng.exportSMF(s); err != nil {
			dialogMsg(d, err.Error())
		}
	})
}

// read records from a TSV file
func readTSV(path string) ([][]string, error) {
	f, err := os.Open(shortcutsPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	r.Comment = '#'
	return r.ReadAll()
}
