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

type IssueWindowEventHandler func(*IssueWindow, termbox.Event) (bool, error)

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
		haystack := fmt.Sprintf("%s %s %s", issue.Number, issue.Repo, issue.Title)
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

// Base Window Impl
type IssueSubwindow struct {
	*TopIssueWindow
}

func (w *IssueSubwindow) Init() error {
	return nil
}

func (w *IssueSubwindow) Draw(x, y, x1, y1 int) {
}

func (w *IssueSubwindow) HandleEvent(ev termbox.Event) (bool, error) {
	return false, nil
}

// Top Level Window
type TopIssueWindow struct {
	Client      *github.Client
	Opts        *Options
	Config      *Config
	API         API
	Target      string
	Filter      string
	Status      string
	Focus       IWindow
	ContextMenu IWindow

	// Milestones are weird
	Milestones map[string][]*Milestone
	Priorities []Priority
	Types      []Type

	// Sub-Windows
	Header            IWindow
	Menu              IWindow
	DefaultMenu       IWindow
	FilterLine        IWindow
	List              IWindow
	ListMenu          IWindow
	ListMilestoneMenu IWindow
	ListPriorityMenu  IWindow
	ListTypeMenu      IWindow
	Alert             IWindow
	StatusLine        IWindow
}

func NewTopIssueWindow(client *github.Client, opts *Options, config *Config, api API, target string) *TopIssueWindow {
	return &TopIssueWindow{
		Client: client,
		Opts:   opts,
		Config: config,
		API:    api,
		Target: target,
	}
}

func (w *TopIssueWindow) Init() error {
	// build our search string
	target := "is:open is:issue"
	if w.Target == "" {
		if len(w.Config.Projects) > 0 {
			for _, project := range w.Config.Projects {
				target += fmt.Sprintf(" repo:%s", project)
			}
		}
	} else {
		target += fmt.Sprintf(" %s", w.Target)
	}
	w.Target = target

	// build our milestones, priorities, types
	milestones := map[string][]*Milestone{}
	for _, project := range w.Config.Projects {
		resp, err := w.API.Milestones(project)
		if err == nil {
			// NOTE(termie): ignoring this error in case people don't use milestones
			//               code later on down the line should fail gracefully if
			//               a milestone operation is attempted
			milestones[project] = resp
		}
	}
	w.Milestones = milestones
	w.Priorities = w.Config.Priorities
	w.Types = w.Config.Types

	list := NewIssueListWindow(w)

	w.Header = NewIssueHeaderWindow(w)
	w.List = list
	w.FilterLine = NewIssueFilterWindow(w)
	w.StatusLine = NewIssueStatusWindow(w)
	w.ListMenu = NewIssueListMenu(list)
	w.ListMilestoneMenu = NewIssueListMilestoneMenu(list)
	w.ListPriorityMenu = NewIssueListPriorityMenu(list)
	w.ListTypeMenu = NewIssueListTypeMenu(list)

	for _, win := range []IWindow{
		w.Header,
		w.List,
		w.ListMenu,
		w.ListMilestoneMenu,
		w.ListPriorityMenu,
		w.ListTypeMenu,
		w.FilterLine,
		w.StatusLine,
	} {
		err := win.Init()
		if err != nil {
			return err
		}
	}
	// w.DefaultMenu = NewIssueWindowDefaultMenu(w)
	// w.IssueMenu = NewIssueWindowIssueMenu(w)
	// w.MilestoneMenu = NewIssueWindowMilestoneMenu(w)
	// w.Menu = w.DefaultMenu
	// w.Filter = NewFilterWindow(w)

	// // Start with the list focused
	w.Focus = w.List
	w.ContextMenu = w.ListMenu
	return nil
}

func (w *TopIssueWindow) Draw(x, y, x1, y1 int) {
	w.Status = ""
	w.Header.Draw(x, y, x1, y)
	// w.Menu.Draw(x, y+1, x1, y+2)
	w.FilterLine.Draw(x, y+2, x1, y+2)
	if w.ContextMenu != nil {
		w.ContextMenu.Draw(x, y+3, x1, y+3)
	}
	w.List.Draw(x, y+4, x1, y1-1)
	w.StatusLine.Draw(x, y1-1, x1, y1-1)
}

