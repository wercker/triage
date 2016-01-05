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
	logger      = logrus.New()
	githubFlags = []cli.Flag{
		cli.StringFlag{Name: "api-token", Value: "", Usage: "github api token", EnvVar: "GITHUB_API_TOKEN"},
	}
	curseCommand = cli.Command{
		Name:  "curse",
		Usage: "curse info",
		Action: func(c *cli.Context) {
			opts, err := NewOptions(c)
			if err != nil {
				logger.Errorln("Invalid options", err)
				os.Exit(1)
			}
			err = cmdCurse(opts)
			if err != nil {
				panic(err)
			}
		},
		Flags: githubFlags,
	}
)

type Options struct {
	APIToken string
}

func NewOptions(c *cli.Context) (*Options, error) {
	return &Options{
		APIToken: c.String("api-token"),
	}, nil
}

func AuthClient(opts *Options) *http.Client {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: opts.APIToken},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)
	return tc
}

func cmdCurse(opts *Options) error {
	if err := termbox.Init(); err != nil {
		return err
	}
	defer termbox.Close()

	tc := AuthClient(opts)
	client := github.NewClient(tc)

	c := NewConsole(client)
	if err := c.Init(); err != nil {
		return err
	}

	// repoWindow := NewRepoWindow(client)
	// if err := repoWindow.Init(); err != nil {
	//   return err
	// }
	// c.AddWindow(repoWindow)

	issueWindow := NewIssueWindow(client)
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
	app.Author = "Team wercker"
	app.Name = "triage"
	app.Usage = ""
	app.Commands = []cli.Command{
		curseCommand,
	}
	app.Run(os.Args)
}
