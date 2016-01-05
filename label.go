package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"

	"github.com/codegangsta/cli"
	"github.com/google/go-github/github"
)

var (
	showLabelsCommand = cli.Command{
		Name:  "show-labels",
		Usage: "output labels for a project suitable for use in config",
		Action: func(c *cli.Context) {
			opts, err := NewOptions(c)
			if err != nil {
				logger.Errorln("Invalid options", err)
				os.Exit(1)
			}
			project := c.Args().First()
			err = cmdShowLabels(opts, project)
			if err != nil {
				panic(err)
			}
		},
		Flags: githubFlags,
	}
	setLabelsCommand = cli.Command{
		Name:  "set-labels",
		Usage: "set labels for a project based on our config",
		Action: func(c *cli.Context) {
			opts, err := NewOptions(c)
			if err != nil {
				logger.Errorln("Invalid options", err)
				os.Exit(1)
			}
			project := c.Args().First()
			err = cmdSetLabels(opts, project)
			if err != nil {
				panic(err)
			}
		},
		Flags: githubFlags,
	}
)

// cmdShowLabels prints the labels for a project for easy inclusion
// in the config
func cmdShowLabels(opts *Options, project string) error {
	tc := AuthClient(opts)
	client := github.NewClient(tc)

	owner, repo, err := ownerRepo(project)
	if err != nil {
		return err
	}

	labels, _, err := client.Issues.ListLabels(owner, repo, &github.ListOptions{})
	if err != nil {
		return err
	}

	out := []Label{}
	for _, label := range labels {
		out = append(out, Label{*label.Name, *label.Color})
	}

	d, err := yaml.Marshal(out)
	if err != nil {
		return err
	}
	fmt.Printf("%s", d)
	return nil
}

func cmdSetLabels(opts *Options, project string) error {
	tc := AuthClient(opts)
	client := github.NewClient(tc)
	config, err := LoadConfig(opts)
	if err != nil {
		return err
	}

	ourLabels := config.Priorities
	ourLabels = append(ourLabels, config.Types...)
	ourLabelsMap := map[string]string{}
	for _, label := range ourLabels {
		ourLabelsMap[label.Name] = label.Color
	}

	owner, repo, err := ownerRepo(project)
	if err != nil {
		return err
	}

	theirLabels, _, err := client.Issues.ListLabels(owner, repo, &github.ListOptions{})
	if err != nil {
		return err
	}
	theirLabelsMap := map[string]github.Label{}
	for _, label := range theirLabels {
		theirLabelsMap[*label.Name] = label
	}

	for _, ours := range ourLabels {
		theirs, ok := theirLabelsMap[ours.Name]
		// check if we already exist but don't have the same color
		if ok && *theirs.Color != ours.Color {
			// update label
			*theirs.Color = ours.Color
			_, _, err = client.Issues.EditLabel(owner, repo, ours.Name, &theirs)
			if err != nil {
				return err
			}
		} else if !ok {
			_, _, err = client.Issues.CreateLabel(
				owner,
				repo,
				&github.Label{Name: &ours.Name, Color: &ours.Color},
			)
			if err != nil {
				return err
			}
		}
	}
	return nil
}