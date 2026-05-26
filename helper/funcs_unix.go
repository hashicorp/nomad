// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris
// +build darwin dragonfly freebsd linux netbsd openbsd solaris

package helper

import "os"

func IsExecutable(i os.FileInfo) bool {
	return !i.IsDir() && i.Mode()&0o111 != 0
}
