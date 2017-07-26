package main

import (
	"bytes"
	"fmt"
)

// The git commit that was compiled. This will be filled in by the compiler.
var GitCommit string
var GitDescribe string

// The main version number that is being run at the moment.
const Version = "0.6.0"

// A pre-release marker for the version. If this is "" (empty string)
// then it means that it is a final release. Otherwise, this is a pre-release
// such as "dev" (in development), "beta", "rc1", etc.
const VersionPrerelease = ""

// GetVersionParts returns the Nomad version strings. Printing of the Nomad
// version should be used in conjunction with the PrettyVersion method.
func GetVersionParts() (rev, ver, rel string) {
	ver = Version
	rel = VersionPrerelease
	if GitDescribe != "" {
		ver = GitDescribe
		// Trim off a leading 'v', we append it anyways.
		if ver[0] == 'v' {
			ver = ver[1:]
		}
	}
	if GitDescribe == "" && rel == "" && VersionPrerelease != "" {
		rel = "dev"
	}

	return GitCommit, ver, rel
}

// PrettyVersion takes the version parts and formats it in a human readable
// string.
func PrettyVersion(revision, version, versionPrerelease string) string {
	var versionString bytes.Buffer

	fmt.Fprintf(&versionString, "Nomad v%s", version)
	if versionPrerelease != "" {
		fmt.Fprintf(&versionString, "-%s", versionPrerelease)

		if revision != "" {
			fmt.Fprintf(&versionString, " (%s)", revision)
		}
	}

	return versionString.String()
}
