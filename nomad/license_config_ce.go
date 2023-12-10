// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent

package nomad

func (c *LicenseConfig) Validate() error {
	return nil
}
