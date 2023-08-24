// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/hashicorp/hcl"
	"github.com/hashicorp/hcl/hcl/ast"
	client "github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/mitchellh/mapstructure"
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
		ACL:   &ACLConfig{},
		Audit: &config.AuditConfig{},
		Consul: &config.ConsulConfig{
			ServiceIdentity:  &config.WorkloadIdentity{},
			TemplateIdentity: &config.WorkloadIdentity{},
		},
		Consuls:   map[string]*config.ConsulConfig{},
		Autopilot: &config.AutopilotConfig{},
		Telemetry: &Telemetry{},
		Vault:     &config.VaultConfig{},
		Vaults:    map[string]*config.VaultConfig{},
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

	// Re-parse the file to extract the multiple Vault configurations, which we
	// need to parse by hand because we don't have a label on the block
	root, err := hcl.Parse(buf.String())
	if err != nil {
		return nil, fmt.Errorf("failed to parse HCL file %s: %w", path, err)
	}
	list, ok := root.Node.(*ast.ObjectList)
	if !ok {
		return nil, fmt.Errorf("error parsing: root should be an object")
	}
	matches := list.Filter("vault")
	if len(matches.Items) > 0 {
		if err := parseVaults(c, matches); err != nil {
			return nil, fmt.Errorf("error parsing 'vault': %w", err)
		}
	}
	matches = list.Filter("consul")
	if len(matches.Items) > 0 {
		if err := parseConsuls(c, matches); err != nil {
			return nil, fmt.Errorf("error parsing 'consul': %w", err)
		}
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

	// The `vault` and `consul` blocks are parsed separately from the Decode method, so it
	// will incorrectly report them as extra keys, of which there may be multiple
	c.ExtraKeysHCL = slices.DeleteFunc(c.ExtraKeysHCL, func(s string) bool { return s == "vault" })
	c.ExtraKeysHCL = slices.DeleteFunc(c.ExtraKeysHCL, func(s string) bool { return s == "consul" })
	if len(c.ExtraKeysHCL) == 0 {
		c.ExtraKeysHCL = nil
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

// parseVaults decodes the `vault` blocks. The hcl.Decode method can't parse
// these correctly as HCL1 because they don't have labels, which would result in
// all the blocks getting merged regardless of name.
func parseVaults(c *Config, list *ast.ObjectList) error {
	if len(list.Items) == 0 {
		return nil
	}

	for _, obj := range list.Items {
		var m map[string]interface{}
		if err := hcl.DecodeObject(&m, obj.Val); err != nil {
			return err
		}
		v := &config.VaultConfig{}
		err := mapstructure.WeakDecode(m, v)
		if err != nil {
			return err
		}
		if v.Name == "" {
			v.Name = "default"
		}
		if exist, ok := c.Vaults[v.Name]; ok {
			c.Vaults[v.Name] = exist.Merge(v)
		} else {
			c.Vaults[v.Name] = v
		}
	}

	c.Vault = c.Vaults["default"]
	return nil
}

// parseConsuls decodes the `consul` blocks. The hcl.Decode method can't parse
// these correctly as HCL1 because they don't have labels, which would result in
// all the blocks getting merged regardless of name.
func parseConsuls(c *Config, list *ast.ObjectList) error {
	if len(list.Items) == 0 {
		return nil
	}

	for _, obj := range list.Items {
		var m map[string]interface{}
		if err := hcl.DecodeObject(&m, obj.Val); err != nil {
			return err
		}

		delete(m, "service_identity")
		delete(m, "template_identity")

		cc := &config.ConsulConfig{}
		err := mapstructure.WeakDecode(m, cc)
		if err != nil {
			return err
		}
		if cc.Name == "" {
			cc.Name = "default"
		}
		if cc.TimeoutHCL != "" {
			d, err := time.ParseDuration(cc.TimeoutHCL)
			if err != nil {
				return err
			}
			cc.Timeout = d
		}

		if exist, ok := c.Consuls[cc.Name]; ok {
			c.Consuls[cc.Name] = exist.Merge(cc)
		} else {
			c.Consuls[cc.Name] = cc
		}

		// decode service and template identity blocks
		var listVal *ast.ObjectList
		if ot, ok := obj.Val.(*ast.ObjectType); ok {
			listVal = ot.List
		} else {
			return fmt.Errorf("should be an object")
		}

		if o := listVal.Filter("service_identity"); len(o.Items) > 0 {
			var m map[string]interface{}
			serviceIdentityBlock := o.Items[0]
			if err := hcl.DecodeObject(&m, serviceIdentityBlock.Val); err != nil {
				return err
			}

			var serviceIdentity config.WorkloadIdentity
			if err := mapstructure.WeakDecode(m, &serviceIdentity); err != nil {
				return err
			}
			c.Consuls[cc.Name].ServiceIdentity = &serviceIdentity
		}

		if o := listVal.Filter("template_identity"); len(o.Items) > 0 {
			var m map[string]interface{}
			templateIdentityBlock := o.Items[0]
			if err := hcl.DecodeObject(&m, templateIdentityBlock.Val); err != nil {
				return err
			}

			var templateIdentity config.WorkloadIdentity
			if err := mapstructure.WeakDecode(m, &templateIdentity); err != nil {
				return err
			}
			c.Consuls[cc.Name].TemplateIdentity = &templateIdentity
		}
	}

	c.Consul = c.Consuls["default"]
	return nil
}
