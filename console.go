package main

import (
	"github.com/google/go-github/github"
	"github.com/nsf/termbox-go"
)

type Window interface {
	Init() error
	Draw()
	SetBounds(x1, y1, x2, y2 int)
	HandleEvent(termbox.Event)
}

type Console struct {
	Windows       []Window
	CurrentWindow Window
}

func NewConsole(client *github.Client) *Console {
	return &Console{}
}

func (c *Console) Init() error {
	return nil
}

// Draw the curent window
func (c *Console) Draw() error {
	termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)
	c.CurrentWindow.Draw()
	termbox.Flush()
	return nil
}

func (c *Console) AddWindow(w Window) {
	c.Windows = append(c.Windows, w)
	width, height := termbox.Size()
	w.SetBounds(0, 0, width, height)
	if c.CurrentWindow == nil {
		c.CurrentWindow = w
	}
}
