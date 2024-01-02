// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

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
