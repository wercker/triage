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

type Config struct {
	NextMilestone    string `yaml:"next-milestone,omitempty"`
	SomedayMilestone string `yaml:"someday-milestone,omitempty"`
	Projects         Projects
	Priorities       []Label
	Types            []Label
}

func LoadConfig(opts *Options) (*Config, error) {
	// TODO(termie): make an option
	f, err := os.Open("triage.yml")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	data, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}