func (w *TopIssueWindow) HandleEvent(ev termbox.Event) (bool, error) {
	handled, err := w.Focus.HandleEvent(ev)
	if err != nil {
		return true, err
	}
	if !handled {
		return w.HandleGlobalEvent(ev)
	}
	return true, nil
}

func (w *TopIssueWindow) HandleGlobalEvent(ev termbox.Event) (bool, error) {
	switch ev.Type {
	case termbox.EventKey:
		switch ev.Key {
		case termbox.KeyEsc:
			w.Focus = w.List
			w.ContextMenu = w.ListMenu
			w.Filter = ""
			return true, nil
		default:
			switch ev.Ch {
			case '/':
				w.Focus = w.FilterLine
				return true, nil
			}
		}
	}
	return false, nil
}

// Header
type IssueHeaderWindow struct {
	*IssueSubwindow
}

func NewIssueHeaderWindow(w *TopIssueWindow) *IssueHeaderWindow {
	return &IssueHeaderWindow{&IssueSubwindow{w}}
}

func (w *IssueHeaderWindow) Draw(x, y, x1, y1 int) {
	printLine(fmt.Sprintf("*triage* %s", w.Target), x, y)
}

// Statusline
type IssueStatusWindow struct {
	*IssueSubwindow
}

func NewIssueStatusWindow(w *TopIssueWindow) *IssueStatusWindow {
	return &IssueStatusWindow{&IssueSubwindow{w}}
}

func (w *IssueStatusWindow) Draw(x, y, x1, y1 int) {
	printLine(fmt.Sprintf("[%s", w.Status), x, y)
}

// Filter
type IssueFilterWindow struct {
	*IssueSubwindow
}

func NewIssueFilterWindow(w *TopIssueWindow) *IssueFilterWindow {
	return &IssueFilterWindow{&IssueSubwindow{w}}
}

func (w *IssueFilterWindow) Draw(x, y, x1, y1 int) {
	cursor := " "
	if w.Focus == w {
		cursor = ">"
	}
	printLine(fmt.Sprintf("%s [/] filter: %s", cursor, w.Filter), x, y)
}

func (w *IssueFilterWindow) HandleEvent(ev termbox.Event) (bool, error) {
	switch ev.Type {
	case termbox.EventKey:
		switch ev.Key {
		case termbox.KeyArrowDown:
			fallthrough
		case termbox.KeyEsc:
			w.Focus = w.List
			w.ContextMenu = w.ListMenu
			return true, nil
		case termbox.KeyBackspace:
			// Backspace starts clearing our filter
			if len(w.Filter) > 0 {
				w.Filter = w.Filter[:len(w.Filter)-1]
			}
			return true, nil
		case termbox.KeySpace:
			w.Filter += " "
			return true, nil
		default:
			switch ev.Ch {
			case 0:
			case ' ':
			default:
				w.Filter += string(ev.Ch)
				return true, nil
			}
		}
	}
	return false, nil
}

// IssueListMenu
type IssueListMenu struct {
	*IssueListWindow
}

func NewIssueListMenu(w *IssueListWindow) *IssueListMenu {
	return &IssueListMenu{w}
}

func (w *IssueListMenu) Draw(x, y, x1, y1 int) {
	printLine("[m] milestone [p] priority [t] type", x+2, y)
}

func (w *IssueListMenu) HandleEvent(ev termbox.Event) (bool, error) {
	switch ev.Type {
	case termbox.EventKey:
		switch ev.Key {
		default:
			switch ev.Ch {
			case 'm':
				w.ContextMenu = w.ListMilestoneMenu
				return true, nil
			case 'p':
				w.ContextMenu = w.ListPriorityMenu
				return true, nil
			case 't':
				w.ContextMenu = w.ListTypeMenu
				return true, nil
			}
		}
	}
	return false, nil
}

// IssueListMilestoneMenu
type IssueListMilestoneMenu struct {
	*IssueListWindow
}

func NewIssueListMilestoneMenu(w *IssueListWindow) *IssueListMilestoneMenu {
	return &IssueListMilestoneMenu{w}
}

func (w *IssueListMilestoneMenu) Draw(x, y, x1, y1 int) {
	printLine("milestone: [1] current [2] next [3] someday", x+2, y)
}

