package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/google/go-github/github"
	"github.com/mitchellh/go-wordwrap"
	"github.com/nsf/termbox-go"
)

var Priority = map[int]string{
	1: "pri:blocker",
	2: "pri:critical",
	3: "pri:normal",
	4: "pri:low",
}

func GetPriority(name string) int {
	for i, check := range Priority {
		if check == name {
			return i
		}
	}
	return 0
}

var Type = map[int]string{
	1: "type:bug",
	2: "type:task",
	3: "type:enhancement",
	4: "type:question",
}

func GetType(name string) int {
	for i, check := range Type {
		if check == name {
			return i
		}
	}
	return 0
}

type IssueWindow struct {
	client *github.Client
	issues []*github.Issue
	// selected      map[string]struct{}
	// selectedRepos []github.Issues
	currentIndex    int
	lastIndex       int
	currentExpanded int
	currentIssues   []*github.Issue
	currentFilter   string
	currentMenu     string
	scrollIndex     int
	// Milestones are weird
	milestones map[int]int

	// currentMode   string
	x  int
	y  int
	x2 int
	y2 int
}

func NewIssueWindow(client *github.Client) *IssueWindow {
	return &IssueWindow{
		client: client,
		// selected:     map[string]struct{}{},
		currentIndex:    -1,
		currentExpanded: -1,
	}
}

func (w *IssueWindow) ID() string {
	return "issues"
}

func (w *IssueWindow) SetBounds(x1, y1, x2, y2 int) {
	w.x = x1
	w.y = y1
	w.x2 = x2
	w.y2 = y2
}

func (w *IssueWindow) searchIssues(query string) ([]github.Issue, error) {
	var issues []github.Issue
	var err error
	if query == "" {
		issues, _, err = w.client.Issues.ListByOrg(
			"wercker",
			&github.IssueListOptions{},
		)
		if err != nil {
			return nil, err
		}
	} else {
		result, _, err := w.client.Search.Issues(query,
			&github.SearchOptions{
				Order:       "updated",
				ListOptions: github.ListOptions{PerPage: 1000},
			})
		if err != nil {
			return nil, err
		}
		issues = result.Issues
	}
	return issues, nil
}

func (w *IssueWindow) RefreshIssues() error {
	var issues []*github.Issue
	var err error
	cached, err := exists("fake_issues.json")
	if err != nil {
		return err
	}
	if cached && false {
		f, err := os.Open("fake_issues.json")
		if err != nil {
			return err
		}

		data, err := ioutil.ReadAll(f)
		if err != nil {
			return err
		}

		err = json.Unmarshal(data, &issues)
		if err != nil {
			return err
		}
	} else {
		rawIssues, err := w.searchIssues("is:open type:issue repo:wercker/sentcli")
		if err != nil {
			return err
		}
		for _, issue := range rawIssues {
			boom := issue
			issues = append(issues, &boom)
		}

		data, err := json.MarshalIndent(issues, "", "  ")
		if err != nil {
			return err
		}
		err = ioutil.WriteFile("fake_issues.json", data, 0666)
		if err != nil {
			return err
		}
	}

	w.issues = issues
	w.currentIssues = issues
	return nil
}

func (w *IssueWindow) Init() error {
	err := w.RefreshIssues()
	if err != nil {
		return err
	}

	// Fake milestones for now
	w.milestones = map[int]int{
		1: 3,
		2: 1,
		3: 2,
	}

	// sort.Sort(w)
	return nil
}

func (w *IssueWindow) Filter(substr string) {
	if substr == "" {
		w.currentIssues = w.issues
		return
	}
	selected := []*github.Issue{}
	for _, issue := range w.issues {
		repo := repoFromURL(*issue.URL)
		if strings.Contains(strings.ToLower(*issue.Title), strings.ToLower(substr)) {
			selected = append(selected, issue)
		} else if strings.Contains(strings.ToLower(repo), strings.ToLower(substr)) {
			selected = append(selected, issue)
		} else if strings.Contains(substr, fmt.Sprintf("#%d", *issue.Number)) {
			selected = append(selected, issue)
		}
	}
	w.currentIssues = selected
}

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

