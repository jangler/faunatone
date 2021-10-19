package main

import (
	"time"

	"github.com/veandco/go-sdl2/sdl"
)

// type that draws a series of string function results in a line
type statusBar struct {
	rect        *sdl.Rect
	funcs       []func() string
	msg         string
	msgTime     time.Time
	msgChan     chan string
	msgDuration time.Duration
}

// initialize a new status bar
func newStatusBar(msgSeconds int, funcs ...func() string) *statusBar {
	return &statusBar{
		rect:        &sdl.Rect{},
		funcs:       funcs,
		msgChan:     make(chan string),
		msgDuration: time.Second * time.Duration(msgSeconds),
	}
}

// draw the status bar
func (sb *statusBar) draw(pr *printer, r *sdl.Renderer) {
	x := int32(padding)
	y := r.GetViewport().H - pr.rect.H - padding
	r.SetDrawColorArray(colorBg2Array...)
	*sb.rect = sdl.Rect{x - padding, y - padding, r.GetViewport().W, pr.rect.H + padding*2}
	r.FillRect(sb.rect)
	for _, f := range sb.funcs {
		s := f()
		if s != "" {
			pr.draw(r, s, x, y)
			x += padding*2 + pr.rect.W*int32(len(s))
		}
	}

	// update message
	select {
	case sb.msg = <-sb.msgChan:
		sb.msgTime = time.Now()
	default:
	}

	// draw
	if time.Now().Sub(sb.msgTime) < sb.msgDuration {
		pr.draw(r, sb.msg, r.GetViewport().W-padding-pr.rect.W*int32(len(sb.msg)), y)
	}
}

// update status bar message and sends redraw flag updates as necessary
func (sb *statusBar) showMessage(s string, redraw chan bool) {
	go func() {
		sb.msgChan <- s
		if redraw != nil {
			redraw <- true
		}
		time.Sleep(sb.msgDuration)
		if redraw != nil {
			redraw <- true
		}
	}()
}
