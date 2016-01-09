package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/google/go-github/github"
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

type IssueResult struct {
	Issues []github.Issue
	Err    error
}

// NewIssue constructor for an Issue from a github.Issue
func NewIssue(issue github.Issue, ms map[string][]*Milestone, ps []Priority, ts []Type) *Issue {
	number := *issue.Number
	title := *issue.Title
	body := ""
	if issue.Body != nil {

		body = *issue.Body
	}
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
func (a *GithubAPI) Search(query string) <-chan *IssueResult {
	params := &github.SearchOptions{
		Order:       "updated",
		ListOptions: github.ListOptions{PerPage: 100},
	}
	out := make(chan *IssueResult)
	go func() {
		defer close(out)
		for {
			result, resp, err := a.client.Search.Issues(query, params)
			if err != nil {
				out <- &IssueResult{nil, err}
			}
			out <- &IssueResult{result.Issues, nil}
			if resp.NextPage == 0 {
				break
			}
			params.ListOptions.Page = resp.NextPage
		}
	}()
	return out
}

// ByOrg lists all issues by org
func (a *GithubAPI) ByOrg(query string) <-chan *IssueResult {
	params := &github.IssueListOptions{
		Filter:      "all",
		Sort:        "updated",
		ListOptions: github.ListOptions{PerPage: 1000},
	}
	out := make(chan *IssueResult)
	go func() {
		defer close(out)
		for {
			logger.Debugln("Issue.ListByOrg, page:", params.ListOptions.Page)
			issues, resp, err := a.client.Issues.ListByOrg(query, params)
			if err != nil {
				out <- &IssueResult{nil, err}
			}
			out <- &IssueResult{issues, nil}
			if resp.NextPage == 0 {
				break
			}
			params.ListOptions.Page = resp.NextPage
		}
	}()
	return out
}

// ByUser lists issues assigned to authenticated user
func (a *GithubAPI) ByUser() <-chan *IssueResult {
	params := &github.IssueListOptions{
		// Filter:      "all",
		Sort:        "updated",
		ListOptions: github.ListOptions{PerPage: 1000},
	}

	out := make(chan *IssueResult)
	go func() {
		defer close(out)
		for {
			logger.Debugln("Issues.List, page:", params.ListOptions.Page)
			issues, resp, err := a.client.Issues.List(true, params)
			if err != nil {
				out <- &IssueResult{nil, err}
			}
			out <- &IssueResult{issues, nil}
			if resp.NextPage == 0 {
				break
			}
			params.ListOptions.Page = resp.NextPage
		}
	}()
	return out
}

// IssueSubwindow is the base Window Impl
type IssueSubwindow struct {
	*TopIssueWindow
}

// Init noop
func (w *IssueSubwindow) Init() error {
	return nil
}

// Draw noop
func (w *IssueSubwindow) Draw(x, y, x1, y1 int) {
}

// HandleEvent noop
func (w *IssueSubwindow) HandleEvent(ev termbox.Event) (bool, error) {
	return false, nil
}

// TopIssueWindow is the Top Level Window
type TopIssueWindow struct {
	Client      *github.Client
	Opts        *Options
	Config      *Config
	API         API
	Org         string
	Target      string
	Sort        string
	Filter      string
	Status      string
	Alert       string
	Focus       Window
	ContextMenu Window
	SortFunc    func(*Issue, *Issue) bool
	SortAsc     bool
	drawSync    sync.Mutex

	// Milestones are weird
	Milestones map[string][]*Milestone
	Priorities []Priority
	Types      []Type

	// Sub-Windows
	Help              Window
	Header            Window
	FilterLine        Window
	SortLine          Window
	List              Window
	ListMenu          Window
	ListMilestoneMenu Window
	ListPriorityMenu  Window
	ListTypeMenu      Window
	AlertModal        Window
	StatusLine        Window
}

// NewTopIssueWindow ctor
func NewTopIssueWindow(client *github.Client, opts *Options, config *Config, api API, target string) *TopIssueWindow {
	return &TopIssueWindow{
		Client: client,
		Opts:   opts,
		Config: config,
		API:    api,
		Target: target,
	}
}

// Init all of the subwindows
func (w *TopIssueWindow) Init() error {
	defer profile("TopIssueWindow.Init").Stop()
	w.drawSync.Lock()
	defer w.drawSync.Unlock()
	// Decide what to search for
	// 1. if org is specified, use that
	// 2. if target is specified, use that
	// 3. if no target is specified but projects are configued, use that
	// 4. if no target and no projects, list by user
	org := w.Opts.CLI.String("org")
	if org != "" {
		w.Org = org
	} else {
		// build our search string
		target := "is:open is:issue"
		if w.Target == "" {
			if len(w.Config.Projects) > 0 {
				for _, project := range w.Config.Projects {
					target += fmt.Sprintf(" repo:%s", project)
				}
				w.Target = target
			} else {
				w.Target = ""
			}
		} else {
			target += fmt.Sprintf(" %s", w.Target)
			w.Target = target
		}
	}

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

	w.Help = NewIssueHelpWindow(w)
	w.Header = NewIssueHeaderWindow(w)
	w.List = list
	w.FilterLine = NewIssueFilterWindow(w)
	w.SortLine = NewIssueSortWindow(w)
	w.StatusLine = NewIssueStatusWindow(w)
	w.ListMenu = NewIssueListMenu(list)
	w.ListMilestoneMenu = NewIssueListMilestoneMenu(list)
	w.ListPriorityMenu = NewIssueListPriorityMenu(list)
	w.ListTypeMenu = NewIssueListTypeMenu(list)
	w.AlertModal = NewIssueAlertWindow(w)

	for _, win := range []Window{
		w.Help,
		w.Header,
		w.List,
		w.ListMenu,
		w.ListMilestoneMenu,
		w.ListPriorityMenu,
		w.ListTypeMenu,
		w.FilterLine,
		w.SortLine,
		w.StatusLine,
		w.AlertModal,
	} {
		err := win.Init()
		if err != nil {
			return err
		}
	}

	// // Start with the list focused
	w.Focus = w.List
	w.ContextMenu = w.ListMenu
	return nil
}

// Draw all the subwindows
func (w *TopIssueWindow) Draw(x, y, x1, y1 int) {
	w.Status = ""
	w.Header.Draw(x, y, x1, y)
	w.SortLine.Draw(x, y+1, x1, y+1)
	w.FilterLine.Draw(x, y+2, x1, y+2)
	if w.ContextMenu != nil {
		w.ContextMenu.Draw(x, y+3, x1, y+3)
	}
	w.List.Draw(x, y+4, x1, y1-2)
	w.StatusLine.Draw(x, y1-1, x1, y1-1)
	w.Help.Draw(x, y, x1, y1)
	w.AlertModal.Draw(x, y, x1, y1)
}

// Redraw is now our main entrypoint to drawing
func (w *TopIssueWindow) Redraw() {
	w.drawSync.Lock()
	defer w.drawSync.Unlock()
	termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)
	width, height := termbox.Size()
	w.Draw(0, 0, width, height)
	termbox.Flush()
}

