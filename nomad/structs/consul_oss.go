// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent
// +build !ent

package structs

func (c *Consul) GetNamespace() string {
	return ""
}
