// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package proclib

var _ ProcessWrangler = (*LinuxWranglerCG2)(nil)