// HandleEvent passes events to the subwindows
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

// HandleGlobalEvent when the subwindows don't handle them
func (w *TopIssueWindow) HandleGlobalEvent(ev termbox.Event) (bool, error) {
	switch ev.Type {
	case termbox.EventKey:
		switch ev.Key {
		case termbox.KeyEsc:
			w.Focus = w.List
			w.ContextMenu = w.ListMenu
			w.Filter = ""
			w.Sort = "+idx"
			w.SortFunc = TriageSort
			w.SortAsc = true
			return true, nil
		default:
			switch ev.Ch {
			case '/':
				w.Focus = w.FilterLine
				return true, nil
			case 's':
				w.Focus = w.SortLine
				return true, nil
			case '?':
				w.Focus = w.Help
				return true, nil
			case ':':
				w.Focus = w.StatusLine
				return true, nil
			}
		}
	}
	return false, nil
}

// Help

// IssueHelpWindow displays help text
type IssueHelpWindow struct {
	*IssueSubwindow
}

// NewIssueHelpWindow ctor
func NewIssueHelpWindow(w *TopIssueWindow) *IssueHelpWindow {
	return &IssueHelpWindow{&IssueSubwindow{w}}
}

// Draw the help window
func (w *IssueHelpWindow) Draw(x, y, x1, y1 int) {
	if w.Focus != w.Help {
		return
	}

	width, height := termbox.Size()
	buffer := termbox.CellBuffer()
	// dim the background
	for ix := 0; ix < width; ix++ {
		for iy := 0; iy < height; iy++ {
			cell := buffer[iy*width+ix]
			termbox.SetCell(ix, iy, cell.Ch, 235, cell.Bg)
		}
	}

	// our overlay
	overlay := `
         **********************************************************************
            ******************            ↳the current github search query
              ↳sort +/- by a column
   ↙  ↙  ↙   ↙
  *** ****  ***  *****

  ↙this number represents your milestone (0 means unassigned)
  *
   ↙this number represents your priority
   *
    ↙this number represents your type
    *
  *** ←together they are a sortable index, showing you the most relevant issues
`
	lines := strings.Split(overlay, "\n")
	lines = lines[1:]
	for iy, line := range lines {
		for ix, c := range line {
			fg := termbox.Attribute(5)
			bg := termbox.ColorDefault
			if c == '*' {
				fg = termbox.ColorDefault | termbox.AttrUnderline
				cell := buffer[iy*width+ix]
				c = cell.Ch
			} else if c == ' ' {
				continue
			}
			termbox.SetCell(ix, iy, c, fg, bg)
		}
	}
}

