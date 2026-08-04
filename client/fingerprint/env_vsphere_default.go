// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build !linux

package fingerprint

import "os"

// readDMI returns os.ErrNotExist on non-Linux platforms. The DMI sysfs
// interface is Linux-only, so the probe will call wrapProbeError and the
// fingerprinter will be silently skipped on Darwin, Windows, etc.
func readDMI(field string) (string, error) {
	return "", os.ErrNotExist
}
