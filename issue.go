package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/google/go-github/github"
	"github.com/mitchellh/go-wordwrap"
	"github.com/nsf/termbox-go"
)

// Issue is the data we care about from the github.Issue, plus some of our own
type Issue struct {
	Milestone *IssueMilestone
	Priority  *IssuePriority
	Type      *IssueType
	Number    int
	Title     string
	Body      string
	URL       string
	Owner     string
	Repo      string
	Project   string
	Labels    []string
}

// IssueMilestone sortable milestone
type IssueMilestone struct {
	Index int
	*Milestone
}

// IssuePriority sortable priority
type IssuePriority struct {
	Index int
	*Priority
}

// IssueType sortable type
type IssueType struct {
	Index int
	*Type
}

// NewIssue constructor for an Issue from a github.Issue
func NewIssue(issue github.Issue, ms map[string][]*Milestone, ps []Priority, ts []Type) *Issue {
	number := *issue.Number
	title := *issue.Title
	body := *issue.Body
	url := *issue.HTMLURL
	owner, repo, _ := ownerRepoFromURL(url)
	project := fmt.Sprintf("%s/%s", owner, repo)

	var issueMilestone IssueMilestone
	var issuePriority IssuePriority
	var issueType IssueType

	// figure out the milestone based on milestone number
	issueMilestone = IssueMilestone{Index: 0}
	if issue.Milestone != nil {
		mNumber := *issue.Milestone.Number
		if ourMs := ms[project]; ourMs != nil {
			for i, m := range ourMs {
				if m.Number == mNumber {
					issueMilestone = IssueMilestone{Index: i + 1, Milestone: m}
				}
			}
		}
	}

	// figure out the priority based on label name
	issuePriority = IssuePriority{Index: 0}
	for i, p := range ps {
		for _, l := range issue.Labels {
			if p.Name == *l.Name {
				issuePriority = IssuePriority{Index: i + 1, Priority: &p}
				break
			}
		}
	}

	// figure out the type based on label name
	issueType = IssueType{Index: 0}
	for i, t := range ts {
		for _, l := range issue.Labels {
			if t.Name == *l.Name {
				issueType = IssueType{Index: i + 1, Type: &t}
				break
			}
		}
	}

	// set the labels
	labels := []string{}
	for _, label := range issue.Labels {
		labels = append(labels, *label.Name)
	}

	return &Issue{
		Milestone: &issueMilestone,
		Priority:  &issuePriority,
		Type:      &issueType,
		Number:    number,
		Title:     title,
		Body:      body,
		URL:       url,
		Owner:     owner,
		Repo:      repo,
		Project:   project,
		Labels:    labels,
	}
}

// Search uses the Github search API
func (a *GithubAPI) Search(query string) ([]github.Issue, error) {
	result, _, err := a.client.Search.Issues(query,
		&github.SearchOptions{
			Order:       "updated",
			ListOptions: github.ListOptions{PerPage: 1000},
		})
	if err != nil {
		return nil, err
	}
	return result.Issues, nil

}

// IssueWindow is the main window for the issue management
type IssueWindow struct {
	client *github.Client
	opts   *Options
	config *Config
	api    API
	target string
	issues []*Issue
	// selected      map[string]struct{}
	// selectedRepos []github.Issues
	currentIndex    int
	lastIndex       int
	currentIssues   []*Issue
	currentFilter   string
	currentMenu     string
	scrollIndex     int
	enableSorting   bool
	enableExpanding bool
	// Milestones are weird
	milestones map[string][]*Milestone
	priorities []Priority
	types      []Type

	// currentMode   string
	x  int
	y  int
	x2 int
	y2 int
}

// NewIssueWindow constructor
func NewIssueWindow(client *github.Client, opts *Options, config *Config, api API, target string) *IssueWindow {
	return &IssueWindow{
		client:          client,
		opts:            opts,
		config:          config,
		api:             api,
		currentIndex:    -1,
		enableSorting:   true,
		enableExpanding: false,
		target:          target,
	}
}

// Init setup initial window state
func (w *IssueWindow) Init() error {
	// build our search string
	target := "is:open is:issue"
	if w.target == "" {
		if len(w.config.Projects) > 0 {
			for _, project := range w.config.Projects {
				target += fmt.Sprintf(" repo:%s", project)
			}
		}
	} else {
		target += fmt.Sprintf(" %s", w.target)
	}
	w.target = target

	// get our milestones
	milestones := map[string][]*Milestone{}
	for _, project := range w.config.Projects {
		resp, err := w.api.Milestones(project)
		if err == nil {
			// NOTE(termie): ignoring this error in case people don't use milestones
			//               code later on down the line should fail gracefully if
			//               a milestone operation is attempted
			milestones[project] = resp
		}
	}
	w.milestones = milestones
	w.priorities = w.config.Priorities
	w.types = w.config.Types

	err := w.RefreshIssues()
	if err != nil {
		return err
	}

	// sort.Sort(w)
	return nil
}