// HandleEvent closes the window on any keypress
func (w *IssueHelpWindow) HandleEvent(ev termbox.Event) (bool, error) {
	switch ev.Type {
	case termbox.EventKey:
		w.Focus = w.List
		w.ContextMenu = w.ListMenu
		return true, nil
	}
	return false, nil
}

// Alert modal

// IssueAlertWindow displays help text
type IssueAlertWindow struct {
	*IssueSubwindow
}

// NewIssueAlertWindow ctor
func NewIssueAlertWindow(w *TopIssueWindow) *IssueAlertWindow {
	return &IssueAlertWindow{&IssueSubwindow{w}}
}

// Draw the help window
func (w *IssueAlertWindow) Draw(x, y, x1, y1 int) {
	if w.Alert == "" {
		return
	}

	width, height := termbox.Size()
	buffer := termbox.CellBuffer()
	// dim the background
	for ix := 0; ix < width; ix++ {
		for iy := 0; iy < height; iy++ {
			cell := buffer[iy*width+ix]
			termbox.SetCell(ix, iy, cell.Ch, 235, cell.Bg)
		}
	}

	lines := strings.Split(w.Alert, "\n")
	// find a center
	maxWidth := 1
	maxHeight := len(lines)
	for _, line := range lines {
		if maxWidth < len(line) {
			maxWidth = len(line)
		}
	}
	startX := (width / 2) - (maxWidth / 2)
	startY := (height / 2) - (maxHeight / 2)

	logger.Debugf("startX: %d startY: %d\n", startX, startY)

	for iy, line := range lines {
		for ix, c := range line {
			fg := termbox.Attribute(5)
			bg := termbox.ColorDefault
			if c == ' ' {
				continue
			}
			termbox.SetCell(ix+startX, iy+startY, c, fg, bg)
		}
	}
}

// HandleEvent closes the window on any keypress
func (w *IssueAlertWindow) HandleEvent(ev termbox.Event) (bool, error) {
	switch ev.Type {
	case termbox.EventKey:
		w.Focus = w.List
		w.ContextMenu = w.ListMenu
		return true, nil
	}
	return false, nil
}

// Header

// IssueHeaderWindow shows the title bar
type IssueHeaderWindow struct {
	*IssueSubwindow
}

// NewIssueHeaderWindow ctor
func NewIssueHeaderWindow(w *TopIssueWindow) *IssueHeaderWindow {
	return &IssueHeaderWindow{&IssueSubwindow{w}}
}

// Draw the titlebar
func (w *IssueHeaderWindow) Draw(x, y, x1, y1 int) {
	// org if it exists, target if it exists, otherwise by user
	title := w.Target
	if w.Org != "" {
		title = fmt.Sprintf("all open issues for org=%s", w.Org)
	} else if title == "" {
		title = fmt.Sprintf("assigned issues for authenticated user")
	}

	printLine(fmt.Sprintf("*triage* %s", title), x, y)
}

// Statusline

// IssueStatusWindow shows some extra info on the bottom of the screen
type IssueStatusWindow struct {
	*IssueSubwindow
	Buffer string
}

