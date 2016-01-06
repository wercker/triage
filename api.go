package main

import "github.com/google/go-github/github"

// API is the interface for interacting with the issue tracker
type API interface {
	Milestones(string) ([]*Milestone, error)
	Search(string) ([]github.Issue, error)
}

// GithubAPI is the implementation of the issue tracker interface for Github
type GithubAPI struct {
	client *github.Client
	opts   *Options
	config *Config
}

// NewGithubAPI constructor
func NewGithubAPI(client *github.Client, opts *Options, config *Config) *GithubAPI {
	return &GithubAPI{client, opts, config}
}
