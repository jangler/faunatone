package main

import (
	"strings"

	"github.com/veandco/go-sdl2/sdl"
	"github.com/veandco/go-sdl2/ttf"
)

// pixels of space left around menu labels
const menuPadding = int32(7)

// top-level drop-down menu bar
type menuBar []*menu

// initialize the menu bar's resources and layout and those of its children
func (mb menuBar) init(f *ttf.Font, r *sdl.Renderer) error {
	x := int32(0)
	err := error(nil)
	for _, m := range mb {
		x, err = m.init(f, r, x)
		if err != nil {
			return err
		}
	}
	return nil
}

// free the menu bar's resources and those of its children
func (mb menuBar) destroy() {
	for _, m := range mb {
		m.destroy()
	}
}

// draw the menu bar and its children
func (mb menuBar) draw(r *sdl.Renderer) {
	if len(mb) > 0 {
		r.SetDrawColorArray(colorHighlightArray...)
		r.FillRect(&sdl.Rect{0, 0, r.GetViewport().W, mb[0].rect.H})
		for _, m := range mb {
			m.draw(r)
		}
	}
}

// respond to mouse motion events
func (mb menuBar) mouseMotion(e *sdl.MouseMotionEvent) {
	// if a menu is being shown and we mouse over a new menu root, show that
	// menu and hide all others
	shown := false
	for _, m := range mb {
		shown = shown || m.shown
	}
	if shown {
		p := sdl.Point{e.X, e.Y}
		for _, m := range mb {
			if p.InRect(m.rect) {
				for _, m := range mb {
					m.shown = false
				}
				m.shown = true
				break
			}
		}
	}
}

// respond to mouse button events
func (mb menuBar) mouseButton(e *sdl.MouseButtonEvent) {
	// only respond to mouse down
	if e.Type != sdl.MOUSEBUTTONDOWN {
		return
	}

	// if we clicked on a menu root, toggle display of that menu
	p := sdl.Point{e.X, e.Y}
	for _, m := range mb {
		if p.InRect(m.rect) {
			m.shown = !m.shown
			return
		}
	}

	// if we clicked on a menu item, run its action
	for _, m := range mb {
		if m.shown {
			for _, mi := range m.items {
				if p.InRect(mi.rect) && mi.action != nil {
					mi.action()
				}
			}
		}
	}

	// finally, hide all menus
	for _, m := range mb {
		m.shown = false
	}
}

// a top-level menu category
type menu struct {
	label     string
	items     []*menuItem
	surface   *sdl.Surface
	texture   *sdl.Texture
	rect      *sdl.Rect
	itemsRect *sdl.Rect
	shown     bool
}

// initialize the menu's resources and layout and those of its children;
// returns (x+w, err)
func (m *menu) init(f *ttf.Font, r *sdl.Renderer, x int32) (int32, error) {
	err := error(nil)
	m.surface, m.texture, err = initTextGfx(f, r, m.label)
	m.rect = &sdl.Rect{x, 0, m.surface.W + menuPadding*2, m.surface.H + menuPadding*2}
	m.itemsRect = &sdl.Rect{x, m.rect.H, 0, 0}
	x2 := int32(0)
	y := m.rect.Y + m.rect.H + menuPadding
	for _, mi := range m.items {
		x2, y, err = mi.init(f, r, x, y)
		if err != nil {
			return m.rect.X + m.rect.W, err
		}
		if x2-x > m.itemsRect.W {
			m.itemsRect.W = x2 - x
		}
		m.itemsRect.H = y - m.itemsRect.Y
	}
	return m.rect.X + m.rect.W, err
}

// draw the menu and its children
func (m *menu) draw(r *sdl.Renderer) {
	drawSurfaceTexture(r, m.texture, m.surface, m.rect.X+menuPadding, m.rect.Y+menuPadding)
	if m.shown {
		r.FillRect(m.itemsRect)
		for _, mi := range m.items {
			mi.draw(r)
		}
	}
}

// free the menu's resources and those of its children
func (m *menu) destroy() {
	m.texture.Destroy()
	m.surface.Free()
	for _, mi := range m.items {
		mi.destroy()
	}
}

// an item in a drop-down menu
type menuItem struct {
	label     string
	shortcuts []string
	action    func()
	surface   *sdl.Surface
	texture   *sdl.Texture
	rect      *sdl.Rect
}

// initialize the menu item's resources and layout; returns (x+w, y+h, err)
func (mi *menuItem) init(f *ttf.Font, r *sdl.Renderer, x, y int32) (int32, int32, error) {
	s := mi.label
	if len(mi.shortcuts) > 0 {
		s += " (" + strings.Join(mi.shortcuts, ", ") + ")"
	}
	err := error(nil)
	mi.surface, mi.texture, err = initTextGfx(f, r, s)
	mi.rect = &sdl.Rect{x, y, mi.surface.W + menuPadding*2, mi.surface.H + menuPadding}
	return mi.rect.X + mi.rect.W, mi.rect.Y + mi.rect.H, err
}

// free the menu item's resources
func (mi *menuItem) destroy() {
	mi.texture.Destroy()
	mi.surface.Free()
}

// draw the menu item
func (mi *menuItem) draw(r *sdl.Renderer) {
	drawSurfaceTexture(r, mi.texture, mi.surface, mi.rect.X+menuPadding, mi.rect.Y)
}

// create a surface and texture from a font, renderer, and string
func initTextGfx(f *ttf.Font, r *sdl.Renderer, s string) (*sdl.Surface, *sdl.Texture, error) {
	surface, err := f.RenderUTF8Blended(s, colorFg)
	if err != nil {
		return surface, nil, err
	}
	texture, err := r.CreateTextureFromSurface(surface)
	return surface, texture, err
}

// draw t at (x, y) using s to determine size
func drawSurfaceTexture(r *sdl.Renderer, t *sdl.Texture, s *sdl.Surface, x, y int32) {
	r.Copy(t, &sdl.Rect{0, 0, s.W, s.H}, &sdl.Rect{x, y, s.W, s.H})
}