// NewIssueStatusWindow ctor
func NewIssueStatusWindow(w *TopIssueWindow) *IssueStatusWindow {
	return &IssueStatusWindow{&IssueSubwindow{w}, ""}
}

// Draw the status line
func (w *IssueStatusWindow) Draw(x, y, x1, y1 int) {
	if w.Focus != w {
		printLine(fmt.Sprintf("[:] %s", w.Status), x, y)
		return
	}
	printLine(fmt.Sprintf(":%s", w.Buffer), x, y)
	termbox.SetCursor(x+1+len(w.Buffer), y)
}

// HandleEvent for our vim-style exit keys
func (w *IssueStatusWindow) HandleEvent(ev termbox.Event) (bool, error) {
	switch ev.Type {
	case termbox.EventKey:
		switch ev.Key {
		case termbox.KeyEsc:
			termbox.HideCursor()
			w.Focus = w.List
			w.ContextMenu = w.ListMenu
			return true, nil
		case termbox.KeyBackspace:
			// Backspace starts clearing our filter
			if len(w.Buffer) > 0 {
				w.Buffer = w.Buffer[:len(w.Buffer)-1]
				return true, nil
			}
			termbox.HideCursor()
			w.Focus = w.List
			w.ContextMenu = w.ListMenu
			return true, nil
		case termbox.KeySpace:
			w.Buffer += " "
			return true, nil
		case termbox.KeyEnter:
			w.execute(w.Buffer)
			return true, nil
		default:
			switch ev.Ch {
			case 0:
			case ' ':
			default:
				w.Buffer += string(ev.Ch)
				return true, nil
			}
		}
	}
	return false, nil
}

// execute the statusline for "q" and "wq"
func (w *IssueStatusWindow) execute(s string) {
	switch s {
	case "q":
		fallthrough
	case "wq":
		termbox.Close()
		os.Exit(0)
	}
}

// Filter

// IssueFilterWindow handles the filter box
type IssueFilterWindow struct {
	*IssueSubwindow
}

// NewIssueFilterWindow ctor
func NewIssueFilterWindow(w *TopIssueWindow) *IssueFilterWindow {
	return &IssueFilterWindow{&IssueSubwindow{w}}
}

// Draw and move the cursor to the filter box
func (w *IssueFilterWindow) Draw(x, y, x1, y1 int) {
	cursor := " "
	fg := termbox.ColorDefault
	bg := termbox.ColorDefault
	if w.Focus == w {
		cursor = ">"
		fg = 0xe9
		bg = 0xfa
	}
	pre := fmt.Sprintf("%s[/] filter: ", cursor)

	printLine(pre, x+1, y)
	printLineColor(w.Filter, x+1+len(pre), y, fg, bg)
	if w.Focus == w {
		for i := x + 1 + len(pre) + len(w.Filter); i < 60; i++ {
			termbox.SetCell(x+i, y, ' ', fg, bg)
			termbox.SetCursor(x+1+len(pre)+len(w.Filter), y)
		}
	}

	// printLine(fmt.Sprintf("%s[/] filter: %s", cursor, w.Filter), x+1, y)
}

