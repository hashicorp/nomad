// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package docker

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// This code is taken from github.com/docker/volume/mounts/windows_parser.go
// See https://github.com/moby/moby/blob/master/LICENSE for the license, Apache License 2.0 at this time.

const (
	// Spec should be in the format [source:]destination[:mode]
	//
	// Examples: c:\foo bar:d:rw
	//           c:\foo:d:\bar
	//           myname:d:
	//           d:\
	//
	// Explanation of this regex! Thanks @thaJeztah on IRC and gist for help. See
	// https://gist.github.com/thaJeztah/6185659e4978789fb2b2. A good place to
	// test is https://regex-golang.appspot.com/assets/html/index.html
	//
	// Useful link for referencing named capturing groups:
	// http://stackoverflow.com/questions/20750843/using-named-matches-from-go-regex
	//
	// There are three match groups: source, destination and mode.
	//

	// rxHostDir is the first option of a source
	rxHostDir = `(?:\\\\\?\\)?[a-z]:[\\/](?:[^\\/:*?"<>|\r\n]+[\\/]?)*`
	// rxName is the second option of a source
	rxName = `[^\\/:*?"<>|\r\n]+\/?.*`

	// RXReservedNames are reserved names not possible on Windows
	rxReservedNames = `(con)|(prn)|(nul)|(aux)|(com[1-9])|(lpt[1-9])`

	// rxPipe is a named path pipe (starts with `\\.\pipe\`, possibly with / instead of \)
	rxPipe = `[/\\]{2}.[/\\]pipe[/\\][^:*?"<>|\r\n]+`
	// rxSource is the combined possibilities for a source
	rxSource = `((?P<source>((` + rxHostDir + `)|(` + rxName + `)|(` + rxPipe + `))):)?`

	// Source. Can be either a host directory, a name, or omitted:
	//  HostDir:
	//    -  Essentially using the folder solution from
	//       https://www.safaribooksonline.com/library/view/regular-expressions-cookbook/9781449327453/ch08s18.html
	//       but adding case insensitivity.
	//    -  Must be an absolute path such as c:\path
	//    -  Can include spaces such as `c:\program files`
	//    -  And then followed by a colon which is not in the capture group
	//    -  And can be optional
	//  Name:
	//    -  Must not contain invalid NTFS filename characters (https://msdn.microsoft.com/en-us/library/windows/desktop/aa365247(v=vs.85).aspx)
	//    -  And then followed by a colon which is not in the capture group
	//    -  And can be optional

	// rxDestination is the regex expression for the mount destination
	rxDestination = `(?P<destination>((?:\\\\\?\\)?([a-z]):((?:[\\/][^\\/:*?"<>\r\n]+)*[\\/]?))|(` + rxPipe + `)|([/].*))`

	// Destination (aka container path):
	//    -  Variation on hostdir but can be a drive followed by colon as well
	//    -  If a path, must be absolute. Can include spaces
	//    -  Drive cannot be c: (explicitly checked in code, not RegEx)

	// rxMode is the regex expression for the mode of the mount
	// Mode (optional):
	//    -  Hopefully self explanatory in comparison to above regex's.
	//    -  Colon is not in the capture group
	rxMode = `(:(?P<mode>(?i)ro|rw))?`
)

func errInvalidSpec(spec string) error {
	return fmt.Errorf("invalid volume specification: '%s'", spec)
}

type fileInfoProvider interface {
	fileInfo(path string) (exist, isDir bool, err error)
}

type defaultFileInfoProvider struct {
}

func (defaultFileInfoProvider) fileInfo(path string) (exist, isDir bool, err error) {
	fi, err := os.Stat(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return false, false, err
		}
		return false, false, nil
	}
	return true, fi.IsDir(), nil
}

var currentFileInfoProvider fileInfoProvider = defaultFileInfoProvider{}

func windowsSplitRawSpec(raw, destRegex string) ([]string, error) {
	specExp := regexp.MustCompile(`^` + rxSource + destRegex + rxMode + `$`)
	match := specExp.FindStringSubmatch(strings.ToLower(raw))

	// Must have something back
	if len(match) == 0 {
		return nil, errInvalidSpec(raw)
	}

	var split []string
	matchgroups := make(map[string]string)
	// Pull out the sub expressions from the named capture groups
	for i, name := range specExp.SubexpNames() {
		matchgroups[name] = strings.ToLower(match[i])
	}
	if source, exists := matchgroups["source"]; exists {
		if source != "" {
			split = append(split, source)
		}
	}
	if destination, exists := matchgroups["destination"]; exists {
		if destination != "" {
			split = append(split, destination)
		}
	}
	if mode, exists := matchgroups["mode"]; exists {
		if mode != "" {
			split = append(split, mode)
		}
	}
	// Fix #26329. If the destination appears to be a file, and the source is null,
	// it may be because we've fallen through the possible naming regex and hit a
	// situation where the user intention was to map a file into a container through
	// a local volume, but this is not supported by the platform.
	if matchgroups["source"] == "" && matchgroups["destination"] != "" {
		volExp := regexp.MustCompile(`^` + rxName + `$`)
		reservedNameExp := regexp.MustCompile(`^` + rxReservedNames + `$`)

		if volExp.MatchString(matchgroups["destination"]) {
			if reservedNameExp.MatchString(matchgroups["destination"]) {
				return nil, fmt.Errorf("volume name %q cannot be a reserved word for Windows filenames", matchgroups["destination"])
			}
		} else {

			exists, isDir, _ := currentFileInfoProvider.fileInfo(matchgroups["destination"])
			if exists && !isDir {
				return nil, fmt.Errorf("file '%s' cannot be mapped. Only directories can be mapped on this platform", matchgroups["destination"])

			}
		}
	}
	return split, nil
}
