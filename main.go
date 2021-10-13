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
	fontSize     = 16
	padding      = 8
	fps          = 30
)

var (
	colorBg      = sdl.Color{0xf0, 0xf0, 0xf0, 0xff}
	colorBgArray = []uint8{0xf0, 0xf0, 0xf0, 0xff}
	colorFg      = sdl.Color{0x10, 0x10, 0x10, 0xff}
	colorFgArray = []uint8{0x10, 0x10, 0x10, 0xff}

	// TODO load font from RW instead of file
	fontPath = filepath.Join("assets", "RobotoMono-Regular-BasicLatin.ttf")
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
		windowWidth, windowHeight, sdl.WINDOW_SHOWN)
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
	surface, err := font.RenderUTF8Blended("Hello, Polyfauna user.", colorFg)
	must(err)
	defer surface.Free()
	texture, err := renderer.CreateTextureFromSurface(surface)
	must(err)
	defer texture.Destroy()

	running := true
	for running {
		for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
			switch event := event.(type) {
			case *sdl.QuitEvent:
				running = false
				break
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
			}
		}

		renderer.SetDrawColorArray(colorBgArray...)
		renderer.Clear()
		renderer.SetDrawColorArray(colorFgArray...)
		renderer.Copy(texture, &sdl.Rect{0, 0, surface.W, surface.H},
			&sdl.Rect{padding, padding, surface.W, surface.H})
		renderer.Present()
		sdl.Delay(1000 / fps)
	}
}
