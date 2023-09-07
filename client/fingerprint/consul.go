// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-hclog"
	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/go-version"
	agentconsul "github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/nomad/structs/config"
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
	logger log.Logger
	states map[string]*consulFingerprintState
}

type consulFingerprintState struct {
	client      *consulapi.Client
	isAvailable bool
	extractors  map[string]consulExtractor
}

// consulExtractor is used to parse out one attribute from consulInfo. Returns
// the value of the attribute, and whether the attribute exists.
type consulExtractor func(agentconsul.Self) (string, bool)

// NewConsulFingerprint is used to create a Consul fingerprint
func NewConsulFingerprint(logger log.Logger) Fingerprint {
	return &ConsulFingerprint{
		logger: logger.Named("consul"),
		states: map[string]*consulFingerprintState{},
	}
}

func (f *ConsulFingerprint) Fingerprint(req *FingerprintRequest, resp *FingerprintResponse) error {
	var mErr *multierror.Error
	for _, cfg := range f.consulConfigs(req) {
		err := f.fingerprintImpl(cfg, resp)
		if err != nil {
			mErr = multierror.Append(mErr, err)
		}
	}

	return mErr.ErrorOrNil()
}

func (f *ConsulFingerprint) fingerprintImpl(cfg *config.ConsulConfig, resp *FingerprintResponse) error {

	logger := f.logger.With("cluster", cfg.Name)

	state, ok := f.states[cfg.Name]
	if !ok {
		state = &consulFingerprintState{}
		f.states[cfg.Name] = state
	}

	if err := state.initialize(cfg, logger); err != nil {
		return err
	}

	// query consul for agent self api
	info := state.query(logger)
	if len(info) == 0 {
		// unable to reach consul, nothing to do this time
		return nil
	}

	// apply the extractor for each attribute
	for attr, extractor := range state.extractors {
		if s, ok := extractor(info); !ok {
			logger.Warn("unable to fingerprint consul", "attribute", attr)
		} else {
			resp.AddAttribute(attr, s)
		}
	}

	// create link for consul
	f.link(resp)

	// indicate Consul is now available
	if !state.isAvailable {
		logger.Info("consul agent is available")
	}

	state.isAvailable = true
	resp.Detected = true
	return nil
}

func (f *ConsulFingerprint) Periodic() (bool, time.Duration) {
	return true, 15 * time.Second
}

func (cfs *consulFingerprintState) initialize(cfg *config.ConsulConfig, logger hclog.Logger) error {
	if cfs.client != nil {
		return nil // already initialized!
	}

	consulConfig, err := cfg.ApiConfig()
	if err != nil {
		return fmt.Errorf("failed to initialize Consul client config: %v", err)
	}

	cfs.client, err = consulapi.NewClient(consulConfig)
	if err != nil {
		return fmt.Errorf("failed to initialize Consul client: %v", err)
	}

	if cfg.Name == "default" {
		cfs.extractors = map[string]consulExtractor{
			"consul.server":        cfs.server,
			"consul.version":       cfs.version,
			"consul.sku":           cfs.sku,
			"consul.revision":      cfs.revision,
			"unique.consul.name":   cfs.name, // note: won't have this for non-default clusters
			"consul.datacenter":    cfs.dc,
			"consul.segment":       cfs.segment,
			"consul.connect":       cfs.connect,
			"consul.grpc":          cfs.grpc(consulConfig.Scheme, logger),
			"consul.ft.namespaces": cfs.namespaces,
		}
	} else {
		cfs.extractors = map[string]consulExtractor{
			fmt.Sprintf("consul.%s.server", cfg.Name):        cfs.server,
			fmt.Sprintf("consul.%s.version", cfg.Name):       cfs.version,
			fmt.Sprintf("consul.%s.sku", cfg.Name):           cfs.sku,
			fmt.Sprintf("consul.%s.revision", cfg.Name):      cfs.revision,
			fmt.Sprintf("consul.%s.datacenter", cfg.Name):    cfs.dc,
			fmt.Sprintf("consul.%s.segment", cfg.Name):       cfs.segment,
			fmt.Sprintf("consul.%s.connect", cfg.Name):       cfs.connect,
			fmt.Sprintf("consul.%s.grpc", cfg.Name):          cfs.grpc(consulConfig.Scheme, logger),
			fmt.Sprintf("consul.%s.ft.namespaces", cfg.Name): cfs.namespaces,
		}
	}

	return nil
}

