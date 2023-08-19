// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !linux

package cgroupslib

type noop struct{}

func (p *noop) Reserve(*idset.Set[idset.CoreID]) {}

func (p *noop) Release(*idset.Set[idset.CoreID]) {}
