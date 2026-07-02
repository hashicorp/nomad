// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: MPL-2.0

//go:build !linux

package flock

import "os"

func flock(file *os.File) error   { return nil }
func funlock(file *os.File) error { return nil }