// HandleEvent builds the filter string
func (w *IssueFilterWindow) HandleEvent(ev termbox.Event) (bool, error) {
	switch ev.Type {
	case termbox.EventKey:
		switch ev.Key {
		case termbox.KeyArrowUp:
			termbox.HideCursor()
			w.Focus = w.SortLine
			w.ContextMenu = nil
			return true, nil
		case termbox.KeyArrowDown:
			fallthrough
		case termbox.KeyEsc:
			termbox.HideCursor()
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

// Sort

// IssueSortWindow handle the sort box and global menu
type IssueSortWindow struct {
	*IssueSubwindow

	valid bool
}

// NewIssueSortWindow ctor
func NewIssueSortWindow(w *TopIssueWindow) *IssueSortWindow {
	return &IssueSortWindow{&IssueSubwindow{w}, false}
}

// Init sets up our initial sorting functions
func (w *IssueSortWindow) Init() error {
	// TODO(termie): allow sort to be specified in options
	w.Sort = "+idx"
	w.update(w.Sort)
	// w.SortAsc = true
	// w.SortFunc = TriageSort
	return nil
}

// Draw and move the cursor to the sort box
func (w *IssueSortWindow) Draw(x, y, x1, y1 int) {
	cursor := " "
	fg := termbox.ColorDefault
	bg := termbox.ColorDefault
	if w.Focus == w {
		cursor = ">"
		if w.valid {
			fg = 0xe9
		} else {
			fg = 0x02
		}
		bg = 0xfa
	}
	pre := fmt.Sprintf("%s[s] sort: ", cursor)

	printLine(pre, x+1, y)
	printLineColor(w.Sort, x+1+len(pre), y, fg, bg)
	if w.Focus == w {
		for i := x + 1 + len(pre) + len(w.Sort); i < x+30; i++ {
			termbox.SetCell(x+i, y, ' ', fg, bg)
			termbox.SetCursor(x+1+len(pre)+len(w.Sort), y)
		}
	}
	printLine(" [?] help [^C] exit", x+30, y)
}

// HandleEvent builds the sort string
func (w *IssueSortWindow) HandleEvent(ev termbox.Event) (bool, error) {
	switch ev.Type {
	case termbox.EventKey:
		switch ev.Key {
		case termbox.KeyArrowDown:
			w.Focus = w.FilterLine
			w.ContextMenu = nil
		case termbox.KeyEsc:
			w.Focus = w.List
			w.ContextMenu = w.ListMenu
			return true, nil
		case termbox.KeyBackspace:
			// Backspace starts clearing our filter
			if len(w.Sort) > 0 {
				w.Sort = w.Sort[:len(w.Sort)-1]
				w.update(w.Sort)
			}
			return true, nil
		case termbox.KeySpace:
			w.Sort += " "
			return true, nil
		default:
			switch ev.Ch {
			case 0:
			case ' ':
			default:
				w.Sort += string(ev.Ch)
				w.update(w.Sort)
				return true, nil
			}
		}
	}
	return false, nil
}

// update the sort func based on sort string
func (w *IssueSortWindow) update(s string) {
	if len(s) < 2 {
		return
	}
	s = strings.ToLower(s)

	asc := true
	if s[0] == '-' {
		asc = false
		s = s[1:]
	} else if s[0] == '+' {
		s = s[1:]
	}

	switch s {
	case "idx":
		w.SortFunc = TriageSort
		w.valid = true
		w.SortAsc = asc
	case "repo":
		w.SortFunc = RepoSort
		w.valid = true
		w.SortAsc = asc
	case "num":
		w.SortFunc = NumberSort
		w.valid = true
		w.SortAsc = asc
	case "title":
		w.SortFunc = TitleSort
		w.valid = true
		w.SortAsc = asc
	default:
		w.SortFunc = nil
		w.valid = false
	}

}

// Context menus for the issue list

// IssueListMenu is the default menu when issue list is focused
type IssueListMenu struct {
	*IssueListWindow
}

// NewIssueListMenu ctor
func NewIssueListMenu(w *IssueListWindow) *IssueListMenu {
	return &IssueListMenu{w}
}

// Init noop (needed to prevent IssueList.Init being called)
func (w *IssueListMenu) Init() error {
	return nil
}

// Draw the menu
func (w *IssueListMenu) Draw(x, y, x1, y1 int) {
	if w.Focus != w.List {
		return
	}

	expand := "expand"
	if w.expanding {
		expand = "collapse"
	}

	printLine(fmt.Sprintf("[m] set milestone [p] set priority [t] set type [enter] %s", expand), x+2, y)
}

// HandleEvent for the menu
func (w *IssueListMenu) HandleEvent(ev termbox.Event) (bool, error) {
	switch ev.Type {
	case termbox.EventKey:
		switch ev.Key {
		case termbox.KeyEnter:
			w.expanding = !w.expanding
			return true, nil
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

// IssueListMilestoneMenu for setting milestones
type IssueListMilestoneMenu struct {
	*IssueListWindow
}

// NewIssueListMilestoneMenu ctor
func NewIssueListMilestoneMenu(w *IssueListWindow) *IssueListMilestoneMenu {
	return &IssueListMilestoneMenu{w}
}

// Init noop (needed to prevent IssueList.Init being called)
func (w *IssueListMilestoneMenu) Init() error {
	return nil
}

// Draw the milestone menu
func (w *IssueListMilestoneMenu) Draw(x, y, x1, y1 int) {
	if w.Focus != w.List {
		return
	}
	printLine("milestone: [1] current [2] next [3] someday", x+2, y)
}

// HandleEvent sets the milestone
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

// IssueListPriorityMenu for setting priority
type IssueListPriorityMenu struct {
	*IssueListWindow
}

// NewIssueListPriorityMenu ctor
func NewIssueListPriorityMenu(w *IssueListWindow) *IssueListPriorityMenu {
	return &IssueListPriorityMenu{w}
}

// Init noop (needed to prevent IssueList.Init being called)
func (w *IssueListPriorityMenu) Init() error {
	return nil
}

// Draw the priority menu
func (w *IssueListPriorityMenu) Draw(x, y, x1, y1 int) {
	if w.Focus != w.List {
		return
	}

	menu := "priority:"
	for i, p := range w.Priorities {
		menu += fmt.Sprintf(" [%d] %s", i+1, p.Name)
	}
	printLine(menu, x+2, y)
}

// HandleEvent sets the priority
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

// IssueListTypeMenu for setting type
type IssueListTypeMenu struct {
	*IssueListWindow
}

// NewIssueListTypeMenu ctor
func NewIssueListTypeMenu(w *IssueListWindow) *IssueListTypeMenu {
	return &IssueListTypeMenu{w}
}

// Init noop (needed to prevent IssueList.Init being called)
func (w *IssueListTypeMenu) Init() error {
	return nil
}

// Draw the type menu
func (w *IssueListTypeMenu) Draw(x, y, x1, y1 int) {
	if w.Focus != w.List {
		return
	}
	menu := "type:"
	for i, p := range w.Types {
		menu += fmt.Sprintf(" [%d] %s", i+1, p.Name)
	}
	printLine(menu, x+2, y)
}

// HandleEvent sets the type
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

// IssueListWindow is the main list of issues
type IssueListWindow struct {
	issues        []*Issue
	currentIssues []*Issue
	currentIndex  int
	lastIndex     int
	scrollIndex   int
	expanding     bool

	currentFilter string

	*IssueSubwindow
}

// NewIssueListWindow ctor
func NewIssueListWindow(w *TopIssueWindow) *IssueListWindow {
	return &IssueListWindow{IssueSubwindow: &IssueSubwindow{w}}
}

// Init fetches the initial issues
func (w *IssueListWindow) Init() error {
	// err := w.refresh()
	// if err != nil {
	//   return err
	// }

	// fetch the initial list of issues, etc
	go w.refresh()

	return nil
}

// Draw the header and list of issues
func (w *IssueListWindow) Draw(x, y, x1, y1 int) {
	w.filter(w.Filter)
	w.sort()

	line := 0

	//debug
	if w.Opts.Debug {
		w.Status += fmt.Sprintf(" ci: %d si: %d li: %d ", w.currentIndex, w.scrollIndex, w.lastIndex)
	}

	// headers
	headerFg := termbox.ColorDefault | termbox.AttrUnderline
	headers := " idx repo  num  title"
	for i, c := range headers {
		fg := headerFg
		if c == ' ' {
			fg = termbox.ColorDefault
		}
		termbox.SetCell(x+1+i, y+line, c, fg, termbox.ColorDefault)
	}

	line++

	if w.scrollIndex > 0 {
		// // printLine("--more--", x+3, y)
		// printLine(string('\u2191'), x, y)
		termbox.SetCell(x, y+line, '\u2191', termbox.ColorDefault, termbox.ColorDefault)
	}

	for i, issue := range w.currentIssues {
		if i < w.scrollIndex {
			continue
		}
		cursor := " "
		if i == w.currentIndex && w.Focus == w {
			cursor = ">"
			w.Status += fmt.Sprintf("%s/%s", issue.Owner, issue.Repo)
			for _, label := range issue.Labels {
				w.Status += fmt.Sprintf(" %s", label)
			}
		}
		w.lastIndex = i

		repo := issue.Repo
		if len(repo) > 5 {
			repo = repo[:5]
		}

		printLine(fmt.Sprintf(
			"%s%d%d%d % 5s/%-4d %s",
			cursor,
			issue.Milestone.Index,
			issue.Priority.Index,
			issue.Type.Index,
			repo,
			issue.Number,
			issue.Title,
		), x+1, y+line)

		// we've reached the edge
		if y+line >= y1 {
			if i < len(w.currentIssues) {
				termbox.SetCell(x, y1, '\u2193', termbox.ColorDefault, termbox.ColorDefault)
				// printLine("--more--", x+3, y1-1)
			}
			break
		}

		// Check for expanded
		if i == w.currentIndex && w.expanding {
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
			if w.lastIndex < w.currentIndex && w.lastIndex < len(w.currentIssues)-1 {
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

			if w.currentIndex < w.scrollIndex {
				w.scroll(-10)
			}
			return true, nil
		}
	}

	return false, nil
}

// refresh updates all the issues for the current query
func (w *IssueListWindow) refresh() error {
	defer profile("IssueListWindow.refresh").Stop()

	// Decide what to search for
	// 1. if org is specified, use that
	// 2. if target is specified, use that
	// 3. if no target is specified but projects are configued, use that
	// 4. if no target and no projects, list by user
	var resultsChan <-chan *IssueResult
	var err error
	if w.Org != "" {
		resultsChan = w.API.ByOrg(w.Org)
	} else if w.Target != "" {
		resultsChan = w.API.Search(w.Target)
	} else {
		resultsChan = w.API.ByUser()
	}
	if err != nil {
		return err
	}

	issues := []*Issue{}
	w.Alert = "Fetching issues..."
	w.Redraw()
	for result := range resultsChan {
		if result.Err != nil {
			return result.Err
		}
		for _, issue := range result.Issues {
			issues = append(issues, NewIssue(issue, w.Milestones, w.Priorities, w.Types))
		}
		w.issues = issues
		w.currentIssues = issues
		w.Alert = fmt.Sprintf("Fetching issues, got: %d", len(issues))
		w.Redraw()
	}

	if w.Opts.Debug {
		data, err := json.MarshalIndent(issues, "", "  ")
		if err != nil {
			return err
		}
		err = ioutil.WriteFile("raw_issues.json", data, 0666)
		if err != nil {
			return err
		}
	}
	w.Alert = ""
	w.Redraw()
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
		haystack := fmt.Sprintf("%d %s %s m%d p%d t%d", issue.Number, issue.Repo, issue.Title, issue.Milestone.Index, issue.Priority.Index, issue.Type.Index)
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

// sort the issues based on sort string
func (w *IssueListWindow) sort() {
	if w.SortFunc == nil {
		return
	}
	sort.Sort(w)
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

// Len for Sortable
func (w *IssueListWindow) Len() int {
	return len(w.currentIssues)
}

// Swap for Sortable
func (w *IssueListWindow) Swap(i, j int) {
	(w.currentIssues)[i], (w.currentIssues)[j] = (w.currentIssues)[j], (w.currentIssues)[i]
}

// Less for Sortable, defers to TriageSortLess
func (w *IssueListWindow) Less(i, j int) bool {
	result := w.SortFunc(w.currentIssues[i], w.currentIssues[j])
	if w.SortAsc == false {
		return !result
	}
	return result
}

// RepoSort sorts by repo name then triagesort
func RepoSort(i, j *Issue) bool {
	if i.Repo == j.Repo {
		return TriageSort(i, j)
	}
	return i.Repo < j.Repo
}

// NumberSort sorts by number then triagesort
func NumberSort(i, j *Issue) bool {
	if i.Number == j.Number {
		return TriageSort(i, j)
	}
	return i.Number < j.Number
}

// TitleSort sorts by title then triagesort
func TitleSort(i, j *Issue) bool {
	if i.Title == j.Title {
		return TriageSort(i, j)
	}
	return i.Title < j.Title
}

// TriageSortLess sorts in order of:
// 1. Anything with Priority 1
// 2. By TriageNumber (MilestonePriorityType)
func TriageSort(i, j *Issue) bool {
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
		if iNumber == jNumber {
			return i.Repo < j.Repo
		}
		return iNumber < jNumber
	} else if jPri == 1 {
		// we already know iPri is not 1
		return false
	}
	if iNumber == jNumber {
		return i.Repo < j.Repo
	}
	return iNumber < jNumber
}
