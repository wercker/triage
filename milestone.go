package main

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/codegangsta/cli"
	"github.com/google/go-github/github"
)

var (
	showMilestonesCommand = cli.Command{
		Name:  "show-milestones",
		Usage: "output the milestones detected for your defined projects",
		Action: func(c *cli.Context) {
			opts, err := NewOptions(c)
			if err != nil {
				logger.Errorln("Invalid options", err)
				os.Exit(1)
			}
			err = cmdShowMilestones(opts)
			if err != nil {
				SoftExit(opts, err)
			}
		},
	}
	setMilestonesCommand = cli.Command{
		Name:      "set-milestones",
		Usage:     "set milestones for a project based on our config",
		ArgsUsage: "[project]",
		Action: func(c *cli.Context) {
			opts, err := NewOptions(c)
			if err != nil {
				logger.Errorln("Invalid options", err)
				os.Exit(1)
			}
			project := c.Args().First()
			err = cmdSetMilestones(opts, project)
			if err != nil {
				SoftExit(opts, err)
			}
		},
	}
	createMilestoneCommand = cli.Command{
		Name:      "create-milestone",
		Usage:     "create milestone for a project based on our config",
		ArgsUsage: "[project]",
		Action: func(c *cli.Context) {
			opts, err := NewOptions(c)
			if err != nil {
				logger.Errorln("Invalid options", err)
				os.Exit(1)
			}
			project := c.Args().First()
			due := c.String("due")
			title := c.String("title")
			err = cmdCreateMilestone(opts, project, due, title)
			if err != nil {
				SoftExit(opts, err)
			}
		},
		Flags: []cli.Flag{
			cli.StringFlag{Name: "due", Usage: "due on YYYY-MM-DD"},
			cli.StringFlag{Name: "title", Usage: "title of the milestone"},
		},
	}
)

// Milestone is all we care about re: milestones
type Milestone struct {
	Number int
	Title  string
	DueOn  *time.Time
}

// Milestones implemenation of milestones-for-project for github api
func (a *GithubAPI) Milestones(project string) ([]*Milestone, error) {
	defer profile("GithubAPI.Milestones").Stop()
	owner, repo, err := ownerRepo(project)
	if err != nil {
		return nil, err
	}

	logger.Debugln("Fetching milestones for:", project)
	milestones, _, err := a.client.Issues.ListMilestones(
		owner,
		repo,
		&github.MilestoneListOptions{},
	)
	if err != nil {
		return nil, err
	}

	var current *Milestone
	var next *Milestone
	var someday *Milestone

	for _, milestone := range milestones {
		logger.Debugf("  found milestone: (%d) %s %v", *milestone.Number, *milestone.Title, milestone.DueOn)
		if current == nil && milestone.DueOn != nil {
			if (*milestone.DueOn).After(time.Now()) {
				current = &Milestone{
					Number: *milestone.Number,
					Title:  *milestone.Title,
					DueOn:  milestone.DueOn,
				}
				logger.Debugln("    using as Current")
			}
		} else if milestone.DueOn == nil {
			if *milestone.Title == a.config.NextMilestone {
				next = &Milestone{
					Number: *milestone.Number,
					Title:  *milestone.Title,
				}
				logger.Debugln("    using as Next")
			}
			if *milestone.Title == a.config.SomedayMilestone {
				someday = &Milestone{
					Number: *milestone.Number,
					Title:  *milestone.Title,
				}
				logger.Debugln("    using as Someday")
			}
		}
	}

	if current == nil || next == nil || someday == nil {
		return []*Milestone{current, next, someday}, fmt.Errorf("Did not find valid milestones for: %s (%v, %v, %v)", project, current, next, someday)
	}

	return []*Milestone{current, next, someday}, nil
}

