package main

import (
	"os"
	"time"
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

type Profiler struct {
	name  string
	start time.Time
}

func (p *Profiler) Stop() {
	now := time.Now()
	d := now.Sub(p.start)
	secs := float64(d.Nanoseconds()) / (1000 * 1000 * 1000)
	logger.Debugf("[prof] %s in %.3fs", p.name, secs)
}

func profile(s string) *Profiler {
	return &Profiler{name: s, start: time.Now()}
}
