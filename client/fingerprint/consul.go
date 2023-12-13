// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	consulapi "github.com/hashicorp/consul/api"
	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-version"
	agentconsul "github.com/hashicorp/nomad/command/agent/consul"
)

const (
	consulAvailable   = "available"
	consulUnavailable = "unavailable"
)

var (
	// consulGRPCPortChangeVersion is the Consul version which made a breaking
	// change to the way gRPC API listeners are created. This means Nomad must
	// perform different fingerprinting depending on which version of Consul it
	// is communicating with.
	consulGRPCPortChangeVersion = version.Must(version.NewVersion("1.14.0"))
)

// ConsulFingerprint is used to fingerprint for Consul
type ConsulFingerprint struct {
	logger     log.Logger
	client     *consulapi.Client
	lastState  string
	extractors map[string]consulExtractor
}

// consulExtractor is used to parse out one attribute from consulInfo. Returns
// the value of the attribute, and whether the attribute exists.
type consulExtractor func(agentconsul.Self) (string, bool)

// NewConsulFingerprint is used to create a Consul fingerprint
func NewConsulFingerprint(logger log.Logger) Fingerprint {
	return &ConsulFingerprint{
		logger:    logger.Named("consul"),
		lastState: consulUnavailable,
	}
}

func (f *ConsulFingerprint) Fingerprint(req *FingerprintRequest, resp *FingerprintResponse) error {

	// establish consul client if necessary
	if err := f.initialize(req); err != nil {
		return err
	}

	// query consul for agent self api
	info := f.query(resp)
	if len(info) == 0 {
		// unable to reach consul, nothing to do this time
		return nil
	}

	// apply the extractor for each attribute
	for attr, extractor := range f.extractors {
		if s, ok := extractor(info); !ok {
			f.logger.Warn("unable to fingerprint consul", "attribute", attr)
		} else {
			resp.AddAttribute(attr, s)
		}
	}

	// create link for consul
	f.link(resp)

	// indicate Consul is now available
	if f.lastState == consulUnavailable {
		f.logger.Info("consul agent is available")
	}

	f.lastState = consulAvailable
	resp.Detected = true
	return nil
}

func (f *ConsulFingerprint) Periodic() (bool, time.Duration) {
	return true, 15 * time.Second
}

func (f *ConsulFingerprint) initialize(req *FingerprintRequest) error {
	// Only create the Consul client once to avoid creating many connections
	if f.client == nil {
		consulConfig, err := req.Config.ConsulConfig.ApiConfig()
		if err != nil {
			return fmt.Errorf("failed to initialize Consul client config: %v", err)
		}

		f.client, err = consulapi.NewClient(consulConfig)
		if err != nil {
			return fmt.Errorf("failed to initialize Consul client: %s", err)
		}

		f.extractors = map[string]consulExtractor{
			"consul.server":        f.server,
			"consul.version":       f.version,
			"consul.sku":           f.sku,
			"consul.revision":      f.revision,
			"unique.consul.name":   f.name,
			"consul.datacenter":    f.dc,
			"consul.segment":       f.segment,
			"consul.connect":       f.connect,
			"consul.grpc":          f.grpc(consulConfig.Scheme),
			"consul.ft.namespaces": f.namespaces,
		}
	}

	return nil
}

func (f *ConsulFingerprint) query(resp *FingerprintResponse) agentconsul.Self {
	// We'll try to detect consul by making a query to to the agent's self API.
	// If we can't hit this URL consul is probably not running on this machine.
	info, err := f.client.Agent().Self()
	if err != nil {
		// indicate consul no longer available
		if f.lastState == consulAvailable {
			f.logger.Info("consul agent is unavailable")
		}
		f.lastState = consulUnavailable
		return nil
	}
	return info
}

func (f *ConsulFingerprint) link(resp *FingerprintResponse) {
	if dc, ok := resp.Attributes["consul.datacenter"]; ok {
		if name, ok2 := resp.Attributes["unique.consul.name"]; ok2 {
			resp.AddLink("consul", fmt.Sprintf("%s.%s", dc, name))
		}
	} else {
		f.logger.Warn("malformed Consul response prevented linking")
	}
}

func (f *ConsulFingerprint) server(info agentconsul.Self) (string, bool) {
	s, ok := info["Config"]["Server"].(bool)
	return strconv.FormatBool(s), ok
}

func (f *ConsulFingerprint) version(info agentconsul.Self) (string, bool) {
	v, ok := info["Config"]["Version"].(string)
	return v, ok
}

func (f *ConsulFingerprint) sku(info agentconsul.Self) (string, bool) {
	return agentconsul.SKU(info)
}

func (f *ConsulFingerprint) revision(info agentconsul.Self) (string, bool) {
	r, ok := info["Config"]["Revision"].(string)
	return r, ok
}

func (f *ConsulFingerprint) name(info agentconsul.Self) (string, bool) {
	n, ok := info["Config"]["NodeName"].(string)
	return n, ok
}

func (f *ConsulFingerprint) dc(info agentconsul.Self) (string, bool) {
	d, ok := info["Config"]["Datacenter"].(string)
	return d, ok
}

func (f *ConsulFingerprint) segment(info agentconsul.Self) (string, bool) {
	tags, tagsOK := info["Member"]["Tags"].(map[string]interface{})
	if !tagsOK {
		return "", false
	}
	s, ok := tags["segment"].(string)
	return s, ok
}

func (f *ConsulFingerprint) connect(info agentconsul.Self) (string, bool) {
	c, ok := info["DebugConfig"]["ConnectEnabled"].(bool)
	return strconv.FormatBool(c), ok
}

func (f *ConsulFingerprint) grpc(scheme string) func(info agentconsul.Self) (string, bool) {
	return func(info agentconsul.Self) (string, bool) {

		// The version is needed in order to understand which config object to
		// query. This is because Consul 1.14.0 added a new gRPC port which
		// broke the previous behaviour.
		v, ok := info["Config"]["Version"].(string)
		if !ok {
			return "", false
		}

		consulVersion, err := version.NewVersion(strings.TrimSpace(v))
		if err != nil {
			f.logger.Warn("invalid Consul version", "version", v)
			return "", false
		}

		// If the Consul agent being fingerprinted is running a version less
		// than 1.14.0 we use the original single gRPC port.
		if consulVersion.Core().LessThan(consulGRPCPortChangeVersion.Core()) {
			return f.grpcPort(info)
		}

		// Now that we know we are querying a Consul agent running v1.14.0 or
		// greater, we need to select the correct port parameter from the
		// config depending on whether we have been asked to speak TLS or not.
		switch strings.ToLower(scheme) {
		case "https":
			return f.grpcTLSPort(info)
		default:
			return f.grpcPort(info)
		}
	}
}

func (f *ConsulFingerprint) grpcPort(info agentconsul.Self) (string, bool) {
	p, ok := info["DebugConfig"]["GRPCPort"].(float64)
	return fmt.Sprintf("%d", int(p)), ok
}

func (f *ConsulFingerprint) grpcTLSPort(info agentconsul.Self) (string, bool) {
	p, ok := info["DebugConfig"]["GRPCTLSPort"].(float64)
	return fmt.Sprintf("%d", int(p)), ok
}

func (f *ConsulFingerprint) namespaces(info agentconsul.Self) (string, bool) {
	return strconv.FormatBool(agentconsul.Namespaces(info)), true
}
