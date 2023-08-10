// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package cgroupslib

// Mode indicates whether the Client node is configured with cgroups v1 or v2,
// or is not configured with cgroups enabled.
type Mode byte

const (
	OFF = iota
	CG1
	CG2
)
