// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build linux

package proclib

var _ ProcessWrangler = (*LinuxWranglerCG2)(nil)
