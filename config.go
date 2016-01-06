package main

import (
	"io/ioutil"
	"os"

	"gopkg.in/yaml.v2"
)

type Projects []string

type Label struct {
	Name  string `yaml:"name,omitempty"`
	Color string `yaml:"color,omitempty"`
}

type Priority Label
type Type Label

type Config struct {
	NextMilestone    string `yaml:"next-milestone,omitempty"`
	SomedayMilestone string `yaml:"someday-milestone,omitempty"`
	Projects         Projects
	Priorities       []Priority
	Types            []Type
}

var DefaultPriorities = []Priority{
	Priority{Name: "blocker", Color: "e11d21"},
	Priority{Name: "critical", Color: "eb6420"},
	Priority{Name: "normal", Color: "fbca04"},
	Priority{Name: "low", Color: "009800"},
}

var DefaultTypes = []Type{
	Type{Name: "bug", Color: "f7c6c7"},
	Type{Name: "task", Color: "fef2c0"},
	Type{Name: "enhancement", Color: "bfe5bf"},
	Type{Name: "question", Color: "c7def8"},
}

var DefaultNextMilestone = "Next"
var DefaultSomedayMilestone = "Someday"

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
