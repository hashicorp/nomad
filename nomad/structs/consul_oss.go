// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !ent
// +build !ent

package structs

func (c *Consul) GetNamespace() string {
	return ""
}
