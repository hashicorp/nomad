package agent

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/hashicorp/hcl"
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
	err = durations([]td{
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
	})
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

// removeEqualFold removes the first string that EqualFold matches
func removeEqualFold(xs *[]string, search string) {
	sl := *xs
	for i, x := range sl {
		if strings.EqualFold(x, search) {
			sl = append(sl[:i], sl[i+1:]...)
			if len(sl) == 0 {
				*xs = nil
			} else {
				*xs = sl
			}
			return
		}
	}
}

func extraKeys(c *Config) error {
	// hcl leaves behind extra keys when parsing JSON. These keys
	// are kept on the top level, taken from slices or the keys of
	// structs contained in slices. Clean up before looking for
	// extra keys.
	for range c.HTTPAPIResponseHeaders {
		removeEqualFold(&c.ExtraKeysHCL, "http_api_response_headers")
	}

	for _, p := range c.Plugins {
		removeEqualFold(&c.ExtraKeysHCL, p.Name)
		removeEqualFold(&c.ExtraKeysHCL, "config")
		removeEqualFold(&c.ExtraKeysHCL, "plugin")
	}

	for _, k := range []string{"options", "meta", "chroot_env", "servers", "server_join"} {
		removeEqualFold(&c.ExtraKeysHCL, k)
		removeEqualFold(&c.ExtraKeysHCL, "client")
	}

	// stats is an unused key, continue to silently ignore it
	removeEqualFold(&c.Client.ExtraKeysHCL, "stats")

	for _, k := range []string{"enabled_schedulers", "start_join", "retry_join", "server_join"} {
		removeEqualFold(&c.ExtraKeysHCL, k)
		removeEqualFold(&c.ExtraKeysHCL, "server")
	}

	for _, k := range []string{"datadog_tags"} {
		removeEqualFold(&c.ExtraKeysHCL, k)
		removeEqualFold(&c.ExtraKeysHCL, "telemetry")
	}

	return extraKeysImpl([]string{}, reflect.ValueOf(*c))
}

// extraKeysImpl returns an error if any extraKeys array is not empty
func extraKeysImpl(path []string, val reflect.Value) error {
	stype := val.Type()
	for i := 0; i < stype.NumField(); i++ {
		ftype := stype.Field(i)
		fval := val.Field(i)

		name := ftype.Name
		prop := ""
		tagSplit(ftype, "hcl", &name, &prop)

		if fval.Kind() == reflect.Ptr {
			fval = reflect.Indirect(fval)
		}

		// struct? recurse. add the struct's key to the path
		if fval.Kind() == reflect.Struct {
			err := extraKeysImpl(append([]string{name}, path...), fval)
			if err != nil {
				return err
			}
		}

		if "unusedKeys" == prop {
			if ks, ok := fval.Interface().([]string); ok && len(ks) != 0 {
				return fmt.Errorf("%s unexpected keys %s",
					strings.Join(path, "."),
					strings.Join(ks, ", "))
			}
		}
	}
	return nil
}

// tagSplit reads the named tag from the structfield and splits its values into strings
func tagSplit(field reflect.StructField, tagName string, vars ...*string) {
	tag := strings.Split(field.Tag.Get(tagName), ",")
	end := len(tag) - 1
	for i, s := range vars {
		if i > end {
			return
		}
		*s = tag[i]
	}
}
