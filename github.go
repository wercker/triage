package main

import "strings"

func ownerRepo(s string) (string, string, error) {
	parts := strings.Split(s, "/")
	return parts[0], parts[1], nil
}

func ownerRepoFromURL(url string) (string, string, error) {
	parts := strings.Split(url, "/")
	return parts[len(parts)-4], parts[len(parts)-3], nil
}
