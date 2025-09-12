// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build windows
// +build windows

package helper

import (
	"os"
	"strings"
)

func IsExecutable(i os.FileInfo) bool {
	return strings.HasSuffix(i.Name(), ".exe")
}
