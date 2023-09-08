package main

import (
	"log"

	"github.com/veandco/go-sdl2/sdl"
	"github.com/veandco/go-sdl2/ttf"
)

// type that renders strings piecewise by caching rendered glyphs
type printer struct {
	font     *ttf.Font
	textures map[rune]*sdl.Texture
	rect     *sdl.Rect // size of an individual glyph
}

// create a new initialized printer from a fixed-width font
func newPrinter(f *ttf.Font) (*printer, error) {
	w, h, err := f.SizeUTF8("A")
	if err != nil {
		return nil, err
	}
	return &printer{
		font:     f,
		textures: make(map[rune]*sdl.Texture),
		rect:     &sdl.Rect{X: 0, Y: 0, W: int32(w), H: int32(h)},
	}, nil
}

// free the printer's resources
func (p *printer) destroy() {
	for _, t := range p.textures {
		t.Destroy()
	}
}

// draw a string, rendering and caching new glpyhs if necessary
func (p *printer) draw(r *sdl.Renderer, s string, x, y int32) {
	p.drawAlpha(r, s, x, y, 255)
}

// like draw, but applies an alpha modifier to textures
func (p *printer) drawAlpha(r *sdl.Renderer, s string, x, y int32, alpha uint8) {
	dst := &sdl.Rect{X: x, Y: y, W: p.rect.W, H: p.rect.H}
	for _, c := range s {
		if _, ok := p.textures[c]; !ok {
			if err := p.prerenderGlyph(r, c); err != nil {
				log.Print(err)
			}
		}
		if t, ok := p.textures[c]; ok {
			t.SetAlphaMod(alpha)
			r.Copy(t, p.rect, dst)
		}
		dst.X += p.rect.W
	}
}

// renders a texture for a glyph and adds it to the printer's map
func (p *printer) prerenderGlyph(r *sdl.Renderer, c rune) error {
	s, err := p.font.RenderGlyphBlended(c, colorFg)
	if err != nil {
		return err
	}
	defer s.Free()
	t, err := r.CreateTextureFromSurface(s)
	if err != nil {
		return err
	}
	p.textures[c] = t
	return nil
}

// returns the size of string if it were rendered
func (p *printer) size(s string) (int32, int32) {
	return int32(len(s)) * p.rect.W, p.rect.H
}
