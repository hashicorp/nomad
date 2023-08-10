// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/hcl"
	client "github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs/config"
)

// ParseConfigFile returns an agent.Config from parsed from a file.
func ParseConfigFile(path string) (*Config, error) {
	// slurp
	var buf bytes.Buffer
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if _, err := io.Copy(&buf, f); err != nil {
		return nil, err
	}

	// parse
	c := &Config{
		Client: &ClientConfig{
			ServerJoin: &ServerJoin{},
			TemplateConfig: &client.ClientTemplateConfig{
				Wait:        &client.WaitConfig{},
				WaitBounds:  &client.WaitConfig{},
				ConsulRetry: &client.RetryConfig{},
				VaultRetry:  &client.RetryConfig{},
				NomadRetry:  &client.RetryConfig{},
			},
		},
		Server: &ServerConfig{
			PlanRejectionTracker: &PlanRejectionTracker{},
			ServerJoin:           &ServerJoin{},
		},
		ACL:       &ACLConfig{},
		Audit:     &config.AuditConfig{},
		Consul:    &config.ConsulConfig{},
		Autopilot: &config.AutopilotConfig{},
		Telemetry: &Telemetry{},
		Vault:     &config.VaultConfig{},
	}

	err = hcl.Decode(c, buf.String())
	if err != nil {
		return nil, fmt.Errorf("failed to decode HCL file %s: %w", path, err)
	}

	// convert strings to time.Durations
	tds := []durationConversionMap{
		{"gc_interval", &c.Client.GCInterval, &c.Client.GCIntervalHCL, nil},
		{"acl.token_ttl", &c.ACL.TokenTTL, &c.ACL.TokenTTLHCL, nil},
		{"acl.policy_ttl", &c.ACL.PolicyTTL, &c.ACL.PolicyTTLHCL, nil},
		{"acl.policy_ttl", &c.ACL.RoleTTL, &c.ACL.RoleTTLHCL, nil},
		{"acl.token_min_expiration_ttl", &c.ACL.TokenMinExpirationTTL, &c.ACL.TokenMinExpirationTTLHCL, nil},
		{"acl.token_max_expiration_ttl", &c.ACL.TokenMaxExpirationTTL, &c.ACL.TokenMaxExpirationTTLHCL, nil},
		{"client.server_join.retry_interval", &c.Client.ServerJoin.RetryInterval, &c.Client.ServerJoin.RetryIntervalHCL, nil},
		{"server.heartbeat_grace", &c.Server.HeartbeatGrace, &c.Server.HeartbeatGraceHCL, nil},
		{"server.min_heartbeat_ttl", &c.Server.MinHeartbeatTTL, &c.Server.MinHeartbeatTTLHCL, nil},
		{"server.failover_heartbeat_ttl", &c.Server.FailoverHeartbeatTTL, &c.Server.FailoverHeartbeatTTLHCL, nil},
		{"server.plan_rejection_tracker.node_window", &c.Server.PlanRejectionTracker.NodeWindow, &c.Server.PlanRejectionTracker.NodeWindowHCL, nil},
		{"server.retry_interval", &c.Server.RetryInterval, &c.Server.RetryIntervalHCL, nil},
		{"server.server_join.retry_interval", &c.Server.ServerJoin.RetryInterval, &c.Server.ServerJoin.RetryIntervalHCL, nil},
		{"consul.timeout", &c.Consul.Timeout, &c.Consul.TimeoutHCL, nil},
		{"autopilot.server_stabilization_time", &c.Autopilot.ServerStabilizationTime, &c.Autopilot.ServerStabilizationTimeHCL, nil},
		{"autopilot.last_contact_threshold", &c.Autopilot.LastContactThreshold, &c.Autopilot.LastContactThresholdHCL, nil},
		{"telemetry.collection_interval", &c.Telemetry.collectionInterval, &c.Telemetry.CollectionInterval, nil},
		{"client.template.block_query_wait", nil, &c.Client.TemplateConfig.BlockQueryWaitTimeHCL,
			func(d *time.Duration) {
				c.Client.TemplateConfig.BlockQueryWaitTime = d
			},
		},
		{"client.template.max_stale", nil, &c.Client.TemplateConfig.MaxStaleHCL,
			func(d *time.Duration) {
				c.Client.TemplateConfig.MaxStale = d
			}},
		{"client.template.wait.min", nil, &c.Client.TemplateConfig.Wait.MinHCL,
			func(d *time.Duration) {
				c.Client.TemplateConfig.Wait.Min = d
			},
		},
		{"client.template.wait.max", nil, &c.Client.TemplateConfig.Wait.MaxHCL,
			func(d *time.Duration) {
				c.Client.TemplateConfig.Wait.Max = d
			},
		},
		{"client.template.wait_bounds.min", nil, &c.Client.TemplateConfig.WaitBounds.MinHCL,
			func(d *time.Duration) {
				c.Client.TemplateConfig.WaitBounds.Min = d
			},
		},
		{"client.template.wait_bounds.max", nil, &c.Client.TemplateConfig.WaitBounds.MaxHCL,
			func(d *time.Duration) {
				c.Client.TemplateConfig.WaitBounds.Max = d
			},
		},
		{"client.template.consul_retry.backoff", nil, &c.Client.TemplateConfig.ConsulRetry.BackoffHCL,
			func(d *time.Duration) {
				c.Client.TemplateConfig.ConsulRetry.Backoff = d
			},
		},
		{"client.template.consul_retry.max_backoff", nil, &c.Client.TemplateConfig.ConsulRetry.MaxBackoffHCL,
			func(d *time.Duration) {
				c.Client.TemplateConfig.ConsulRetry.MaxBackoff = d
			},
		},
		{"client.template.vault_retry.backoff", nil, &c.Client.TemplateConfig.VaultRetry.BackoffHCL,
			func(d *time.Duration) {
				c.Client.TemplateConfig.VaultRetry.Backoff = d
			},
		},
		{"client.template.vault_retry.max_backoff", nil, &c.Client.TemplateConfig.VaultRetry.MaxBackoffHCL,
			func(d *time.Duration) {
				c.Client.TemplateConfig.VaultRetry.MaxBackoff = d
			},
		},
		{"client.template.nomad_retry.backoff", nil, &c.Client.TemplateConfig.NomadRetry.BackoffHCL,
			func(d *time.Duration) {
				c.Client.TemplateConfig.NomadRetry.Backoff = d
			},
		},
		{"client.template.nomad_retry.max_backoff", nil, &c.Client.TemplateConfig.NomadRetry.MaxBackoffHCL,
			func(d *time.Duration) {
				c.Client.TemplateConfig.NomadRetry.MaxBackoff = d
			},
		},
	}

	// Add enterprise audit sinks for time.Duration parsing
	for i, sink := range c.Audit.Sinks {
		tds = append(tds, durationConversionMap{
			fmt.Sprintf("audit.sink.%d", i), &sink.RotateDuration, &sink.RotateDurationHCL, nil})
	}

	// convert strings to time.Durations
	err = convertDurations(tds)
	if err != nil {
		return nil, err
	}

	// report unexpected keys
	err = extraKeys(c)
	if err != nil {
		return nil, err
	}

	// Set client template config or its members to nil if not set.
	finalizeClientTemplateConfig(c)

	return c, nil
}

