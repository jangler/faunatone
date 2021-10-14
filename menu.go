package main

import (
	"log"
	"path/filepath"
	"strings"

	"github.com/veandco/go-sdl2/sdl"
)

var shortcutsPath = filepath.Join(configPath, "shortcuts.tsv")

// top-level drop-down menu bar
type menuBar struct {
	menus     []*menu
	shortcuts map[string]*menuItem
}

// initialize the menu bar's properties and layout and those of its children
func (mb *menuBar) init(p *printer) {
	// connect shortcuts
	if mb.shortcuts == nil {
		mb.shortcuts = make(map[string]*menuItem)
	}
	if records, err := readTSV(shortcutsPath); err == nil {
		for _, rec := range records {
			ok := false
			if len(rec) == 3 {
				key, menuLabel, itemLabel := rec[0], rec[1], rec[2]
			outer:
				for _, m := range mb.menus {
					if m.label == menuLabel {
						for _, mi := range m.items {
							if mi.label == itemLabel {
								ok = true
								mi.shortcuts = append(mi.shortcuts, key)
								mb.shortcuts[key] = mi
								break outer
							}
						}
					}
				}
			}
			if !ok {
				log.Printf("bad shortcut config record: %q", rec)
			}
		}
	} else {
		log.Print(err)
	}

	// init menu layouts
	x := int32(0)
	for _, m := range mb.menus {
		x = m.init(p, x)
	}
}

// draw the menu bar and its children
func (mb *menuBar) draw(p *printer, r *sdl.Renderer) {
	if len(mb.menus) > 0 {
		r.SetDrawColorArray(colorHighlightArray...)
		r.FillRect(&sdl.Rect{0, 0, r.GetViewport().W, mb.menus[0].rect.H})
		for _, m := range mb.menus {
			m.draw(p, r)
		}
	}
}

// respond to keyboard events, returning true if an action was triggered
func (mb *menuBar) keyboardEvent(e *sdl.KeyboardEvent) bool {
	if e.Repeat != 0 || e.State != sdl.PRESSED {
		return false
	}
	if item, ok := mb.shortcuts[formatKeyEvent(e)]; ok && item.action != nil {
		item.action()
		return true
	}
	return false
}

// convert a keyboard event into a shortcut string
func formatKeyEvent(e *sdl.KeyboardEvent) string {
	keys := []string{}
	if e.Keysym.Mod&sdl.KMOD_CTRL != 0 {
		keys = append(keys, "Ctrl")
	}
	if e.Keysym.Mod&sdl.KMOD_ALT != 0 {
		keys = append(keys, "Alt")
	}
	if e.Keysym.Mod&sdl.KMOD_SHIFT != 0 {
		keys = append(keys, "Shift")
	}
	keys = append(keys, sdl.GetKeyName(e.Keysym.Sym))
	return strings.Join(keys, "+")
}

// respond to mouse motion events
func (mb *menuBar) mouseMotion(e *sdl.MouseMotionEvent) {
	// if a menu is being shown and we mouse over a new menu root, show that
	// menu and hide all others
	shown := false
	for _, m := range mb.menus {
		shown = shown || m.shown
	}
	if shown {
		p := sdl.Point{e.X, e.Y}
		for _, m := range mb.menus {
			if p.InRect(m.rect) {
				for _, m := range mb.menus {
					m.shown = false
				}
				m.shown = true
				break
			}
		}
	}
}

// respond to mouse button events
func (mb *menuBar) mouseButton(e *sdl.MouseButtonEvent) {
	// only respond to mouse down
	if e.Type != sdl.MOUSEBUTTONDOWN {
		return
	}

	// if we clicked on a menu root, toggle display of that menu
	p := sdl.Point{e.X, e.Y}
	for _, m := range mb.menus {
		if p.InRect(m.rect) {
			m.shown = !m.shown
			return
		}
	}

	// if we clicked on a menu item, run its action
	for _, m := range mb.menus {
		if m.shown {
			for _, mi := range m.items {
				if p.InRect(mi.rect) && mi.action != nil {
					mi.action()
				}
			}
		}
	}

	// finally, hide all menus
	for _, m := range mb.menus {
		m.shown = false
	}
}

// a top-level menu category
type menu struct {
	label     string
	items     []*menuItem
	rect      *sdl.Rect
	itemsRect *sdl.Rect // for drawing background under shwon items
	shown     bool
}

// initialize the menu's properties and layout and those of its children;
// returns x+w
func (m *menu) init(p *printer, x int32) int32 {
	w, h := p.size(m.label)
	m.rect = &sdl.Rect{x, 0, w + padding*2, h + padding*2}
	m.itemsRect = &sdl.Rect{x, m.rect.H, 0, 0}
	x2 := int32(0)
	y := m.rect.Y + m.rect.H + padding
	for _, mi := range m.items {
		x2, y = mi.init(p, x, y)
		if x2-x > m.itemsRect.W {
			m.itemsRect.W = x2 - x
		}
		m.itemsRect.H = y - m.itemsRect.Y
	}
	return m.rect.X + m.rect.W
}

// draw the menu and its children
func (m *menu) draw(p *printer, r *sdl.Renderer) {
	p.draw(r, m.label, m.rect.X+padding, m.rect.Y+padding)
	if m.shown {
		r.FillRect(m.itemsRect)
		for _, mi := range m.items {
			mi.draw(p, r)
		}
	}
}

// an item in a drop-down menu
type menuItem struct {
	label     string
	shortcuts []string
	text      string // final text to be drawn
	action    func()
	rect      *sdl.Rect
}

// initialize the menu item's properties and layout; returns (x+w, y+h)
func (mi *menuItem) init(p *printer, x, y int32) (int32, int32) {
	mi.text = mi.label
	if len(mi.shortcuts) > 0 {
		mi.text += " (" + strings.Join(mi.shortcuts, ", ") + ")"
	}
	w, h := p.size(mi.text)
	mi.rect = &sdl.Rect{x, y, w + padding*2, h + padding}
	return mi.rect.X + mi.rect.W, mi.rect.Y + mi.rect.H
}

// draw the menu item
func (mi *menuItem) draw(p *printer, r *sdl.Renderer) {
	p.draw(r, mi.text, mi.rect.X+padding, mi.rect.Y)
}
