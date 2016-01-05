package main

import (
	"fmt"

	"github.com/google/go-github/github"
	"github.com/nsf/termbox-go"
)

type Window interface {
	ID() string
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

	c.DrawHeader()

	c.CurrentWindow.Draw()
	termbox.Flush()
	return nil
}

func (c *Console) DrawHeader() error {
	currentX := 1
	currentY := 1
	for _, window := range c.Windows {
		var item string
		if window.ID() == c.CurrentWindow.ID() {
			item = fmt.Sprintf("_%s_", window.ID())
		} else {
			item = fmt.Sprintf(" %s ", window.ID())
		}
		printLine(item, currentX, currentY)
		currentX += len(item)
	}
	return nil
}

func (c *Console) AddWindow(w Window) {
	c.Windows = append(c.Windows, w)
	width, height := termbox.Size()
	w.SetBounds(3, 2, width, height)
	if c.CurrentWindow == nil {
		c.CurrentWindow = w
	}
}

func (c *Console) SelectWindow(id string) {
	for _, window := range c.Windows {
		if window.ID() == id {
			c.CurrentWindow = window
		}
	}
}
