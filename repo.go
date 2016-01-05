package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/google/go-github/github"
	"github.com/nsf/termbox-go"
)

type RepoWindow struct {
	client *github.Client
	repos  []github.Repository
	// selected      map[string]struct{}
	// selectedRepos []github.Repository
	currentIndex  int
	currentRepos  []github.Repository
	currentFilter string
	// currentMode   string
}

func NewRepoWindow(client *github.Client) *RepoWindow {
	return &RepoWindow{
		client: client,
		// selected:     map[string]struct{}{},
		currentIndex: 0,
	}
}

func (w *RepoWindow) ID() string {
	return "repos"
}

func (w *RepoWindow) Init() error {
	repos, _, err := w.client.Repositories.ListByOrg(
		"wercker",
		&github.RepositoryListByOrgOptions{Type: "private"},
	)
	if err != nil {
		return err
	}
	w.repos = repos
	w.currentRepos = repos
	sort.Sort(w)
	return nil
}

func (w *RepoWindow) Filter(substr string) {
	if substr == "" {
		w.currentRepos = w.repos
		return
	}
	selected := []github.Repository{}
	for _, repo := range w.repos {
		if strings.Contains(*repo.Name, substr) {
			selected = append(selected, repo)
		}
	}
	w.currentRepos = selected
}

// HandleEvent for keypresses
// up/down: move the cursor
// a: show all repos
// b: show selected
// left/right: unselect/select
func (w *RepoWindow) HandleEvent(ev termbox.Event) {
	switch ev.Type {
	case termbox.EventKey:
		switch ev.Key {
		case termbox.KeyArrowDown:
			w.currentIndex += 1
		case termbox.KeyArrowUp:
			w.currentIndex -= 1
			// case termbox.KeyArrowLeft:
			//   name := *((*w.currentRepos)[w.currentIndex].Name)
			//   w.selected[name] = struct{}{}
			// case termbox.KeyArrowRight:
			//   name := *((*w.currentRepos)[w.currentIndex].Name)
			//   delete(w.selected, name)

		// Backspace starts clearing our filter
		case termbox.KeyBackspace:
			if len(w.currentFilter) > 0 {
				w.currentFilter = w.currentFilter[:len(w.currentFilter)-1]
			}
		case termbox.KeyEnter:
			// Go to the issues page for this repo
		}

		switch ev.Ch {
		case 0:
		case ' ':
		default:
			w.currentFilter += string(ev.Ch)
		}
	}
}

func (w *RepoWindow) Draw() {
	w.Filter(w.currentFilter)
	printLine(fmt.Sprintf("filter: %s", w.currentFilter), 3, 2)
	for i, repo := range w.currentRepos {
		cursor := " "
		selected := " "
		if i == w.currentIndex {
			cursor = ">"
		}
		// if _, ok := w.selected[*repo.Name]; ok {
		//   selected = "*"
		// }
		printLine(fmt.Sprintf("%s %s%s", cursor, selected, *repo.Name), 3, 3+i)
	}
}

func (w *RepoWindow) Len() int {
	return len(w.currentRepos)
}

func (w *RepoWindow) Swap(i, j int) {
	(w.currentRepos)[i], (w.currentRepos)[j] = (w.currentRepos)[j], (w.currentRepos)[i]
}

func (w *RepoWindow) Less(i, j int) bool {
	return *(w.currentRepos[i].Name) < *(w.currentRepos[j].Name)
}
