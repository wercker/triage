package main

import (
	"github.com/google/go-github/github"
	"github.com/nsf/termbox-go"
)

// Window is the basic interface for things that draw on the screen
type Window interface {
	Init() error
	Draw(x, y, x1, y1 int)
	HandleEvent(termbox.Event) (bool, error)
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
	termbox.SetOutputMode(termbox.Output256)
	return nil
}

// AddWindow in case we ever have more than one?
func (c *Console) AddWindow(w Window) {
	c.Windows = append(c.Windows, w)
	// w.SetBounds(0, 0, width, height)
	if c.CurrentWindow == nil {
		c.CurrentWindow = w
	}
}
