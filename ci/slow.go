// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package ci

import (
	"os"
	"strconv"
	"testing"
)

// SkipSlow skips a slow test unless NOMAD_SLOW_TEST is set to a true value.
func SkipSlow(t *testing.T, reason string) {
	value := os.Getenv("NOMAD_SLOW_TEST")
	run, err := strconv.ParseBool(value)
	if !run || err != nil {
		t.Skipf("Skipping slow test: %s", reason)
	}
}

// Parallel runs t in parallel, unless CI is set to a true value.
//
// In CI (CircleCI / GitHub Actions) we get better performance by running tests
// in serial while not restricting GOMAXPROCS.
func Parallel(t *testing.T) {
	value := os.Getenv("CI")
	isCI, err := strconv.ParseBool(value)
	if !isCI || err != nil {
		t.Parallel()
	}
}

// TinyChroot is useful for testing, where we do not use anything other than
// trivial /bin commands like sleep and sh. Copying a minimal chroot helps in
// environments like GHA with very poor [network] disk performance.
//
// Note that you cannot chroot a symlink.
//
// Do not modify this value.
var TinyChroot = map[string]string{
	// destination: /bin
	"/usr/bin/sleep": "/bin/sleep",
	"/usr/bin/dash":  "/bin/sh",
	"/usr/bin/bash":  "/bin/bash",
	"/usr/bin/cat":   "/bin/cat",

	// destination: /usr/bin
	"/usr/bin/stty":   "/usr/bin/stty",
	"/usr/bin/head":   "/usr/bin/head",
	"/usr/bin/mktemp": "/usr/bin/mktemp",
	"/usr/bin/echo":   "/usr/bin/echo",
	"/usr/bin/touch":  "/usr/bin/touch",
	"/usr/bin/stat":   "/usr/bin/stat",

	// destination: /etc/
	"/etc/ld.so.cache":  "/etc/ld.so.cache",
	"/etc/ld.so.conf":   "/etc/ld.so.conf",
	"/etc/ld.so.conf.d": "/etc/ld.so.conf.d",
	"/etc/passwd":       "/etc/passwd",
	"/etc/resolv.conf":  "/etc/resolv.conf",

	// others
	"/lib":                 "/lib",
	"/lib32":               "/lib32",
	"/lib64":               "/lib64",
	"/usr/lib/jvm":         "/usr/lib/jvm",
	"/run/resolvconf":      "/run/resolvconf",
	"/run/systemd/resolve": "/run/systemd/resolve",
}