// durationConversionMap holds args for one duration conversion
type durationConversionMap struct {
	targetFieldPath string
	targetField     *time.Duration
	sourceField     *string
	setFunc         func(*time.Duration)
}

// convertDurations parses the duration strings specified in the config files
// into time.Durations
func convertDurations(xs []durationConversionMap) error {
	for _, x := range xs {
		// if targetField is not a pointer itself, use the field map.
		if x.targetField != nil && x.sourceField != nil && "" != *x.sourceField {
			d, err := time.ParseDuration(*x.sourceField)
			if err != nil {
				return fmt.Errorf("%s can't parse time duration %s", x.targetFieldPath, *x.sourceField)
			}

			*x.targetField = d
		} else if x.setFunc != nil && x.sourceField != nil && "" != *x.sourceField {
			// if targetField is a pointer itself, use the setFunc closure.
			d, err := time.ParseDuration(*x.sourceField)
			if err != nil {
				return fmt.Errorf("%s can't parse time duration %s", x.targetFieldPath, *x.sourceField)
			}
			x.setFunc(&d)
		}
	}

	return nil
}

func extraKeys(c *Config) error {
	// hcl leaves behind extra keys when parsing JSON. These keys
	// are kept on the top level, taken from slices or the keys of
	// structs contained in slices. Clean up before looking for
	// extra keys.
	for range c.HTTPAPIResponseHeaders {
		helper.RemoveEqualFold(&c.ExtraKeysHCL, "http_api_response_headers")
	}

	for _, p := range c.Plugins {
		helper.RemoveEqualFold(&c.ExtraKeysHCL, p.Name)
		helper.RemoveEqualFold(&c.ExtraKeysHCL, "config")
		helper.RemoveEqualFold(&c.ExtraKeysHCL, "plugin")
	}

	for _, k := range []string{"options", "meta", "chroot_env", "servers", "server_join"} {
		helper.RemoveEqualFold(&c.ExtraKeysHCL, k)
		helper.RemoveEqualFold(&c.ExtraKeysHCL, "client")
	}

	// stats is an unused key, continue to silently ignore it
	helper.RemoveEqualFold(&c.Client.ExtraKeysHCL, "stats")

	// Remove HostVolume extra keys
	for _, hv := range c.Client.HostVolumes {
		helper.RemoveEqualFold(&c.Client.ExtraKeysHCL, hv.Name)
		helper.RemoveEqualFold(&c.Client.ExtraKeysHCL, "host_volume")
	}

	// Remove HostNetwork extra keys
	for _, hn := range c.Client.HostNetworks {
		helper.RemoveEqualFold(&c.Client.ExtraKeysHCL, hn.Name)
		helper.RemoveEqualFold(&c.Client.ExtraKeysHCL, "host_network")
	}

	// Remove AuditConfig extra keys
	for _, f := range c.Audit.Filters {
		helper.RemoveEqualFold(&c.Audit.ExtraKeysHCL, f.Name)
		helper.RemoveEqualFold(&c.Audit.ExtraKeysHCL, "filter")
	}

	for _, s := range c.Audit.Sinks {
		helper.RemoveEqualFold(&c.Audit.ExtraKeysHCL, s.Name)
		helper.RemoveEqualFold(&c.Audit.ExtraKeysHCL, "sink")
	}

	for _, k := range []string{"enabled_schedulers", "start_join", "retry_join", "server_join"} {
		helper.RemoveEqualFold(&c.ExtraKeysHCL, k)
		helper.RemoveEqualFold(&c.ExtraKeysHCL, "server")
	}

	for _, k := range []string{"preemption_config"} {
		helper.RemoveEqualFold(&c.Server.ExtraKeysHCL, k)
	}

	for _, k := range []string{"datadog_tags"} {
		helper.RemoveEqualFold(&c.ExtraKeysHCL, k)
		helper.RemoveEqualFold(&c.ExtraKeysHCL, "telemetry")
	}

	return helper.UnusedKeys(c)
}

// hcl.Decode will error if the ClientTemplateConfig isn't initialized with empty
// structs, however downstream code expect nils if the struct only contains fields
// with the zero value for its type. This function nils out type members that are
// structs where all the member fields are just the zero value for its type.
func finalizeClientTemplateConfig(config *Config) {
	if config.Client.TemplateConfig.Wait.IsEmpty() {
		config.Client.TemplateConfig.Wait = nil
	}

	if config.Client.TemplateConfig.WaitBounds.IsEmpty() {
		config.Client.TemplateConfig.WaitBounds = nil
	}

	if config.Client.TemplateConfig.ConsulRetry.IsEmpty() {
		config.Client.TemplateConfig.ConsulRetry = nil
	}

	if config.Client.TemplateConfig.VaultRetry.IsEmpty() {
		config.Client.TemplateConfig.VaultRetry = nil
	}

	if config.Client.TemplateConfig.NomadRetry.IsEmpty() {
		config.Client.TemplateConfig.NomadRetry = nil
	}

	if config.Client.TemplateConfig.IsEmpty() {
		config.Client.TemplateConfig = nil
	}
}
