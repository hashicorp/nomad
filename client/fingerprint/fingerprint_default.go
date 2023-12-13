// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build darwin || dragonfly || freebsd || netbsd || openbsd || solaris || windows
// +build darwin dragonfly freebsd netbsd openbsd solaris windows

package fingerprint

func initPlatformFingerprints(fps map[string]Factory) {
}
