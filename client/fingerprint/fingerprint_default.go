// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build darwin || dragonfly || freebsd || netbsd || openbsd || solaris || windows
// +build darwin dragonfly freebsd netbsd openbsd solaris windows

package fingerprint

func initPlatformFingerprints(fps map[string]Factory) {
}
