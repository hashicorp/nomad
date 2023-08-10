// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package useragent

import (
	"fmt"
	"net/http"
	"runtime"

	"github.com/hashicorp/nomad/version"
)

const (
	// Header is the User-Agent header key
	// https://www.rfc-editor.org/rfc/rfc7231#section-5.5.3
	Header = `User-Agent`
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

// HeaderSetter is anything that implements SetHeaders(http.Header).
type HeaderSetter interface {
	SetHeaders(http.Header)
}

// SetHeaders configures the User-Agent http.Header for the client.
func SetHeaders(client HeaderSetter) {
	client.SetHeaders(http.Header{
		Header: []string{String()},
	})
}