// SetBounds to manage window size
func (w *IssueWindow) SetBounds(x1, y1, x2, y2 int) {
	w.x = x1
	w.y = y1
	w.x2 = x2
	w.y2 = y2
}

// RefreshIssues updates all the issues for the current query
func (w *IssueWindow) RefreshIssues() error {
	rawIssues, err := w.api.Search(w.target)
	if err != nil {
		return err
	}

	issues := []*Issue{}
	for _, issue := range rawIssues {
		issues = append(issues, NewIssue(issue, w.milestones, w.priorities, w.types))
	}

	if w.opts.Debug {
		data, err := json.MarshalIndent(rawIssues, "", "  ")
		if err != nil {
			return err
		}
		err = ioutil.WriteFile("raw_issues.json", data, 0666)
		if err != nil {
			return err
		}
	}

	w.issues = issues
	w.currentIssues = issues
	return nil
}

// Filter the issues based on substring
func (w *IssueWindow) Filter(substr string) {
	if substr == "" {
		w.currentIssues = w.issues
		return
	}

	parts := strings.Split(substr, " ")

	selected := []*Issue{}

	IssueLoop:
	for _, issue := range w.issues {
		haystack := fmt.Sprintf("%s %s", issue.Title, issue.Repo)
		for _, label := range issue.Labels {
			haystack += fmt.Sprintf(" %s", label)
		}
		haystack = strings.ToLower(haystack)

		for _, search := range parts {
			if strings.Contains(haystack, strings.ToLower(search)) {
				// selected = append(selected, issue)
			} else if strings.Contains(substr, fmt.Sprintf("#%d", issue.Number)) {
				// selected = append(selected, issue)
			} else {
				// If we failed a match, skip to the next issue
				continue IssueLoop
			}
		}
		// if we got here we matched
		selected = append(selected, issue)
	}
	w.currentIssues = selected
}

// Scroll moves the dang window contents around
func (w *IssueWindow) Scroll(i int) {
	w.scrollIndex += i
	if w.scrollIndex > len(w.currentIssues) {
		w.scrollIndex = len(w.currentIssues) - 20
	}
	if w.scrollIndex < 0 {
		w.scrollIndex = 0
	}
	if w.scrollIndex > w.currentIndex {
		w.currentIndex = w.scrollIndex
	}
}

// HandleEvent is the entry point into key presses
func (w *IssueWindow) HandleEvent(ev termbox.Event) {
	switch ev.Type {
	case termbox.EventKey:
		switch ev.Key {
		case termbox.KeyEsc:
			fallthrough
		case termbox.KeyArrowLeft:
			// back out of stuff
			if w.currentMenu != "" {
				w.currentMenu = ""
				break
			}
			if w.currentIndex != -1 {
				w.currentIndex = -1
				break
			}
			if w.currentFilter != "" {
				w.currentFilter = ""
			}
			return
		case termbox.KeyPgdn:
			w.Scroll(20)
			return
		case termbox.KeyPgup:
			w.Scroll(-20)
			return
		case termbox.KeyArrowDown:
			// move down and maintain expandededness
			w.currentIndex++
			if w.currentIndex >= len(w.currentIssues) {
				w.currentIndex = len(w.currentIssues) - 1
			}
			if w.currentIndex-w.scrollIndex > w.y2-10 {
				w.Scroll(20)
			}
			if w.currentIndex < w.scrollIndex {
				w.currentIndex = w.scrollIndex
			}
			return
		case termbox.KeyArrowUp:
			// don't go past -1
			if w.currentIndex < 0 {
				break
			}

			if w.currentIndex-w.scrollIndex < w.y+3 {
				w.Scroll(-20)
			}

			// move up and maintain expandededness
			w.currentIndex--
			return
		case termbox.KeyF2:
			w.enableSorting = !w.enableSorting
		case termbox.KeyF5:
			w.RefreshIssues()
		case termbox.KeyBackspace:
			// Backspace starts clearing our filter
			if len(w.currentFilter) > 0 {
				w.currentFilter = w.currentFilter[:len(w.currentFilter)-1]

				// reset scrollidex
				w.scrollIndex = 0
			}
			return
		case termbox.KeyEnter:
			if w.currentFilter == ":q" || w.currentFilter == ":wq" {
				termbox.Close()
				os.Exit(0)
			}
			w.enableExpanding = !w.enableExpanding
			return
		}

		if w.currentIndex == -1 {
			// Add to the filter if we have nothing selected
			switch ev.Key {
			case termbox.KeySpace:
				w.currentFilter += " "
				w.scrollIndex = 0
			default:
				switch ev.Ch {
				case 0:
				case ' ':
				default:
					w.currentFilter += string(ev.Ch)
					// reset scrollidex
					w.scrollIndex = 0
				}
			}
		} else {
			// Try to find the menu item
			// TODO(termie): hardcoded for now
			switch ev.Ch {
			case 'p':
				w.currentMenu = "priority"
			case 't':
				w.currentMenu = "type"
			case 'm':
				w.currentMenu = "milestone"
			default:
				if w.currentMenu != "" {
					// We're in a menu, handle a menu event
					switch w.currentMenu {
					case "priority":
						w.HandlePriorityEvent(ev)
					case "type":
						w.HandleTypeEvent(ev)
					case "milestone":
						w.HandleMilestoneEvent(ev)
					}
				}
			}
		}
	}
}

