// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"fmt"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/go-sockaddr"
	"github.com/hashicorp/go-version"
	agentconsul "github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/nomad/structs"
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
	logger hclog.Logger

	// clusters maintains the latest fingerprinted state for each cluster
	// defined in nomad consul client configuration(s).
	clusters map[string]*consulState

	// Once initial fingerprints are complete, we no-op all periodic
	// fingerprints to prevent Consul availability issues causing a thundering
	// herd of node updates. This behavior resets if we reload the
	// configuration.
	initialResponse     *FingerprintResponse
	initialResponseLock sync.RWMutex
}

type consulState struct {
	client *consulapi.Client

	// readers associates a function used to parse the value associated
	// with the given key from a consul api response
	readers map[string]valueReader

	// tracks that we've successfully fingerprinted this cluster at least once
	// since the last Fingerprint call
	fingerprintedOnce bool

	// we currently can't disable Consul fingerprinting, so for users who aren't
	// using it we want to make sure we report the periodic failure only once
	reportedOnce bool
}

// valueReader is used to parse out one attribute from consulInfo. Returns
// the value of the attribute, and whether the attribute exists.
type valueReader func(agentconsul.Self) (string, bool)

// NewConsulFingerprint is used to create a Consul fingerprint
func NewConsulFingerprint(logger hclog.Logger) Fingerprint {
	return &ConsulFingerprint{
		logger:   logger.Named("consul"),
		clusters: map[string]*consulState{},
	}
}

func (f *ConsulFingerprint) Fingerprint(req *FingerprintRequest, resp *FingerprintResponse) error {
	if f.readInitialResponse(resp) {
		return nil
	}

	var mErr *multierror.Error
	consulConfigs := req.Config.GetConsulConfigs(f.logger)
	for _, cfg := range consulConfigs {
		err := f.fingerprintImpl(cfg, resp)
		if err != nil {
			mErr = multierror.Append(mErr, err)
		}
	}

	fingerprintCount := 0
	for _, state := range f.clusters {
		if state.fingerprintedOnce {
			fingerprintCount++
		}
	}
	if fingerprintCount == len(consulConfigs) {
		f.setInitialResponse(resp)
	}

	return mErr.ErrorOrNil()
}

// readInitialResponse checks for a previously seen response. It returns true
// and shallow-copies the response into the argument if one is available. We
// only want to hold the lock open during the read and not the Fingerprint so
// that we don't block a Reload call while waiting for Consul requests to
// complete. If the Reload clears the initialResponse after we take the lock
// again in setInitialResponse (ex. 2 reloads quickly in a row), the worst that
// happens is we do an extra fingerprint when the Reload caller calls
// Fingerprint
func (f *ConsulFingerprint) readInitialResponse(resp *FingerprintResponse) bool {
	f.initialResponseLock.RLock()
	defer f.initialResponseLock.RUnlock()
	if f.initialResponse != nil {
		*resp = *f.initialResponse
		return true
	}
	return false
}

func (f *ConsulFingerprint) setInitialResponse(resp *FingerprintResponse) {
	f.initialResponseLock.Lock()
	defer f.initialResponseLock.Unlock()
	f.initialResponse = resp
}

func (f *ConsulFingerprint) fingerprintImpl(cfg *config.ConsulConfig, resp *FingerprintResponse) error {
	logger := f.logger.With("cluster", cfg.Name)

	state, ok := f.clusters[cfg.Name]
	if !ok {
		state = &consulState{}
		f.clusters[cfg.Name] = state
	}

	if err := state.initialize(cfg, logger); err != nil {
		return err
	}

	// query consul for agent self api
	info := state.query(logger)
	if len(info) == 0 {
		// unable to reach consul, clear out existing attributes
		resp.Detected = true
		return nil
	}

	// apply the extractor for each attribute
	for attr, extractor := range state.readers {
		if s, ok := extractor(info); !ok {
			logger.Warn("unable to fingerprint consul", "attribute", attr)
		} else if s != "" {
			resp.AddAttribute(attr, s)
		}
	}

	// create link for consul
	f.link(resp)

	state.fingerprintedOnce = true
	resp.Detected = true
	return nil
}

