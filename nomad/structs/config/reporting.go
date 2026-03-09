// Copyright IBM Corp. 2015, 2025
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

	// SnapshotRetentionTime overrides the default time we retain utilization
	// snapshots in Raft.
	SnapshotRetentionTime    time.Duration
	SnapshotRetentionTimeHCL string `hcl:"snapshot_retention_time"`

	// DisableUsageReporting disables reporting detailed product usage information.
	// This does not disable license reporting.
	DisableUsageReporting *bool `hcl:"disable_product_usage_reporting"`

	// NonProduction is set on the server config
	NonProduction bool
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

	if b.NonProduction {
		result.NonProduction = true
	}

	if result.License == nil && b.License != nil {
		result.License = b.License
	} else if b.License != nil {
		result.License = result.License.Merge(b.License)
	}
	if b.ExportAddress != "" {
		result.ExportAddress = b.ExportAddress
	}
	if b.ExportIntervalHCL != "" {
		result.ExportIntervalHCL = b.ExportIntervalHCL
	}
	if b.ExportInterval != 0 {
		result.ExportInterval = b.ExportInterval
	}
	if b.SnapshotRetentionTime != 0 {
		result.SnapshotRetentionTime = b.SnapshotRetentionTime
	}
	if b.SnapshotRetentionTimeHCL != "" {
		result.SnapshotRetentionTimeHCL = b.SnapshotRetentionTimeHCL
	}
	if b.DisableUsageReporting != nil {
		result.DisableUsageReporting = b.DisableUsageReporting
	}

	return &result
}

func DefaultReporting() *ReportingConfig {
	return &ReportingConfig{
		License: &LicenseReportingConfig{},
	}
}
