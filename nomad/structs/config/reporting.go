// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package config

import (
	"time"

	"github.com/hashicorp/nomad/helper/pointer"
)

type LicenseReportingConfig struct {
	Enabled *bool `hcl:"enabled"`
}

func (lc *LicenseReportingConfig) Copy() *LicenseReportingConfig {
	if lc == nil {
		return nil
	}

	nlc := *lc
	nlc.Enabled = pointer.Copy(lc.Enabled)

	return &nlc
}

func (lc *LicenseReportingConfig) Merge(b *LicenseReportingConfig) *LicenseReportingConfig {
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

type ReportingConfig struct {
	License *LicenseReportingConfig `hcl:"license,block"`

	// ExportAddress overrides the Census license server. This is intended
	// for testing and should not be configured by end-users.
	ExportAddress string `hcl:"address" json:"-"`

	// ExportInterval overrides the default export interval. This is intended
	// for testing and should not be configured by end-users.
	ExportInterval    time.Duration
	ExportIntervalHCL string `hcl:"export_interval" json:"-"`
}

func (r *ReportingConfig) Copy() *ReportingConfig {
	if r == nil {
		return nil
	}

	nr := *r
	nr.License = r.License.Copy()

	return &nr
}

func (r *ReportingConfig) Merge(b *ReportingConfig) *ReportingConfig {
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
	if r.ExportAddress == "" {
		result.ExportAddress = b.ExportAddress
	}
	if r.ExportIntervalHCL == "" {
		result.ExportIntervalHCL = b.ExportIntervalHCL
	}
	if r.ExportInterval == 0 {
		result.ExportInterval = b.ExportInterval
	}

	return &result
}

func DefaultReporting() *ReportingConfig {
	return &ReportingConfig{
		License: &LicenseReportingConfig{},
	}
}
