package main

import (
	"fmt"

	"github.com/codegangsta/cli"
)

var (
	// GitCommit is the git commit hash associated with this build.
	GitCommit = ""

	// MajorVersion is the semver major version.
	MajorVersion = "0"

	// MinorVersion is the semver minor version.
	MinorVersion = "3"

	// PatchVersion is the semver patch version. (use 0 for dev, build process
	// will inject a build number)
	PatchVersion = "0"

	// Compiled is the unix timestamp when this binary got compiled.
	Compiled = ""

	versionCommand = cli.Command{
		Name:  "version",
		Usage: "print version",
		Action: func(c *cli.Context) {
			fmt.Println(Version())
		},
	}
)

// Version returns a semver compatible version for this build.
func Version() string {
	return fmt.Sprintf("%s.%s.%s", MajorVersion, MinorVersion, PatchVersion)
}