func (f *ConsulFingerprint) Periodic() (bool, time.Duration) {
	return true, 15 * time.Second
}

// Reload satisfies ReloadableFingerprint and resets the gate on periodic
// fingerprinting.
func (f *ConsulFingerprint) Reload() {
	f.setInitialResponse(nil)
}

func (cfs *consulState) initialize(cfg *config.ConsulConfig, logger hclog.Logger) error {
	cfs.fingerprintedOnce = false
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

	if cfg.Name == structs.ConsulDefaultCluster {
		cfs.readers = map[string]valueReader{
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
			"consul.partition":     cfs.partition,
			"consul.dns.port":      cfs.dnsPort,
			"consul.dns.addr":      cfs.dnsAddr(logger),
		}
	} else {
		cfs.readers = map[string]valueReader{
			fmt.Sprintf("consul.%s.server", cfg.Name):        cfs.server,
			fmt.Sprintf("consul.%s.version", cfg.Name):       cfs.version,
			fmt.Sprintf("consul.%s.sku", cfg.Name):           cfs.sku,
			fmt.Sprintf("consul.%s.revision", cfg.Name):      cfs.revision,
			fmt.Sprintf("consul.%s.datacenter", cfg.Name):    cfs.dc,
			fmt.Sprintf("consul.%s.segment", cfg.Name):       cfs.segment,
			fmt.Sprintf("consul.%s.connect", cfg.Name):       cfs.connect,
			fmt.Sprintf("consul.%s.grpc", cfg.Name):          cfs.grpc(consulConfig.Scheme, logger),
			fmt.Sprintf("consul.%s.ft.namespaces", cfg.Name): cfs.namespaces,
			fmt.Sprintf("consul.%s.partition", cfg.Name):     cfs.partition,
			fmt.Sprintf("consul.%s.dns.port", cfg.Name):      cfs.dnsPort,
			fmt.Sprintf("consul.%s.dns.addr", cfg.Name):      cfs.dnsAddr(logger),
		}
	}

	return nil
}

