// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package fingerprint

import (
	"os"
	"path/filepath"
	"strings"
)

// readDMI reads a single DMI field from the sysfs interface and returns its
// trimmed value. The raw error is returned so callers can distinguish between
// os.ErrNotExist (DMI not available) and os.ErrPermission (agent not root).
func readDMI(field string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dmiBase, field))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}
