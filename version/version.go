// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package version

import (
	"bytes"
	"fmt"
	"time"
)

var (
	// BuildDate is the time of the git commit used to build the program,
	// in RFC3339 format. It is filled in by the compiler via makefile.
	BuildDate string

	// The git commit that was compiled. This will be filled in by the compiler.
	GitCommit   string
	GitDescribe string

	// The main version number that is being run at the moment.
	Version = "1.10.4"

	// A pre-release marker for the version. If this is "" (empty string)
	// then it means that it is a final release. Otherwise, this is a pre-release
	// such as "dev" (in development), "beta", "rc1", etc.
	VersionPrerelease = ""

	// VersionMetadata is metadata further describing the build type.
	VersionMetadata = ""
)

// VersionInfo
type VersionInfo struct {
	BuildDate         time.Time
	Revision          string
	Version           string
	VersionPrerelease string
	VersionMetadata   string
}

func (v *VersionInfo) Copy() *VersionInfo {
	if v == nil {
		return nil
	}

	nv := *v
	return &nv
}

func GetVersion() *VersionInfo {
	ver := Version
	rel := VersionPrerelease
	md := VersionMetadata
	if GitDescribe != "" {
		ver = GitDescribe
	}
	if GitDescribe == "" && rel == "" && VersionPrerelease != "" {
		rel = "dev"
	}

	// on parse error, will be zero value time.Time{}
	built, _ := time.Parse(time.RFC3339, BuildDate)

	return &VersionInfo{
		BuildDate:         built,
		Revision:          GitCommit,
		Version:           ver,
		VersionPrerelease: rel,
		VersionMetadata:   md,
	}
}

func (v *VersionInfo) VersionNumber() string {
	version := v.Version

	if v.VersionPrerelease != "" {
		version = fmt.Sprintf("%s-%s", version, v.VersionPrerelease)
	}

	if v.VersionMetadata != "" {
		version = fmt.Sprintf("%s+%s", version, v.VersionMetadata)
	}

	return version
}

func (v *VersionInfo) FullVersionNumber(rev bool) string {
	var versionString bytes.Buffer

	fmt.Fprintf(&versionString, "Nomad v%s", v.Version)
	if v.VersionPrerelease != "" {
		fmt.Fprintf(&versionString, "-%s", v.VersionPrerelease)
	}

	if v.VersionMetadata != "" {
		fmt.Fprintf(&versionString, "+%s", v.VersionMetadata)
	}

	if !v.BuildDate.IsZero() {
		fmt.Fprintf(&versionString, "\nBuildDate %s", v.BuildDate.Format(time.RFC3339))
	}

	if rev && v.Revision != "" {
		fmt.Fprintf(&versionString, "\nRevision %s", v.Revision)
	}

	return versionString.String()
}
