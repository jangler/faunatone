package main

import (
	"fmt"
	"time"

	"github.com/veandco/go-sdl2/sdl"
)

// queue for status messages
var statusChan = make(chan string, 10)

// add a message to the status message queue if possible
func statusf(format string, args ...interface{}) {
	s := fmt.Sprintf(format, args...)
	select {
	case statusChan <- s:
	default:
	}
}

// type that draws a series of string function results in a line
type statusBar struct {
	rect        *sdl.Rect
	funcs       []func() string
	msg         string
	msgTime     time.Time
	msgDuration time.Duration
}

// initialize a new status bar
func newStatusBar(msgSeconds int, funcs ...func() string) *statusBar {
	return &statusBar{
		rect:        &sdl.Rect{},
		funcs:       funcs,
		msgDuration: time.Second * time.Duration(msgSeconds),
	}
}

// draw the status bar
func (sb *statusBar) draw(pr *printer, r *sdl.Renderer, redraw chan bool) {
	x := int32(padding)
	y := r.GetViewport().H - pr.rect.H - padding
	r.SetDrawColorArray(colorBg2Array...)
	*sb.rect = sdl.Rect{X: x - padding, Y: y - padding, W: r.GetViewport().W, H: pr.rect.H + padding*2}
	r.FillRect(sb.rect)
	for _, f := range sb.funcs {
		s := f()
		if s != "" {
			pr.draw(r, s, x, y)
			x += padding*2 + pr.rect.W*int32(len(s))
		}
	}

	if time.Since(sb.msgTime) < sb.msgDuration {
		// draw message
		pr.draw(r, sb.msg, r.GetViewport().W-padding-pr.rect.W*int32(len(sb.msg)), y)
	} else {
		// check for message updates
		select {
		case sb.msg = <-statusChan:
			sb.msgTime = time.Now()
			go func() {
				time.Sleep(sb.msgDuration)
				redraw <- true
			}()
		default:
		}
	}
}
