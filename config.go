package main

import (
	"io/ioutil"
	"os"

	"gopkg.in/yaml.v2"
)

// Projects are the list of projects we will care about by default
type Projects []string

// Label is a name and a color
type Label struct {
	Name  string `yaml:"name,omitempty"`
	Color string `yaml:"color,omitempty"`
}

// Priority probably doesn't need to be its own type
type Priority Label

// Type probably doesn't need to be its own type
type Type Label

// Config is our main config struct
type Config struct {
	NextMilestone    string `yaml:"next-milestone,omitempty"`
	SomedayMilestone string `yaml:"someday-milestone,omitempty"`
	Projects         Projects
	Priorities       []Priority
	Types            []Type
}

// DefaultPriorities if none are specified in the config
var DefaultPriorities = []Priority{
	Priority{Name: "blocker", Color: "e11d21"},
	Priority{Name: "critical", Color: "eb6420"},
	Priority{Name: "normal", Color: "fbca04"},
	Priority{Name: "low", Color: "009800"},
}

// DefaultTypes if none are specified in the config
var DefaultTypes = []Type{
	Type{Name: "bug", Color: "f7c6c7"},
	Type{Name: "task", Color: "fef2c0"},
	Type{Name: "enhancement", Color: "bfe5bf"},
	Type{Name: "question", Color: "c7def8"},
}

// DefaultNextMilestone if none is specified in the config
var DefaultNextMilestone = "Next"

// DefaultSomedayMilestone if none is specified in the config
var DefaultSomedayMilestone = "Someday"

// LoadConfig is the entrypoint into the config
func LoadConfig(opts *Options) (*Config, error) {

	var config Config

	// TODO(termie): make an option
	f, err := os.Open("triage.yml")
	if err == nil {
		defer f.Close()

		data, err := ioutil.ReadAll(f)
		if err != nil {
			return nil, err
		}

		err = yaml.Unmarshal(data, &config)
		if err != nil {
			return nil, err
		}
	} else {
		config = Config{}
	}

	// set defaults
	if len(config.Priorities) < 1 {
		config.Priorities = DefaultPriorities
	}

	if len(config.Types) < 1 {
		config.Types = DefaultTypes
	}

	if config.NextMilestone == "" {
		config.NextMilestone = DefaultNextMilestone
	}

	if config.SomedayMilestone == "" {
		config.SomedayMilestone = DefaultSomedayMilestone
	}

	return &config, nil
}