// HandlePriorityEvent is the entrypoint into the priority submenu
func (w *IssueWindow) HandlePriorityEvent(ev termbox.Event) {
	issue := w.currentIssues[w.currentIndex]
	labels := []string{}

	// filter out any label that means a priority
	for _, label := range issue.Labels {
		found := false
		for _, ours := range w.priorities {
			if label == ours.Name {
				found = true
			}
		}
		if !found {
			labels = append(labels, label)
		}
	}

	// now attempt to grab our label via the index keyed in
	i, err := strconv.Atoi(fmt.Sprintf("%c", ev.Ch))
	if err != nil {
		// TODO(termie): warning
		return
	}

	if i > len(w.priorities) {
		// TODO(termie): warning
		return
	}
	var issuePriority IssuePriority
	// a "0" will delete the label
	if i > 0 {
		pri := w.priorities[i-1]
		issuePriority = IssuePriority{Index: i, Priority: &pri}
		labels = append(labels, pri.Name)
	} else {
		issuePriority = IssuePriority{Index: 0}
	}

	_, _, err = w.client.Issues.ReplaceLabelsForIssue(issue.Owner, issue.Repo, issue.Number, labels)
	if err != nil {
		panic(err)
	}
	issue.Priority = &issuePriority
	issue.Labels = labels
}

// HandleTypeEvent is the entrypoint into the type submenu
func (w *IssueWindow) HandleTypeEvent(ev termbox.Event) {
	issue := w.currentIssues[w.currentIndex]
	labels := []string{}

	// filter out any label that means a priority
	for _, label := range issue.Labels {
		found := false
		for _, ours := range w.types {
			if label == ours.Name {
				found = true
			}
		}
		if !found {
			labels = append(labels, label)
		}
	}

	// now attempt to grab our label via the index keyed in
	i, err := strconv.Atoi(fmt.Sprintf("%c", ev.Ch))
	if err != nil {
		// TODO(termie): warning
		return
	}

	if i > len(w.priorities) {
		// TODO(termie): warning
		return
	}
	// a "0" will delete the label
	var issueType IssueType
	// a "0" will delete the label
	if i > 0 {
		t := w.types[i-1]
		issueType = IssueType{Index: i, Type: &t}
		labels = append(labels, t.Name)
	} else {
		issueType = IssueType{Index: 0}
	}

	_, _, err = w.client.Issues.ReplaceLabelsForIssue(issue.Owner, issue.Repo, issue.Number, labels)
	if err != nil {
		panic(err)
	}
	issue.Type = &issueType
	issue.Labels = labels
}

// HandleMilestoneEvent is the entrypoint into the milestone submenu
func (w *IssueWindow) HandleMilestoneEvent(ev termbox.Event) {
	issue := w.currentIssues[w.currentIndex]
	milestones := w.milestones[issue.Project]
	if milestones == nil {
		// TODO(termie): display error/warning
		return
	}

	var milestone *Milestone
	var index int
	switch ev.Ch {
	case '1':
		// set current milestone
		index = 1
		milestone = milestones[0]
	case '2':
		// set next milestone
		index = 2
		milestone = milestones[1]

	case '3':
		// set someday milestone
		index = 3
		milestone = milestones[2]

	default:
		return
	}

	_, _, err := w.client.Issues.Edit(issue.Owner, issue.Repo, issue.Number, &github.IssueRequest{Milestone: &milestone.Number})
	if err != nil {
		panic(err)
	}

	issue.Milestone = &IssueMilestone{Index: index, Milestone: milestone}

}

func wordWrap(text string, length int) []string {
	s := wordwrap.WrapString(text, uint(length))
	return strings.Split(s, "\n")
}

// DrawHeader handles the top of the window
func (w *IssueWindow) DrawHeader() {
	printLine(fmt.Sprintf("[triage] %s", w.target), w.x, w.y)
}

