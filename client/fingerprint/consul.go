package fingerprint

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	consul "github.com/hashicorp/consul/api"
	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-version"
)

const (
	consulAvailable   = "available"
	consulUnavailable = "unavailable"
)

// ConsulFingerprint is used to fingerprint for Consul
type ConsulFingerprint struct {
	logger     log.Logger
	client     *consul.Client
	lastState  string
	extractors map[string]consulExtractor
}

// consulInfo aliases the type returned from the Consul agent self endpoint.
type consulInfo = map[string]map[string]interface{}

// consulExtractor is used to parse out one attribute from consulInfo. Returns
// the value of the attribute, and whether the attribute exists.
type consulExtractor func(consulInfo) (string, bool)

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

// clearConsulAttributes removes consul attributes and links from the passed Node.
func (f *ConsulFingerprint) clearConsulAttributes(r *FingerprintResponse) {
	for attr := range f.extractors {
		r.RemoveAttribute(attr)
	}
	r.RemoveLink("consul")
}

func (f *ConsulFingerprint) initialize(req *FingerprintRequest) error {
	// Only create the Consul client once to avoid creating many connections
	if f.client == nil {
		consulConfig, err := req.Config.ConsulConfig.ApiConfig()
		if err != nil {
			return fmt.Errorf("failed to initialize Consul client config: %v", err)
		}

		f.client, err = consul.NewClient(consulConfig)
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
			"consul.grpc":          f.grpc,
			"consul.ft.namespaces": f.namespaces,
		}
	}

	return nil
}

func (f *ConsulFingerprint) query(resp *FingerprintResponse) consulInfo {
	// We'll try to detect consul by making a query to to the agent's self API.
	// If we can't hit this URL consul is probably not running on this machine.
	info, err := f.client.Agent().Self()
	if err != nil {
		f.clearConsulAttributes(resp)

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

func (f *ConsulFingerprint) server(info consulInfo) (string, bool) {
	s, ok := info["Config"]["Server"].(bool)
	return strconv.FormatBool(s), ok
}

func (f *ConsulFingerprint) version(info consulInfo) (string, bool) {
	v, ok := info["Config"]["Version"].(string)
	return v, ok
}

func (f *ConsulFingerprint) sku(info consulInfo) (string, bool) {
	v, ok := info["Config"]["Version"].(string)
	if !ok {
		return "", ok
	}

	ver, vErr := version.NewVersion(v)
	if vErr != nil {
		return "", false
	}
	if strings.Contains(ver.Metadata(), "ent") {
		return "ent", true
	}
	return "oss", true
}

func (f *ConsulFingerprint) revision(info consulInfo) (string, bool) {
	r, ok := info["Config"]["Revision"].(string)
	return r, ok
}

func (f *ConsulFingerprint) name(info consulInfo) (string, bool) {
	n, ok := info["Config"]["NodeName"].(string)
	return n, ok
}

func (f *ConsulFingerprint) dc(info consulInfo) (string, bool) {
	d, ok := info["Config"]["Datacenter"].(string)
	return d, ok
}

func (f *ConsulFingerprint) segment(info consulInfo) (string, bool) {
	tags, tagsOK := info["Member"]["Tags"].(map[string]interface{})
	if !tagsOK {
		return "", false
	}
	s, ok := tags["segment"].(string)
	return s, ok
}

func (f *ConsulFingerprint) connect(info consulInfo) (string, bool) {
	c, ok := info["DebugConfig"]["ConnectEnabled"].(bool)
	return strconv.FormatBool(c), ok
}

func (f *ConsulFingerprint) grpc(info consulInfo) (string, bool) {
	p, ok := info["DebugConfig"]["GRPCPort"].(float64)
	return fmt.Sprintf("%d", int(p)), ok
}

func (f *ConsulFingerprint) namespaces(info consulInfo) (string, bool) {
	return f.feature("Namespaces", info)
}

// possible values as of v1.9.5+ent:
//   Automated Backups, Automated Upgrades, Enhanced Read Scalability,
//   Network Segments, Redundancy Zone, Advanced Network Federation,
//   Namespaces, SSO, Audit Logging
func (f *ConsulFingerprint) feature(name string, info consulInfo) (string, bool) {
	lic, licOK := info["Stats"]["license"].(map[string]interface{})
	if !licOK {
		return "", false
	}

	features, exists := lic["features"].(string)
	if !exists {
		return "", false
	}

	if !strings.Contains(features, name) {
		return "", false
	}

	return "true", true
}
