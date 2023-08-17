// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !linux

package docker

type CpusetFixer interface {
	Start()
}

func newCpusetFixer(*Driver) CpusetFixer {
	return new(noop)
}

type noop struct {
	// empty
}

func (*noop) Start() {
	// empty
}
