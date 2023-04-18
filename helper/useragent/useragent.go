package useragent

import (
	"fmt"
	"runtime"

	"github.com/hashicorp/nomad/version"
)

var (
	// projectURL is the project URL.
	projectURL = "https://www.nomadproject.io/"

	// rt is the runtime - variable for tests.
	rt = runtime.Version()

	// versionFunc is the func that returns the current version. This is a
	// function to take into account the different build processes and distinguish
	// between enterprise and oss builds.
	versionFunc = func() string {
		return version.GetVersion().VersionNumber()
	}
)

// String returns the consistent user-agent string for Nomad.
func String() string {
	return fmt.Sprintf("Nomad/%s (+%s; %s)",
		versionFunc(), projectURL, rt)
}
