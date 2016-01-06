package main

import (
	"github.com/google/go-github/github"
	"github.com/nsf/termbox-go"
)

// Window interface for basic window functionality
type Window interface {
	Init() error
	Draw()
	SetBounds(x1, y1, x2, y2 int)
	HandleEvent(termbox.Event)
}

// Console implements the outer level of console interaction
type Console struct {
	Windows       []Window
	CurrentWindow Window
}

// NewConsole constructor
func NewConsole(client *github.Client) *Console {
	return &Console{}
}

// Init sets up the outer console
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

// AddWindow in case we ever have more than one?
func (c *Console) AddWindow(w Window) {
	c.Windows = append(c.Windows, w)
	width, height := termbox.Size()
	w.SetBounds(0, 0, width, height)
	if c.CurrentWindow == nil {
		c.CurrentWindow = w
	}
}
