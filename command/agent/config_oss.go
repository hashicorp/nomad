// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !ent
// +build !ent

package agent

// DefaultEntConfig is an empty config in open source
func DefaultEntConfig() *Config {
	return &Config{}
}