// HandleEvent for keypresses
// up/down: move the cursor
// a: show all issues
// b: show selected
// left/right: unselect/select
func (w *IssueWindow) HandleEvent(ev termbox.Event) {
	switch ev.Type {
	case termbox.EventKey:
		switch ev.Key {
		case termbox.KeyEsc:
			// back out of stuff
			if w.currentMenu != "" {
				w.currentMenu = ""
				break
			}
			if w.currentExpanded != -1 {
				w.currentExpanded = -1
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
			w.currentIndex += 1
			if w.currentIndex >= len(w.currentIssues) {
				w.currentIndex = len(w.currentIssues) - 1
			}
			if w.currentExpanded != -1 {
				w.currentExpanded = w.currentIndex
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
			w.currentIndex -= 1
			if w.currentExpanded != -1 {
				w.currentExpanded = w.currentIndex
			}
			// case termbox.KeyArrowLeft:
			//   name := *((*w.currentIssues)[w.currentIndex].Title)
			//   w.selected[name] = struct{}{}
			// case termbox.KeyArrowRight:
			//   name := *((*w.currentIssues)[w.currentIndex].Title)
			//   delete(w.selected, name)
			return
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
			if w.currentFilter == ":q" {
				termbox.Close()
				os.Exit(0)
			}
			if w.currentExpanded == w.currentIndex {
				w.currentExpanded = -1
			} else {
				w.currentExpanded = w.currentIndex
			}
			return
		}

		if w.currentIndex == -1 {
			// Add to the filter if we have nothing selected
			switch ev.Ch {
			case 0:
			case ' ':
			default:
				w.currentFilter += string(ev.Ch)
				// reset scrollidex
				w.scrollIndex = 0
			}
		} else if w.currentMenu == "" {
			// Try to find the menu item
			// TODO(termie): hardcoded for now
			switch ev.Ch {
			case 'p':
				w.currentMenu = "priority"
			case 't':
				w.currentMenu = "type"
			case 'm':
				w.currentMenu = "milestone"
			}
		} else if w.currentMenu != "" {
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

func (w *IssueWindow) HandlePriorityEvent(ev termbox.Event) {
	issue := w.currentIssues[w.currentIndex]
	labels := issue.Labels
	labelNames := []string{}

	for _, label := range labels {
		if !strings.HasPrefix(*label.Name, "priority:") {
			labelNames = append(labelNames, *label.Name)
		}
	}

	switch ev.Ch {
	case '1':
		// set type bug
		labelNames = append(labelNames, "priority:blocker")
	case '2':
		// set type task
		labelNames = append(labelNames, "priority:critical")
	case '3':
		// set type enhancement
		labelNames = append(labelNames, "priority:normal")
	case '4':
		// set type question
		labelNames = append(labelNames, "priority:low")
	}

	owner := ownerFromURL(*issue.URL)
	repo := repoFromURL(*issue.URL)
	newLabels, _, err := w.client.Issues.ReplaceLabelsForIssue(owner, repo, *issue.Number, labelNames)
	if err != nil {
		panic(err)
	}
	updateIssue := w.FindIssue(*issue.Number)
	updateIssue.Labels = newLabels
}

func (w *IssueWindow) HandleTypeEvent(ev termbox.Event) {
	issue := w.currentIssues[w.currentIndex]
	labels := issue.Labels
	labelNames := []string{}

	for _, label := range labels {
		if !strings.HasPrefix(*label.Name, "type:") {
			labelNames = append(labelNames, *label.Name)
		}
	}

	switch ev.Ch {
	case '1':
		// set type bug
		labelNames = append(labelNames, "type:bug")
	case '2':
		// set type task
		labelNames = append(labelNames, "type:task")
	case '3':
		// set type enhancement
		labelNames = append(labelNames, "type:enhancement")
	case '4':
		// set type question
		labelNames = append(labelNames, "type:question")
	}

	owner := ownerFromURL(*issue.URL)
	repo := repoFromURL(*issue.URL)
	newLabels, _, err := w.client.Issues.ReplaceLabelsForIssue(owner, repo, *issue.Number, labelNames)
	if err != nil {
		panic(err)
	}
	updateIssue := w.FindIssue(*issue.Number)
	updateIssue.Labels = newLabels
}

func (w *IssueWindow) FindIssue(number int) *github.Issue {
	for _, issue := range w.issues {
		if *issue.Number == number {
			return issue
		}
	}
	return nil
}

func (w *IssueWindow) HandleMilestoneEvent(ev termbox.Event) {
	issue := w.currentIssues[w.currentIndex]
	owner := ownerFromURL(*issue.URL)
	repo := repoFromURL(*issue.URL)
	number := *issue.Number
	var milestone int
	switch ev.Ch {
	case '1':
		// set current milestone
		milestone = w.milestones[1]
	case '2':
		// set next milestone
		milestone = w.milestones[2]
	case '3':
		// set someday milestone
		milestone = w.milestones[3]
	default:
		return
	}

	respIssue, _, err := w.client.Issues.Edit(owner, repo, number, &github.IssueRequest{Milestone: &milestone})
	if err != nil {
		panic(err)
	}

	updateIssue := w.FindIssue(number)
	updateIssue.Milestone = respIssue.Milestone

}

func wordWrap(text string, length int) []string {
	s := wordwrap.WrapString(text, uint(length))
	return strings.Split(s, "\n")
}

func repoFromURL(url string) string {
	parts := strings.Split(url, "/")
	return parts[len(parts)-3]
}

func ownerFromURL(url string) string {
	parts := strings.Split(url, "/")
	return parts[len(parts)-4]
}

func issuePriority(issue *github.Issue) int {
	for _, label := range issue.Labels {
		pri := GetPriority(*label.Name)
		if pri != 0 {
			return pri
		}
	}
	return 0
}

func issueType(issue *github.Issue) int {
	for _, label := range issue.Labels {
		t := GetType(*label.Name)
		if t != 0 {
			return t
		}
	}
	return 0
}

func (w *IssueWindow) issueMilestone(issue *github.Issue) int {
	if issue.Milestone == nil {
		return 0
	}
	return w.GetMilestone(*issue.Milestone.URL)
}

func (w *IssueWindow) GetMilestone(url string) int {
	parts := strings.Split(url, "/")
	milestone := parts[len(parts)-1]
	for i, check := range w.milestones {
		if fmt.Sprintf("%d", check) == milestone {
			return i
		}
	}
	return 0
}

func (w *IssueWindow) DrawFilter() {
	cursor := " "
	if w.currentIndex == -1 {
		cursor = ">"
	}
	if w.currentFilter == "" && w.currentIndex == -1 {
		printLine(">filter: (type anything to start filtering, down-arrow to select issue)", w.x, w.y+1)
		return
	}
	printLine(fmt.Sprintf("%sfilter: %s", cursor, w.currentFilter), w.x, w.y+1)
}

func (w *IssueWindow) DrawMenu() {
	if w.currentIndex == -1 {
		return
	}

	if w.currentMenu == "" {
		printLine(fmt.Sprintf("[m]ilestone [p]riority [t]ype [enter] expand/contract"), w.x, w.y)
	}

	if w.currentMenu == "priority" {
		printLine(" priority: [1] blocker [2] critical [3] normal [4] low", w.x, w.y)
	}

	if w.currentMenu == "type" {
		printLine("     type: [1] bug [2] task [3] enhancement [4] question", w.x, w.y)
	}

	if w.currentMenu == "milestone" {
		printLine("milestone: [1] current [2] next [3] someday", w.x, w.y)
	}

}

func (w *IssueWindow) Draw() {
	w.Filter(w.currentFilter)
	w.DrawMenu()
	w.DrawFilter()
	y := 0

	//debug
	// printLine(fmt.Sprintf("ci: %d si: %d li: %d", w.currentIndex, w.scrollIndex, w.lastIndex), 1, 1)

	if w.scrollIndex > 0 {
		y += 1
		printLine("--more--", w.x, 3+y)
	}

	for i, issue := range w.currentIssues {
		if i < w.scrollIndex {
			continue
		}
		y += 1
		// we've reached the edge
		if y >= w.y2-4 {
			if i < len(w.currentIssues) {
				printLine("--more--", w.x, w.y2-1)
			}
			break
		}
		cursor := " "
		selected := " "
		if i == w.currentIndex {
			cursor = ">"
		}
		w.lastIndex = i
		// if _, ok := w.selected[*issue.Title]; ok {
		//   selected = "*"
		// }

		repo := repoFromURL(*issue.URL)[:5]
		pri := issuePriority(issue)
		t := issueType(issue)
		milestone := w.issueMilestone(issue)

		printLine(fmt.Sprintf("%s %d%d%d %s%s/%-4d %s", cursor, milestone, pri, t, selected, repo, *issue.Number, *issue.Title), w.x, 3+y)

		// Check for expanded
		if i == w.currentExpanded {
			lines := wordWrap(*issue.Body, w.x2-9)
			for _, line := range lines {
				y += 1
				printLine(line, 8, 3+y)
			}
		}
	}
}

func (w *IssueWindow) Len() int {
	return len(w.currentIssues)
}

func (w *IssueWindow) Swap(i, j int) {
	(w.currentIssues)[i], (w.currentIssues)[j] = (w.currentIssues)[j], (w.currentIssues)[i]
}

func (w *IssueWindow) Less(i, j int) bool {
	return *(w.currentIssues[i].Title) < *(w.currentIssues[j].Title)
}