func (cfs *consulState) query(logger hclog.Logger) agentconsul.Self {
	// We'll try to detect consul by making a query to to the agent's self API.
	// If we can't hit this URL consul is probably not running on this machine.
	info, err := cfs.client.Agent().Self()
	if err != nil {
		if cfs.reportedOnce {
			return nil
		}
		cfs.reportedOnce = true
		logger.Warn("failed to acquire consul self endpoint", "error", err)
		return nil
	}

	cfs.reportedOnce = false
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

func (cfs *consulState) server(info agentconsul.Self) (string, bool) {
	s, ok := info["Config"]["Server"].(bool)
	return strconv.FormatBool(s), ok
}

func (cfs *consulState) version(info agentconsul.Self) (string, bool) {
	v, ok := info["Config"]["Version"].(string)
	return v, ok
}

func (cfs *consulState) sku(info agentconsul.Self) (string, bool) {
	return agentconsul.SKU(info)
}

func (cfs *consulState) revision(info agentconsul.Self) (string, bool) {
	r, ok := info["Config"]["Revision"].(string)
	return r, ok
}

func (cfs *consulState) name(info agentconsul.Self) (string, bool) {
	n, ok := info["Config"]["NodeName"].(string)
	return n, ok
}

func (cfs *consulState) dc(info agentconsul.Self) (string, bool) {
	d, ok := info["Config"]["Datacenter"].(string)
	return d, ok
}

func (cfs *consulState) segment(info agentconsul.Self) (string, bool) {
	tags, tagsOK := info["Member"]["Tags"].(map[string]interface{})
	if !tagsOK {
		return "", false
	}
	s, ok := tags["segment"].(string)
	return s, ok
}

func (cfs *consulState) connect(info agentconsul.Self) (string, bool) {
	c, ok := info["DebugConfig"]["ConnectEnabled"].(bool)
	return strconv.FormatBool(c), ok
}

func (cfs *consulState) grpc(scheme string, logger hclog.Logger) func(info agentconsul.Self) (string, bool) {
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

func (cfs *consulState) grpcPort(info agentconsul.Self) (string, bool) {
	p, ok := info["DebugConfig"]["GRPCPort"].(float64)
	return fmt.Sprintf("%d", int(p)), ok
}

func (cfs *consulState) grpcTLSPort(info agentconsul.Self) (string, bool) {
	p, ok := info["DebugConfig"]["GRPCTLSPort"].(float64)
	return fmt.Sprintf("%d", int(p)), ok
}

func (cfs *consulState) dnsPort(info agentconsul.Self) (string, bool) {
	p, ok := info["DebugConfig"]["DNSPort"].(float64)
	return fmt.Sprintf("%d", int(p)), ok
}

// dnsAddr fingerprints the Consul DNS address, but only if Nomad can use it
// usefully to provide an iptables rule to a task
func (cfs *consulState) dnsAddr(logger hclog.Logger) func(info agentconsul.Self) (string, bool) {
	return func(info agentconsul.Self) (string, bool) {

		var listenOnEveryIP bool

		dnsAddrs, ok := info["DebugConfig"]["DNSAddrs"].([]any)
		if !ok {
			logger.Warn("Consul returned invalid addresses.dns config",
				"value", info["DebugConfig"]["DNSAddrs"])
			return "", false
		}

		for _, d := range dnsAddrs {
			dnsAddr, ok := d.(string)
			if !ok {
				logger.Warn("Consul returned invalid addresses.dns config",
					"value", info["DebugConfig"]["DNSAddrs"])
				return "", false

			}
			dnsAddr = strings.TrimPrefix(dnsAddr, "tcp://")
			dnsAddr = strings.TrimPrefix(dnsAddr, "udp://")

			parsed, err := netip.ParseAddrPort(dnsAddr)
			if err != nil {
				logger.Warn("could not parse Consul addresses.dns config",
					"value", dnsAddr, "error", err)
				return "", false // response is somehow malformed
			}

			// only addresses we can use for an iptables rule from a
			// container to the host will be fingerprinted
			if parsed.Addr().IsUnspecified() {
				listenOnEveryIP = true
				break
			}
			if !parsed.Addr().IsLoopback() {
				return parsed.Addr().String(), true
			}
		}

		// if Consul DNS is bound on 0.0.0.0, we want to fingerprint the private
		// IP (or at worst, the public IP) of the host so that we have a valid
		// IP address for the iptables rule
		if listenOnEveryIP {

			privateIP, err := sockaddr.GetPrivateIP()
			if err != nil {
				logger.Warn("could not query network interfaces", "error", err)
				return "", false // something is very wrong, so bail out
			}
			if privateIP != "" {
				return privateIP, true
			}
			publicIP, err := sockaddr.GetPublicIP()
			if err != nil {
				logger.Warn("could not query network interfaces", "error", err)
				return "", false // something is very wrong, so bail out
			}
			if publicIP != "" {
				return publicIP, true
			}
		}

		// if we've hit here, Consul is bound on localhost and we won't be able
		// to configure container DNS to use it, but we also don't want to have
		// the fingerprinter return an error
		return "", true
	}
}

func (cfs *consulState) namespaces(info agentconsul.Self) (string, bool) {
	return strconv.FormatBool(agentconsul.Namespaces(info)), true
}

func (cfs *consulState) partition(info agentconsul.Self) (string, bool) {
	sku, ok := agentconsul.SKU(info)
	if ok && sku == "ent" {
		p, ok := info["Config"]["Partition"].(string)
		if !ok {
			p = "default"
		}
		return p, true
	}
	return "", true // prevent warnings on Consul CE
}
