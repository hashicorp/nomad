// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package config

import (
	"slices"

	"github.com/hashicorp/nomad/helper"
)

// SentinelConfig is configuration specific to Sentinel
type SentinelConfig struct {
	// Imports are the configured imports
	Imports []*SentinelImport `hcl:"import,expand"`
}

func (s *SentinelConfig) Copy() *SentinelConfig {
	if s == nil {
		return nil
	}

	ns := *s
	ns.Imports = helper.CopySlice(s.Imports)
	return &ns
}

// SentinelImport is used per configured import
type SentinelImport struct {
	Name string   `hcl:",key"`
	Path string   `hcl:"path"`
	Args []string `hcl:"args"`
}

func (s *SentinelImport) Copy() *SentinelImport {
	if s == nil {
		return nil
	}

	ns := *s
	ns.Args = slices.Clone(s.Args)
	return &ns
}

// Merge is used to merge two Sentinel configs together. The settings from the input always take precedence.
func (a *SentinelConfig) Merge(b *SentinelConfig) *SentinelConfig {
	result := *a
	if len(b.Imports) > 0 {
		result.Imports = append(result.Imports, b.Imports...)
	}
	return &result
}
