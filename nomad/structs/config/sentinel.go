// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package config

import (
	"slices"

	"github.com/hashicorp/nomad/helper"
)

// SentinelConfig is configuration specific to Sentinel
type SentinelConfig struct {
	// Imports are configured external imports
	Imports []*SentinelImport `hcl:"import,expand"`

	// AdditionalEnabledModules specifies allowing "stdlib" imports that are
	// normally disallowed. We currently enable all Sentinel's standard imports
	// except "http". In the future if any new imports aren't automatically
	// enabled, users can override that here
	AdditionalEnabledModules []string `hcl:"additional_enabled_modules"`
}

func (s *SentinelConfig) Copy() *SentinelConfig {
	if s == nil {
		return nil
	}

	ns := *s
	ns.Imports = helper.CopySlice(s.Imports)
	ns.AdditionalEnabledModules = slices.Clone(s.AdditionalEnabledModules)
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

// Merge is used to merge two Sentinel configs together. All slice fields are
// combined.
func (s *SentinelConfig) Merge(b *SentinelConfig) *SentinelConfig {
	result := *s
	if len(b.Imports) > 0 {
		result.Imports = append(result.Imports, b.Imports...)
	}
	if len(b.AdditionalEnabledModules) > 0 {
		result.AdditionalEnabledModules = append(
			result.AdditionalEnabledModules, b.AdditionalEnabledModules...)
	}
	return &result
}