func (cfs *consulFingerprintState) query(logger hclog.Logger) agentconsul.Self {
	// We'll try to detect consul by making a query to to the agent's self API.
	// If we can't hit this URL consul is probably not running on this machine.
	info, err := cfs.client.Agent().Self()
	if err != nil {
		// indicate consul no longer available
		if cfs.isAvailable {
			logger.Info("consul agent is unavailable: %v", err)
		}
		cfs.isAvailable = false
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

func (cfs *consulFingerprintState) server(info agentconsul.Self) (string, bool) {
	s, ok := info["Config"]["Server"].(bool)
	return strconv.FormatBool(s), ok
}

func (cfs *consulFingerprintState) version(info agentconsul.Self) (string, bool) {
	v, ok := info["Config"]["Version"].(string)
	return v, ok
}

func (cfs *consulFingerprintState) sku(info agentconsul.Self) (string, bool) {
	return agentconsul.SKU(info)
}

func (cfs *consulFingerprintState) revision(info agentconsul.Self) (string, bool) {
	r, ok := info["Config"]["Revision"].(string)
	return r, ok
}

func (cfs *consulFingerprintState) name(info agentconsul.Self) (string, bool) {
	n, ok := info["Config"]["NodeName"].(string)
	return n, ok
}

func (cfs *consulFingerprintState) dc(info agentconsul.Self) (string, bool) {
	d, ok := info["Config"]["Datacenter"].(string)
	return d, ok
}

func (cfs *consulFingerprintState) segment(info agentconsul.Self) (string, bool) {
	tags, tagsOK := info["Member"]["Tags"].(map[string]interface{})
	if !tagsOK {
		return "", false
	}
	s, ok := tags["segment"].(string)
	return s, ok
}

func (cfs *consulFingerprintState) connect(info agentconsul.Self) (string, bool) {
	c, ok := info["DebugConfig"]["ConnectEnabled"].(bool)
	return strconv.FormatBool(c), ok
}

func (cfs *consulFingerprintState) grpc(scheme string, logger hclog.Logger) func(info agentconsul.Self) (string, bool) {
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
			logger.Warn("invalid Consul version", "version", v)
			return "", false
		}

		// If the Consul agent being fingerprinted is running a version less
		// than 1.14.0 we use the original single gRPC port.
		if consulVersion.Core().LessThan(consulGRPCPortChangeVersion.Core()) {
			return cfs.grpcPort(info)
		}

		// Now that we know we are querying a Consul agent running v1.14.0 or
		// greater, we need to select the correct port parameter from the
		// config depending on whether we have been asked to speak TLS or not.
		switch strings.ToLower(scheme) {
		case "https":
			return cfs.grpcTLSPort(info)
		default:
			return cfs.grpcPort(info)
		}
	}
}

func (cfs *consulFingerprintState) grpcPort(info agentconsul.Self) (string, bool) {
	p, ok := info["DebugConfig"]["GRPCPort"].(float64)
	return fmt.Sprintf("%d", int(p)), ok
}

func (cfs *consulFingerprintState) grpcTLSPort(info agentconsul.Self) (string, bool) {
	p, ok := info["DebugConfig"]["GRPCTLSPort"].(float64)
	return fmt.Sprintf("%d", int(p)), ok
}

func (cfs *consulFingerprintState) namespaces(info agentconsul.Self) (string, bool) {
	return strconv.FormatBool(agentconsul.Namespaces(info)), true
}