// DrawMenu handles the hotkeys and menu items
func (w *IssueWindow) DrawMenu() {
	// top menu

	sorting := "true"
	if !w.enableSorting {
		sorting = "nope"
	}
	expanding := "true"
	if !w.enableExpanding {
		expanding = "nope"
	}

	printLine(fmt.Sprintf("  hotkeys: [F2] sorting: %s [F5] refresh issues [enter] expand: %s", sorting, expanding), w.x, w.y+1)

	if w.currentIndex == -1 {
		return
	}

	// sub menu
	if w.currentMenu == "" {
		printLine(fmt.Sprintf("    issue: [m]ilestone [p]riority [t]ype"), w.x, w.y+2)
	}

	if w.currentMenu == "milestone" {
		printLine("milestone: [1] current [2] next [3] someday", w.x, w.y+2)
	}

	if w.currentMenu == "priority" {
		menu := " priority:"
		for i, p := range w.priorities {
			menu += fmt.Sprintf(" [%d] %s", i+1, p.Name)
		}
		printLine(menu, w.x, w.y+2)
	}

	if w.currentMenu == "type" {
		menu := "     type:"
		for i, p := range w.types {
			menu += fmt.Sprintf(" [%d] %s", i+1, p.Name)
		}
		printLine(menu, w.x, w.y+2)
	}

}

// DrawFilter shows the current filter
func (w *IssueWindow) DrawFilter() {
	cursor := " "
	if w.currentIndex == -1 {
		cursor = ">"
	}
	if w.currentFilter == "" && w.currentIndex == -1 {
		printLine(" > filter: (type anything to start filtering, down-arrow to select issue)", w.x, w.y+3)
		return
	}
	printLine(fmt.Sprintf(" %s filter: %s", cursor, w.currentFilter), w.x, w.y+3)
}

// Draw does all the issues
func (w *IssueWindow) Draw() {
	w.DrawHeader()
	w.DrawMenu()
	w.DrawFilter()

	w.Filter(w.currentFilter)
	if w.enableSorting {
		sort.Sort(w)
	}
	y := 0

	//debug
	// printLine(fmt.Sprintf("ci: %d si: %d li: %d", w.currentIndex, w.scrollIndex, w.lastIndex), 1, 1)

	if w.scrollIndex > 0 {
		y++
		printLine("--more--", w.x+3, 3+y)
	}

	for i, issue := range w.currentIssues {
		if i < w.scrollIndex {
			continue
		}
		y++
		// we've reached the edge
		if y >= w.y2-4 {
			if i < len(w.currentIssues) {
				printLine("--more--", w.x+3, w.y2-1)
			}
			break
		}
		cursor := " "
		if i == w.currentIndex {
			cursor = ">"
		}
		w.lastIndex = i

		printLine(fmt.Sprintf(
			"%s %d%d%d %s/%-4d %s",
			cursor,
			issue.Milestone.Index,
			issue.Priority.Index,
			issue.Type.Index,
			issue.Repo[:5],
			issue.Number,
			issue.Title,
		), w.x+2, 3+y)

		// Check for expanded
		if i == w.currentIndex && w.enableExpanding {
			y++
			printLine(issue.URL, 8, 3+y)
			lines := wordWrap(issue.Body, w.x2-9)
			for _, line := range lines {
				y++
				printLine(line, 8, 3+y)
			}
		}
	}
}

// Len for Sortable
func (w *IssueWindow) Len() int {
	return len(w.currentIssues)
}

// Swap for Sortable
func (w *IssueWindow) Swap(i, j int) {
	(w.currentIssues)[i], (w.currentIssues)[j] = (w.currentIssues)[j], (w.currentIssues)[i]
}

// Less for Sortable, defers to TriageSortLess
func (w *IssueWindow) Less(i, j int) bool {
	return TriageSortLess(w.currentIssues[i], w.currentIssues[j])
}

// TriageSortLess sorts in order of:
// 1. Anything with Priority 1
// 2. By TriageNumber (MilestonePriorityType)
func TriageSortLess(i, j *Issue) bool {
	iNumber, _ := strconv.Atoi(fmt.Sprintf("%d%d%d", i.Milestone.Index, i.Priority.Index, i.Type.Index))
	jNumber, _ := strconv.Atoi(fmt.Sprintf("%d%d%d", j.Milestone.Index, j.Priority.Index, j.Type.Index))
	iPri := i.Priority.Index
	jPri := j.Priority.Index

	// tiebreaker
	if iNumber == jNumber {
		iNumber += i.Number
		jNumber += j.Number
	}

	if iPri == 1 {
		if jPri != 1 {
			return true
		}
		return iNumber < jNumber
	} else if jPri == 1 {
		// we already know iPri is not 1
		return false
	}
	return iNumber < jNumber
}