// cmdShowMilestones prints the milestones we detected on your projects
func cmdShowMilestones(opts *Options) error {
	tc := AuthClient(opts)
	client := github.NewClient(tc)

	config, err := LoadConfig(opts)
	if err != nil {
		return err
	}

	api := NewGithubAPI(client, opts, config)

	out := map[string][]*Milestone{}
	for _, project := range config.Projects {
		milestones, err := api.Milestones(project)
		if err != nil {
			logger.Errorln(err)
		}
		out[project] = milestones

	}

	for project, milestones := range out {
		fmt.Printf("%s:\n", project)
		for i, milestone := range milestones {
			switch i {
			case 0:
				fmt.Printf("  current: ")
			case 1:
				fmt.Printf("     next: ")
			case 2:
				fmt.Printf("  someday: ")
			}
			if milestone != nil {
				fmt.Printf("(%d) %s", milestone.Number, milestone.Title)
				if milestone.DueOn != nil {
					fmt.Printf(" %v", milestone.DueOn)
				}
				fmt.Printf("\n")
			} else {
				fmt.Printf("nil\n")
			}
		}
	}
	return nil
}

// cmdSetMilestones sets our Next and Someday milestones in target projects
func cmdSetMilestones(opts *Options, target string) error {
	tc := AuthClient(opts)
	client := github.NewClient(tc)
	config, err := LoadConfig(opts)
	if err != nil {
		return err
	}

	ourMilestones := []string{config.NextMilestone, config.SomedayMilestone}

	var projects []string
	if target == "all" {
		projects = config.Projects
	} else {
		projects = strings.Split(target, " ")
	}

	for _, project := range projects {
		logger.Debugln("Setting milestones for:", project)

		owner, repo, err := ownerRepo(project)
		if err != nil {
			return err
		}

		theirMilestones, _, err := client.Issues.ListMilestones(owner, repo, &github.MilestoneListOptions{})
		if err != nil {
			return err
		}

		theirMilestonesList := []string{}
		for _, milestone := range theirMilestones {
			theirMilestonesList = append(theirMilestonesList, *milestone.Title)
		}

	OurMilestones:
		for _, ours := range ourMilestones {
			for _, theirs := range theirMilestonesList {
				if ours == theirs {
					logger.Debugln("  found existing:", ours)
					continue OurMilestones
				}
			}
			// if we got here we didn't match, create a milestone
			logger.Debugln("  creating:", ours)
			_, _, err := client.Issues.CreateMilestone(owner, repo, &github.Milestone{Title: &ours})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// cmdCreateMilestone creates a new milestone in all projects
func cmdCreateMilestone(opts *Options, target, due, title string) error {
	tc := AuthClient(opts)
	client := github.NewClient(tc)
	config, err := LoadConfig(opts)
	if err != nil {
		return err
	}

	var date time.Time
	if due != "" {
		date, err = time.Parse("2006-01-02", due)
		if err != nil {
			return err
		}
	} else {
		// Default to Monday next week with a little wiggle room
		check := time.Now().Add(5 * 24 * time.Hour)
		check = check.Round(1 * 24 * time.Hour)
		check = check.Add(1 * time.Second)
		for check.Weekday() != 0 {
			check = check.Add(1 * 24 * time.Hour)
		}
		date = check
	}

	if title == "" {
		year, week := date.ISOWeek()
		seed, err := strconv.Atoi(fmt.Sprintf("%d%02d", year, week))
		if err != nil {
			return err
		}
		rand.Seed(int64(seed))
		ship := Titles[rand.Intn(len(Titles))]
		title = fmt.Sprintf("%d-%02d %s", year, week, ship)
	}

	var projects []string
	if target == "all" {
		projects = config.Projects
	} else {
		projects = strings.Split(target, " ")
	}

	for _, project := range projects {
		logger.Debugln("Creating milestone for:", project)

		owner, repo, err := ownerRepo(project)
		if err != nil {
			return err
		}

		// if we got here we didn't match, create a milestone
		logger.Debugf("  creating: %s (%v)", title, due)
		_, _, err = client.Issues.CreateMilestone(owner, repo, &github.Milestone{Title: &title, DueOn: &date})
		if err != nil {
			return err
		}
	}
	fmt.Printf("New Milestone: %s\n", title)
	return nil
}
