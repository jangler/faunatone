package main

import (
	"github.com/veandco/go-sdl2/sdl"
)

// type that draws a series of string function results in a line
type statusBar struct {
	rect  *sdl.Rect
	funcs []func() string
}

// draw the status bar
func (sb *statusBar) draw(pr *printer, r *sdl.Renderer) {
	x := int32(padding)
	y := r.GetViewport().H - pr.rect.H - padding
	r.SetDrawColorArray(colorHighlightArray...)
	*sb.rect = sdl.Rect{x - padding, y - padding, r.GetViewport().W, pr.rect.H + padding*2}
	r.FillRect(sb.rect)
	for _, f := range sb.funcs {
		s := f()
		if s != "" {
			pr.draw(r, s, x, y)
			x += padding*2 + pr.rect.W*int32(len(s))
		}
	}
}
