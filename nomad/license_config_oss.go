// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !ent

package nomad

func (c *LicenseConfig) Validate() error {
	return nil
}
