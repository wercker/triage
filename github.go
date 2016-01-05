package main

import "strings"

func ownerRepo(s string) (string, string, error) {
	parts := strings.Split(s, "/")
	return parts[0], parts[1], nil
}