func (w *IssueListMilestoneMenu) HandleEvent(ev termbox.Event) (bool, error) {
	issue := w.currentIssues[w.currentIndex]
	milestones := w.Milestones[issue.Project]
	if milestones == nil {
		// TODO(termie): display error/warning
		return false, nil
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
		return false, nil
	}

	_, _, err := w.Client.Issues.Edit(issue.Owner, issue.Repo, issue.Number, &github.IssueRequest{Milestone: &milestone.Number})
	if err != nil {
		return true, err
	}

	issue.Milestone = &IssueMilestone{Index: index, Milestone: milestone}
	return true, nil

}

// IssueListPriorityMenu
type IssueListPriorityMenu struct {
	*IssueListWindow
}

func NewIssueListPriorityMenu(w *IssueListWindow) *IssueListPriorityMenu {
	return &IssueListPriorityMenu{w}
}

func (w *IssueListPriorityMenu) Draw(x, y, x1, y1 int) {
	menu := "priority:"
	for i, p := range w.Priorities {
		menu += fmt.Sprintf(" [%d] %s", i+1, p.Name)
	}
	printLine(menu, x+2, y)
}

func (w *IssueListPriorityMenu) HandleEvent(ev termbox.Event) (bool, error) {
	issue := w.currentIssues[w.currentIndex]
	labels := []string{}

	// filter out any label that means a priority
	for _, label := range issue.Labels {
		found := false
		for _, ours := range w.Priorities {
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
		return false, nil
	}

	if i > len(w.Priorities) {
		// TODO(termie): warning
		return false, nil
	}
	var issuePriority IssuePriority
	// a "0" will delete the label
	if i > 0 {
		pri := w.Priorities[i-1]
		issuePriority = IssuePriority{Index: i, Priority: &pri}
		labels = append(labels, pri.Name)
	} else {
		issuePriority = IssuePriority{Index: 0}
	}

	_, _, err = w.Client.Issues.ReplaceLabelsForIssue(issue.Owner, issue.Repo, issue.Number, labels)
	if err != nil {
		return true, err
	}
	issue.Priority = &issuePriority
	issue.Labels = labels
	return true, nil
}

// IssueListTypeMenu
type IssueListTypeMenu struct {
	*IssueListWindow
}

func NewIssueListTypeMenu(w *IssueListWindow) *IssueListTypeMenu {
	return &IssueListTypeMenu{w}
}

func (w *IssueListTypeMenu) Draw(x, y, x1, y1 int) {
	menu := "type:"
	for i, p := range w.Types {
		menu += fmt.Sprintf(" [%d] %s", i+1, p.Name)
	}
	printLine(menu, x+2, y)
}

func (w *IssueListTypeMenu) HandleEvent(ev termbox.Event) (bool, error) {
	issue := w.currentIssues[w.currentIndex]
	labels := []string{}

	// filter out any label that means a priority
	for _, label := range issue.Labels {
		found := false
		for _, ours := range w.Types {
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
		return false, nil
	}

	if i > len(w.Types) {
		// TODO(termie): warning
		return false, nil
	}
	var issueType IssueType
	// a "0" will delete the label
	if i > 0 {
		pri := w.Types[i-1]
		issueType = IssueType{Index: i, Type: &pri}
		labels = append(labels, pri.Name)
	} else {
		issueType = IssueType{Index: 0}
	}

	_, _, err = w.Client.Issues.ReplaceLabelsForIssue(issue.Owner, issue.Repo, issue.Number, labels)
	if err != nil {
		return true, err
	}
	issue.Type = &issueType
	issue.Labels = labels
	return true, nil
}

// Issue List
type IssueListWindow struct {
	issues          []*Issue
	currentIssues   []*Issue
	currentIndex    int
	lastIndex       int
	scrollIndex     int
	enableSorting   bool
	enableExpanding bool

	currentFilter string

	*IssueSubwindow
}

func NewIssueListWindow(w *TopIssueWindow) *IssueListWindow {
	return &IssueListWindow{IssueSubwindow: &IssueSubwindow{w}}
}

func (w *IssueListWindow) Init() error {
	// fetch the initial list of issues, etc
	err := w.refresh()
	if err != nil {
		return err
	}

	return nil
}

func (w *IssueListWindow) Draw(x, y, x1, y1 int) {
	w.filter(w.Filter)

	// if w.enableSorting {
	//   sort.Sort(w)
	// }
	line := 0

	//debug
	w.Status += fmt.Sprintf(" ci: %d si: %d li: %d", w.currentIndex, w.scrollIndex, w.lastIndex)

	if w.scrollIndex > 0 {
		printLine("--more--", x+3, y)
		line++
	}

	for i, issue := range w.currentIssues {
		if i < w.scrollIndex {
			continue
		}
		// we've reached the edge
		if y+line >= y1-1 {
			if i < len(w.currentIssues) {
				printLine("--more--", x+3, y1-1)
			}
			break
		}
		cursor := " "
		if i == w.currentIndex && w.Focus == w {
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
		), x, y+line)

		// Check for expanded
		if i == w.currentIndex && w.enableExpanding {
			y++
			printLine(issue.URL, x+5, y+line)
			body := wordWrap(issue.Body, x1-9)
			for _, text := range body {
				y++
				printLine(text, x+5, y+line)
			}
		}

		line++
	}
}

// HandleEvent is mostly movement events and triggering submenus
func (w *IssueListWindow) HandleEvent(ev termbox.Event) (bool, error) {
	// Check the menu first
	handled, err := w.ContextMenu.HandleEvent(ev)
	if err != nil {
		return true, err
	}
	if handled {
		return true, nil
	}
	// Otherwise we'll handle the event
	switch ev.Type {
	case termbox.EventKey:
		switch ev.Key {
		case termbox.KeyEsc:
			if w.ContextMenu != w.ListMenu {
				w.ContextMenu = w.ListMenu
				return true, nil
			}
			return false, nil
		case termbox.KeyPgdn:
			w.scroll(10)
			return true, nil
		case termbox.KeyPgup:
			w.scroll(-10)
			return true, nil
		case termbox.KeyArrowDown:
			w.currentIndex++
			if w.currentIndex >= len(w.currentIssues) {
				w.currentIndex = len(w.currentIssues) - 1
			}
			if w.lastIndex-w.currentIndex < 1 && w.lastIndex < len(w.currentIssues)-1 {
				w.scroll(10)
			}
			if w.currentIndex > w.lastIndex {
				w.currentIndex = w.lastIndex
			}
			return true, nil
		case termbox.KeyArrowUp:
			// if we're already at 0 and we hit up, go to the filter
			if w.currentIndex == 0 {
				w.Focus = w.FilterLine
				w.ContextMenu = nil
				return true, nil
			}
			// move up
			w.currentIndex--

			if w.currentIndex < 1 {
				w.currentIndex = 0
			}

			if w.currentIndex-w.scrollIndex < 1 {
				w.scroll(-10)
			}
			return true, nil
		}
	}

	return false, nil
}

// refreshIssues updates all the issues for the current query
func (w *IssueListWindow) refresh() error {
	rawIssues, err := w.API.Search(w.Target)
	if err != nil {
		return err
	}

	issues := []*Issue{}
	for _, issue := range rawIssues {
		issues = append(issues, NewIssue(issue, w.Milestones, w.Priorities, w.Types))
	}

	if w.Opts.Debug {
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

// filter the issues based on substring
func (w *IssueListWindow) filter(substr string) {
	if substr == w.currentFilter {
		return
	}
	w.currentFilter = substr
	w.scrollIndex = 0
	w.currentIndex = 0
	if substr == "" {
		w.currentIssues = w.issues
		return
	}

	parts := strings.Split(substr, " ")

	selected := []*Issue{}

IssueLoop:
	for _, issue := range w.issues {
		haystack := fmt.Sprintf("%s %s %s", issue.Number, issue.Repo, issue.Title)
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

// scroll moves the dang window contents around
func (w *IssueListWindow) scroll(i int) {
	w.scrollIndex += i
	if w.scrollIndex >= len(w.currentIssues) {
		w.scrollIndex = len(w.currentIssues) - 10
	}
	if w.scrollIndex < 0 {
		w.scrollIndex = 0
	}
	if w.scrollIndex > w.currentIndex {
		w.currentIndex = w.scrollIndex
	}
	if w.currentIndex > w.lastIndex {
		w.currentIndex = w.lastIndex
	}
}

// Sorting Stuff

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
