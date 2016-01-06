package main

import "github.com/google/go-github/github"

type API interface {
	Milestones(string) ([]*Milestone, error)
	Search(string) ([]github.Issue, error)
}

type GithubAPI struct {
	client *github.Client
	opts   *Options
	config *Config
}

func NewGithubAPI(client *github.Client, opts *Options, config *Config) *GithubAPI {
	return &GithubAPI{client, opts, config}
}
