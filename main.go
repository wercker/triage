package main

import (
	"net/http"
	"os"

	"golang.org/x/oauth2"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/google/go-github/github"
	"github.com/nsf/termbox-go"
)

var (
	logger = logrus.New()
	uiCommand = cli.Command{
		Name:  "ui",
		Usage: "go into termbox mode",
		Action: func(c *cli.Context) {
			opts, err := NewOptions(c)
			if err != nil {
				logger.Errorln("Invalid options", err)
				os.Exit(1)
			}
			target := c.Args().First()
			err = cmdUI(opts, target)
			if err != nil {
				panic(err)
			}
		},
	}
)

// Options are our global options
type Options struct {
	APIToken string
	Debug    bool
}

// NewOptions constructor
func NewOptions(c *cli.Context) (*Options, error) {
	debug := c.GlobalBool("debug")
	if debug {
		logger.Level = logrus.DebugLevel
	}

	return &Options{
		APIToken: c.GlobalString("api-token"),
		Debug:    debug,
	}, nil
}

// AuthClient for github
func AuthClient(opts *Options) *http.Client {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: opts.APIToken},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)
	return tc
}

func cmdUI(opts *Options, target string) error {
	if err := termbox.Init(); err != nil {
		return err
	}
	defer termbox.Close()

	tc := AuthClient(opts)
	client := github.NewClient(tc)
	config, err := LoadConfig(opts)
	if err != nil {
		return err
	}
	api := NewGithubAPI(client, opts, config)
	c := NewConsole(client)
	if err := c.Init(); err != nil {
		return err
	}

	issueWindow := NewIssueWindow(client, opts, config, api, target)
	if err := issueWindow.Init(); err != nil {
		return err
	}
	c.AddWindow(issueWindow)

	if err := c.Draw(); err != nil {
		return err
	}

TermLoop:
	for {
		switch ev := termbox.PollEvent(); ev.Type {
		case termbox.EventKey:
			switch ev.Key {
			case termbox.KeyCtrlC:
				break TermLoop
			default:
				c.CurrentWindow.HandleEvent(ev)
				if err := c.Draw(); err != nil {
					return err
				}
			}
		case termbox.EventResize:
			if err := c.Draw(); err != nil {
				return err
			}
		}
	}

	return nil
}

func printLine(str string, x int, y int) {
	for i := range str {
		termbox.SetCell(x+i, y, rune(str[i]), termbox.ColorDefault, termbox.ColorDefault)
	}
}

func drawAll(c *Console) {
	termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)

	c.CurrentWindow.Draw()

	termbox.Flush()
}

func main() {
	app := cli.NewApp()
	app.Author = "termie"
	app.Name = "triage"
	app.Usage = ""
	app.Commands = []cli.Command{
		uiCommand,
		showLabelsCommand,
		setLabelsCommand,
		showProjectsCommand,
		showMilestonesCommand,
		setMilestonesCommand,
		createMilestoneCommand,
		versionCommand,
	}
	app.Flags = []cli.Flag{
		cli.BoolFlag{Name: "debug", Usage: "output debug info"},
		cli.StringFlag{Name: "api-token", Value: "", Usage: "github api token", EnvVar: "GITHUB_API_TOKEN"},
	}
	app.Run(os.Args)
}
