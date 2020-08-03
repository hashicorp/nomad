package agent

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/hcl"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs/config"
)

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
		Client:    &ClientConfig{ServerJoin: &ServerJoin{}},
		ACL:       &ACLConfig{},
		Audit:     &config.AuditConfig{},
		Server:    &ServerConfig{ServerJoin: &ServerJoin{}},
		Consul:    &config.ConsulConfig{},
		Autopilot: &config.AutopilotConfig{},
		Telemetry: &Telemetry{},
		Vault:     &config.VaultConfig{},
	}

	err = hcl.Decode(c, buf.String())
	if err != nil {
		return nil, err
	}

	// convert strings to time.Durations
	tds := []td{
		{"gc_interval", &c.Client.GCInterval, &c.Client.GCIntervalHCL},
		{"acl.token_ttl", &c.ACL.TokenTTL, &c.ACL.TokenTTLHCL},
		{"acl.policy_ttl", &c.ACL.PolicyTTL, &c.ACL.PolicyTTLHCL},
		{"client.server_join.retry_interval", &c.Client.ServerJoin.RetryInterval, &c.Client.ServerJoin.RetryIntervalHCL},
		{"server.heartbeat_grace", &c.Server.HeartbeatGrace, &c.Server.HeartbeatGraceHCL},
		{"server.min_heartbeat_ttl", &c.Server.MinHeartbeatTTL, &c.Server.MinHeartbeatTTLHCL},
		{"server.retry_interval", &c.Server.RetryInterval, &c.Server.RetryIntervalHCL},
		{"server.server_join.retry_interval", &c.Server.ServerJoin.RetryInterval, &c.Server.ServerJoin.RetryIntervalHCL},
		{"consul.timeout", &c.Consul.Timeout, &c.Consul.TimeoutHCL},
		{"autopilot.server_stabilization_time", &c.Autopilot.ServerStabilizationTime, &c.Autopilot.ServerStabilizationTimeHCL},
		{"autopilot.last_contact_threshold", &c.Autopilot.LastContactThreshold, &c.Autopilot.LastContactThresholdHCL},
		{"telemetry.collection_interval", &c.Telemetry.collectionInterval, &c.Telemetry.CollectionInterval},
	}

	// Add enterprise audit sinks for time.Duration parsing
	for i, sink := range c.Audit.Sinks {
		tds = append(tds, td{
			fmt.Sprintf("audit.sink.%d", i), &sink.RotateDuration, &sink.RotateDurationHCL,
		})
	}

	// convert strings to time.Durations
	err = durations(tds)
	if err != nil {
		return nil, err
	}

	// report unexpected keys
	err = extraKeys(c)
	if err != nil {
		return nil, err
	}

	return c, nil
}

// td holds args for one duration conversion
type td struct {
	path string
	td   *time.Duration
	str  *string
}

// durations parses the duration strings specified in the config files
// into time.Durations
func durations(xs []td) error {
	for _, x := range xs {
		if x.td != nil && x.str != nil && "" != *x.str {
			d, err := time.ParseDuration(*x.str)
			if err != nil {
				return fmt.Errorf("%s can't parse time duration %s", x.path, *x.str)
			}

			*x.td = d
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

	for _, k := range []string{"datadog_tags"} {
		helper.RemoveEqualFold(&c.ExtraKeysHCL, k)
		helper.RemoveEqualFold(&c.ExtraKeysHCL, "telemetry")
	}

	return helper.UnusedKeys(c)
}
