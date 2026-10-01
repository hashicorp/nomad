// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: MPL-2.0

package flock

import (
	"errors"
	"os"
)

var ErrLocked = errors.New("file locked")

func FLock(file *os.File) error {
	return flock(file)
}

func FUnlock(file *os.File) error {
	return funlock(file)
}
