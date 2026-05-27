// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build !linux

package agent

import (
	"io"
)

func openNotify() (io.WriteCloser, error) {
	return nil, nil
}

func sdNotify(_ io.Writer, _ string) {}

func sdNotifyReloading(_ io.Writer) {}
