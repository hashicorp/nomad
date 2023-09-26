// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package config

import "github.com/hashicorp/nomad/helper/pointer"

func DefaultReporting() *Reporting {
	return &Reporting{
		&LicenseConfig{},
	}
}

type LicenseConfig struct {
	Enabled *bool `hcl:"enabled"`
}

func (lc *LicenseConfig) Copy() *LicenseConfig {
	if lc == nil {
		return nil
	}

	nlc := *lc
	nlc.Enabled = pointer.Copy(lc.Enabled)

	return &nlc
}

func (lc *LicenseConfig) Merge(b *LicenseConfig) *LicenseConfig {
	if lc == nil {
		return b
	}

	result := *lc

	if b == nil {
		return &result
	}

	if b.Enabled != nil {
		result.Enabled = b.Enabled
	}

	return &result
}

type Reporting struct {
	License *LicenseConfig `hcl:"license,block"`
}

func (r *Reporting) Copy() *Reporting {
	if r == nil {
		return nil
	}

	nr := *r
	nr.License = r.License.Copy()

	return &nr
}

func (r *Reporting) Merge(b *Reporting) *Reporting {
	if r == nil {
		return b
	}

	result := *r

	if b == nil {
		return &result
	}

	if result.License == nil && b.License != nil {
		result.License = b.License
	} else if b.License != nil {
		result.License = result.License.Merge(b.License)
	}

	return &result
}
