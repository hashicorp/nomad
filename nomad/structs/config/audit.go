// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package config

import (
	"slices"
	"time"

	"github.com/hashicorp/nomad/helper/pointer"
)

// AuditConfig is the configuration specific to Audit Logging
type AuditConfig struct {
	// Enabled controls the Audit Logging mode
	Enabled *bool `hcl:"enabled"`

	// Sinks configure output sinks for audit logs
	Sinks []*AuditSink `hcl:"sink"`

	// Filters configure audit event filters to filter out certain eevents
	// from being written to a sink.
	Filters []*AuditFilter `hcl:"filter"`

	// ExtraKeysHCL is used by hcl to surface unexpected keys
	ExtraKeysHCL []string `hcl:",unusedKeys" json:"-"`
}

type AuditSink struct {
	// Name is a unique name given to the filter
	Name string `hcl:",key"`

	// DeliveryGuarantee is the level at which delivery of logs must
	// be met in order to successfully make requests
	DeliveryGuarantee string `hcl:"delivery_guarantee"`

	// Type is the sink type to configure. (file)
	Type string `hcl:"type"`

	// Format is the sink output format. (json)
	Format string `hcl:"format"`

	// FileName is the name that the audit log should follow.
	// If rotation is enabled the pattern will be name-timestamp.log
	Path string `hcl:"path"`

	// RotateDuration is the time period that logs should be rotated in
	RotateDuration    time.Duration
	RotateDurationHCL string `hcl:"rotate_duration" json:"-"`

	// RotateBytes is the max number of bytes that should be written to a file
	RotateBytes int `hcl:"rotate_bytes"`

	// RotateMaxFiles is the max number of log files to keep
	RotateMaxFiles int `hcl:"rotate_max_files"`

	// Mode is the octal formatted permissions for the audit log files.
	Mode string `hcl:"mode"`
}

// AuditFilter is the configuration for a Audit Log Filter
type AuditFilter struct {
	// Name is a unique name given to the filter
	Name string `hcl:",key"`

	// Type of auditing event to filter, such as HTTPEvent
	Type string `hcl:"type"`

	// Endpoints is the list of endpoints to include in the filter
	Endpoints []string `hcl:"endpoints"`

	// State is the auditing request lifecycle stage to filter
	Stages []string `hcl:"stages"`

	// Operations is the type of operation to filter, such as GET, DELETE
	Operations []string `hcl:"operations"`
}

// Copy returns a new copy of an AuditConfig
func (a *AuditConfig) Copy() *AuditConfig {
	if a == nil {
		return nil
	}

	nc := new(AuditConfig)
	*nc = *a

	// Copy bool pointers
	if a.Enabled != nil {
		nc.Enabled = pointer.Of(*a.Enabled)
	}

	// Copy Sinks and Filters
	nc.Sinks = copySliceAuditSink(nc.Sinks)
	nc.Filters = copySliceAuditFilter(nc.Filters)

	return nc
}

// Merge is used to merge two Audit Configs together. Settings from the input take precedence.
func (a *AuditConfig) Merge(b *AuditConfig) *AuditConfig {
	result := a.Copy()

	if b.Enabled != nil {
		result.Enabled = pointer.Of(*b.Enabled)
	}

	// Merge Sinks
	if len(a.Sinks) == 0 && len(b.Sinks) != 0 {
		result.Sinks = copySliceAuditSink(b.Sinks)
	} else if len(b.Sinks) != 0 {
		result.Sinks = auditSinkSliceMerge(a.Sinks, b.Sinks)
	}

	// Merge Filters
	if len(a.Filters) == 0 && len(b.Filters) != 0 {
		result.Filters = copySliceAuditFilter(b.Filters)
	} else if len(b.Filters) != 0 {
		result.Filters = auditFilterSliceMerge(a.Filters, b.Filters)
	}

	return result
}

func (a *AuditSink) Copy() *AuditSink {
	if a == nil {
		return nil
	}

	nc := new(AuditSink)
	*nc = *a

	return nc
}

func (a *AuditFilter) Copy() *AuditFilter {
	if a == nil {
		return nil
	}

	nc := new(AuditFilter)
	*nc = *a

	// Copy slices
	nc.Endpoints = slices.Clone(nc.Endpoints)
	nc.Stages = slices.Clone(nc.Stages)
	nc.Operations = slices.Clone(nc.Operations)

	return nc
}

func copySliceAuditFilter(a []*AuditFilter) []*AuditFilter {
	l := len(a)
	if l == 0 {
		return nil
	}

	ns := make([]*AuditFilter, l)
	for idx, cfg := range a {
		ns[idx] = cfg.Copy()
	}

	return ns
}

func auditFilterSliceMerge(a, b []*AuditFilter) []*AuditFilter {
	n := make([]*AuditFilter, len(a))
	seenKeys := make(map[string]int, len(a))

	for i, config := range a {
		n[i] = config.Copy()
		seenKeys[config.Name] = i
	}

	for _, config := range b {
		if fIndex, ok := seenKeys[config.Name]; ok {
			n[fIndex] = config.Copy()
			continue
		}

		n = append(n, config.Copy())
	}

	return n
}

func copySliceAuditSink(a []*AuditSink) []*AuditSink {
	l := len(a)
	if l == 0 {
		return nil
	}

	ns := make([]*AuditSink, l)
	for idx, cfg := range a {
		ns[idx] = cfg.Copy()
	}

	return ns
}

func auditSinkSliceMerge(a, b []*AuditSink) []*AuditSink {
	n := make([]*AuditSink, len(a))
	seenKeys := make(map[string]int, len(a))

	for i, config := range a {
		n[i] = config.Copy()
		seenKeys[config.Name] = i
	}

	for _, config := range b {
		if fIndex, ok := seenKeys[config.Name]; ok {
			n[fIndex] = config.Copy()
			continue
		}

		n = append(n, config.Copy())
	}

	return n
}
