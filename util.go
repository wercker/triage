package main

import (
	"os"
	"strings"
	"time"

	"github.com/mitchellh/go-wordwrap"
)

// exists is like python's os.path.exists and too many lines in Go
func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func wordWrap(text string, length int) []string {
	s := wordwrap.WrapString(text, uint(length))
	return strings.Split(s, "\n")
}

// Profiler gets us timing numbers
type Profiler struct {
	name  string
	start time.Time
}

// Stop the profiler and log the timing
func (p *Profiler) Stop() {
	now := time.Now()
	d := now.Sub(p.start)
	secs := float64(d.Nanoseconds()) / (1000 * 1000 * 1000)
	logger.Debugf("[prof] %s in %.3fs", p.name, secs)
}

func profile(s string) *Profiler {
	return &Profiler{name: s, start: time.Now()}
}
