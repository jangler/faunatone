package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/veandco/go-sdl2/sdl"
	"github.com/veandco/go-sdl2/ttf"
	"gitlab.com/gomidi/midi/writer"
	driver "gitlab.com/gomidi/rtmididrv"
)

const (
	windowWidth  = 1280
	windowHeight = 720
	fontSize     = 14
	fps          = 30
)

var (
	colorBg             = sdl.Color{0xf0, 0xf0, 0xf0, 0xff}
	colorBgArray        = []uint8{0xf0, 0xf0, 0xf0, 0xff}
	colorHighlightArray = []uint8{0xe0, 0xe0, 0xe0, 0xff}
	colorFg             = sdl.Color{0x10, 0x10, 0x10, 0xff}
	colorFgArray        = []uint8{0x10, 0x10, 0x10, 0xff}

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

	running := true

	mb := menuBar([]*menu{
		{
			label: "File",
			items: []*menuItem{
				{label: "Open"},
				{label: "Save"},
				{label: "Export"},
				{label: "Quit", action: func() { running = false }},
			},
		},
		{
			label: "MIDI",
			items: []*menuItem{
				{label: "Reset all controllers"},
				{label: "All notes off"},
			},
		},
	})
	mb.init(pr)

	for running {
		for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
			switch event := event.(type) {
			case *sdl.MouseMotionEvent:
				mb.mouseMotion(event)
			case *sdl.MouseButtonEvent:
				mb.mouseButton(event)
			case *sdl.KeyboardEvent:
				if event.Repeat == 0 && event.Keysym.Sym == sdl.K_z {
					if event.State == sdl.PRESSED {
						if err := writer.NoteOn(wr, 60, 100); err != nil {
							log.Print(err)
						}
					} else {
						if err := writer.NoteOff(wr, 60); err != nil {
							log.Print(err)
						}
					}
				}
			case *sdl.QuitEvent:
				running = false
				break
			}
		}

		renderer.SetDrawColorArray(colorBgArray...)
		renderer.Clear()
		renderer.SetDrawColorArray(colorFgArray...)
		pr.draw(renderer, "Hello, Polyfauna user.", 100, 100)
		mb.draw(pr, renderer)
		renderer.Present()
		sdl.Delay(1000 / fps)
	}
}
