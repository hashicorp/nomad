// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !ent
// +build !ent

package nomad

// allocQuota returns the quota object associated with the allocation. In
// anything but Premium this will always be empty
func (n *nomadFSM) allocQuota(_ string) (string, error) {
	return "", nil
}
