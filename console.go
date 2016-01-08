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

type IWindow interface {
	Init() error
	Draw(x, y, x1, y1 int)
	// SetBounds(x1, y1, x2, y2 int)
	HandleEvent(termbox.Event) (bool, error)
}

// Console implements the outer level of console interaction
type Console struct {
	Windows       []IWindow
	CurrentWindow IWindow
}

// NewConsole constructor
func NewConsole(client *github.Client) *Console {
	return &Console{}
}

// Init sets up the outer console
func (c *Console) Init() error {
	termbox.SetOutputMode(termbox.Output256)
	return nil
}

// Draw the curent window
func (c *Console) Draw() error {
	termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)

	width, height := termbox.Size()
	c.CurrentWindow.Draw(0, 0, width, height)
	termbox.Flush()
	return nil
}

// AddWindow in case we ever have more than one?
func (c *Console) AddWindow(w IWindow) {
	c.Windows = append(c.Windows, w)
	// w.SetBounds(0, 0, width, height)
	if c.CurrentWindow == nil {
		c.CurrentWindow = w
	}
}
