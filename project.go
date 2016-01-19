package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"

	"github.com/codegangsta/cli"
	"github.com/google/go-github/github"
)

var (
	showProjectsCommand = cli.Command{
		Name:      "show-projects",
		Usage:     "output projects for user suitable for use in config",
		ArgsUsage: "[target]",
		Action: func(c *cli.Context) {
			opts, err := NewOptions(c)
			if err != nil {
				logger.Errorln("Invalid options", err)
				os.Exit(1)
			}
			target := c.Args().First()
			err = cmdShowProjects(opts, target)
			if err != nil {
				SoftExit(opts, err)
			}
		},
	}
)

func cmdShowProjects(opts *Options, target string) error {
	tc := AuthClient(opts)
	client := github.NewClient(tc)

	var repos []github.Repository
	var err error

	// if we specified an org we need to do a different search
	if target != "" {
		repos, _, err = client.Repositories.ListByOrg(
			target,
			&github.RepositoryListByOrgOptions{
				Type:        "all",
				ListOptions: github.ListOptions{PerPage: 1000},
			},
		)

	} else {
		repos, _, err = client.Repositories.List(
			"",
			&github.RepositoryListOptions{
				Type:        "all",
				ListOptions: github.ListOptions{PerPage: 1000},
			},
		)
		if err != nil {
			return err
		}
	}

	out := []string{}
	for _, repo := range repos {
		owner := *repo.Owner
		out = append(out, fmt.Sprintf("%s/%s", *owner.Login, *repo.Name))
	}

	d, err := yaml.Marshal(out)
	if err != nil {
		return err
	}
	fmt.Printf("%s", d)
	return nil
}
