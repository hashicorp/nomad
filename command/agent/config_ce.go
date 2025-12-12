// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent
// +build !ent

package agent

// DefaultEntConfig is an empty config in open source
func DefaultEntConfig() *Config {
	return &Config{}
}
