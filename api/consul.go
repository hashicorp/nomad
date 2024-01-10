// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"time"

	"golang.org/x/exp/maps"
)

// Consul represents configuration related to consul.
type Consul struct {
	// (Enterprise-only) Namespace represents a Consul namespace.
	Namespace string `mapstructure:"namespace" hcl:"namespace,optional"`

	// (Enterprise-only) Cluster represents a specific Consul cluster.
	Cluster string `mapstructure:"cluster" hcl:"cluster,optional"`

	// Partition is the Consul admin partition where the workload should
	// run. This is available in Nomad CE but only works with Consul ENT
	Partition string `mapstructure:"partition" hcl:"partition,optional"`
}

// Canonicalize Consul into a canonical form. The Canonicalize structs containing
// a Consul should ensure it is not nil.
func (c *Consul) Canonicalize() {
	if c.Cluster == "" {
		c.Cluster = "default"
	}

	// If Namespace is nil, that is a choice of the job submitter that
	// we should inherit from higher up (i.e. job<-group). Likewise, if
	// Namespace is set but empty, that is a choice to use the default consul
	// namespace.

	// Partition should never be defaulted to "default" because non-ENT Consul
	// clusters don't have admin partitions
}

// Copy creates a deep copy of c.
func (c *Consul) Copy() *Consul {
	return &Consul{
		Namespace: c.Namespace,
		Cluster:   c.Cluster,
		Partition: c.Partition,
	}
}

// MergeNamespace sets Namespace to namespace if not already configured.
// This is used to inherit the job-level consul_namespace if the group-level
// namespace is not explicitly configured.
func (c *Consul) MergeNamespace(namespace *string) {
	// only inherit namespace from above if not already set
	if c.Namespace == "" && namespace != nil {
		c.Namespace = *namespace
	}
}

// ConsulConnect represents a Consul Connect jobspec block.
type ConsulConnect struct {
	Native         bool                  `hcl:"native,optional"`
	Gateway        *ConsulGateway        `hcl:"gateway,block"`
	SidecarService *ConsulSidecarService `mapstructure:"sidecar_service" hcl:"sidecar_service,block"`
	SidecarTask    *SidecarTask          `mapstructure:"sidecar_task" hcl:"sidecar_task,block"`
}

func (cc *ConsulConnect) Canonicalize() {
	if cc == nil {
		return
	}

	cc.SidecarService.Canonicalize()
	cc.SidecarTask.Canonicalize()
	cc.Gateway.Canonicalize()
}

// ConsulSidecarService represents a Consul Connect SidecarService jobspec
// block.
type ConsulSidecarService struct {
	Tags                   []string          `hcl:"tags,optional"`
	Port                   string            `hcl:"port,optional"`
	Proxy                  *ConsulProxy      `hcl:"proxy,block"`
	DisableDefaultTCPCheck bool              `mapstructure:"disable_default_tcp_check" hcl:"disable_default_tcp_check,optional"`
	Meta                   map[string]string `hcl:"meta,block"`
}

func (css *ConsulSidecarService) Canonicalize() {
	if css == nil {
		return
	}

	if len(css.Tags) == 0 {
		css.Tags = nil
	}

	if len(css.Meta) == 0 {
		css.Meta = nil
	}

	css.Proxy.Canonicalize()
}

// SidecarTask represents a subset of Task fields that can be set to override
// the fields of the Task generated for the sidecar
type SidecarTask struct {
	Name          string                 `hcl:"name,optional"`
	Driver        string                 `hcl:"driver,optional"`
	User          string                 `hcl:"user,optional"`
	Config        map[string]interface{} `hcl:"config,block"`
	Env           map[string]string      `hcl:"env,block"`
	Resources     *Resources             `hcl:"resources,block"`
	Meta          map[string]string      `hcl:"meta,block"`
	KillTimeout   *time.Duration         `mapstructure:"kill_timeout" hcl:"kill_timeout,optional"`
	LogConfig     *LogConfig             `mapstructure:"logs" hcl:"logs,block"`
	ShutdownDelay *time.Duration         `mapstructure:"shutdown_delay" hcl:"shutdown_delay,optional"`
	KillSignal    string                 `mapstructure:"kill_signal" hcl:"kill_signal,optional"`
}

func (st *SidecarTask) Canonicalize() {
	if st == nil {
		return
	}

	if len(st.Config) == 0 {
		st.Config = nil
	}

	if len(st.Env) == 0 {
		st.Env = nil
	}

	if st.Resources == nil {
		st.Resources = DefaultResources()
	} else {
		st.Resources.Canonicalize()
	}

	if st.LogConfig == nil {
		st.LogConfig = DefaultLogConfig()
	} else {
		st.LogConfig.Canonicalize()
	}

	if len(st.Meta) == 0 {
		st.Meta = nil
	}

	if st.KillTimeout == nil {
		st.KillTimeout = pointerOf(5 * time.Second)
	}

	if st.ShutdownDelay == nil {
		st.ShutdownDelay = pointerOf(time.Duration(0))
	}
}

// ConsulProxy represents a Consul Connect sidecar proxy jobspec block.
type ConsulProxy struct {
	LocalServiceAddress string                 `mapstructure:"local_service_address" hcl:"local_service_address,optional"`
	LocalServicePort    int                    `mapstructure:"local_service_port" hcl:"local_service_port,optional"`
	Expose              *ConsulExposeConfig    `mapstructure:"expose" hcl:"expose,block"`
	ExposeConfig        *ConsulExposeConfig    // Deprecated: only to maintain backwards compatibility. Use Expose instead.
	Upstreams           []*ConsulUpstream      `hcl:"upstreams,block"`
	Config              map[string]interface{} `hcl:"config,block"`
}

func (cp *ConsulProxy) Canonicalize() {
	if cp == nil {
		return
	}

	cp.Expose.Canonicalize()

	if len(cp.Upstreams) == 0 {
		cp.Upstreams = nil
	}

	for _, upstream := range cp.Upstreams {
		upstream.Canonicalize()
	}

	if len(cp.Config) == 0 {
		cp.Config = nil
	}
}

// ConsulMeshGateway is used to configure mesh gateway usage when connecting to
// a connect upstream in another datacenter.
type ConsulMeshGateway struct {
	// Mode configures how an upstream should be accessed with regard to using
	// mesh gateways.
	//
	// local - the connect proxy makes outbound connections through mesh gateway
	// originating in the same datacenter.
	//
	// remote - the connect proxy makes outbound connections to a mesh gateway
	// in the destination datacenter.
	//
	// none (default) - no mesh gateway is used, the proxy makes outbound connections
	// directly to destination services.
	//
	// https://www.consul.io/docs/connect/gateways/mesh-gateway#modes-of-operation
	Mode string `mapstructure:"mode" hcl:"mode,optional"`
}

func (c *ConsulMeshGateway) Canonicalize() {
	// Mode may be empty string, indicating behavior will defer to Consul
	// service-defaults config entry.
}

func (c *ConsulMeshGateway) Copy() *ConsulMeshGateway {
	if c == nil {
		return nil
	}

	return &ConsulMeshGateway{
		Mode: c.Mode,
	}
}

// ConsulUpstream represents a Consul Connect upstream jobspec block.
type ConsulUpstream struct {
	DestinationName      string             `mapstructure:"destination_name" hcl:"destination_name,optional"`
	DestinationNamespace string             `mapstructure:"destination_namespace" hcl:"destination_namespace,optional"`
	DestinationPeer      string             `mapstructure:"destination_peer" hcl:"destination_peer,optional"`
	DestinationType      string             `mapstructure:"destination_type" hcl:"destination_type,optional"`
	LocalBindPort        int                `mapstructure:"local_bind_port" hcl:"local_bind_port,optional"`
	Datacenter           string             `mapstructure:"datacenter" hcl:"datacenter,optional"`
	LocalBindAddress     string             `mapstructure:"local_bind_address" hcl:"local_bind_address,optional"`
	LocalBindSocketPath  string             `mapstructure:"local_bind_socket_path" hcl:"local_bind_socket_path,optional"`
	LocalBindSocketMode  string             `mapstructure:"local_bind_socket_mode" hcl:"local_bind_socket_mode,optional"`
	MeshGateway          *ConsulMeshGateway `mapstructure:"mesh_gateway" hcl:"mesh_gateway,block"`
	Config               map[string]any     `mapstructure:"config" hcl:"config,block"`
}

func (cu *ConsulUpstream) Copy() *ConsulUpstream {
	if cu == nil {
		return nil
	}
	return &ConsulUpstream{
		DestinationName:      cu.DestinationName,
		DestinationNamespace: cu.DestinationNamespace,
		DestinationPeer:      cu.DestinationPeer,
		DestinationType:      cu.DestinationType,
		LocalBindPort:        cu.LocalBindPort,
		Datacenter:           cu.Datacenter,
		LocalBindAddress:     cu.LocalBindAddress,
		LocalBindSocketPath:  cu.LocalBindSocketPath,
		LocalBindSocketMode:  cu.LocalBindSocketMode,
		MeshGateway:          cu.MeshGateway.Copy(),
		Config:               maps.Clone(cu.Config),
	}
}

func (cu *ConsulUpstream) Canonicalize() {
	if cu == nil {
		return
	}
	cu.MeshGateway.Canonicalize()
	if len(cu.Config) == 0 {
		cu.Config = nil
	}
}

type ConsulExposeConfig struct {
	Paths []*ConsulExposePath `mapstructure:"path" hcl:"path,block"`
	Path  []*ConsulExposePath // Deprecated: only to maintain backwards compatibility. Use Paths instead.
}

func (cec *ConsulExposeConfig) Canonicalize() {
	if cec == nil {
		return
	}

	if len(cec.Paths) == 0 {
		cec.Paths = nil
	}

	if len(cec.Path) == 0 {
		cec.Path = nil
	}
}

type ConsulExposePath struct {
	Path          string `hcl:"path,optional"`
	Protocol      string `hcl:"protocol,optional"`
	LocalPathPort int    `mapstructure:"local_path_port" hcl:"local_path_port,optional"`
	ListenerPort  string `mapstructure:"listener_port" hcl:"listener_port,optional"`
}

// ConsulGateway is used to configure one of the Consul Connect Gateway types.
type ConsulGateway struct {
	// Proxy is used to configure the Envoy instance acting as the gateway.
	Proxy *ConsulGatewayProxy `hcl:"proxy,block"`

	// Ingress represents the Consul Configuration Entry for an Ingress Gateway.
	Ingress *ConsulIngressConfigEntry `hcl:"ingress,block"`

	// Terminating represents the Consul Configuration Entry for a Terminating Gateway.
	Terminating *ConsulTerminatingConfigEntry `hcl:"terminating,block"`

	// Mesh indicates the Consul service should be a Mesh Gateway.
	Mesh *ConsulMeshConfigEntry `hcl:"mesh,block"`
}

func (g *ConsulGateway) Canonicalize() {
	if g == nil {
		return
	}
	g.Proxy.Canonicalize()
	g.Ingress.Canonicalize()
	g.Terminating.Canonicalize()
}

func (g *ConsulGateway) Copy() *ConsulGateway {
	if g == nil {
		return nil
	}

	return &ConsulGateway{
		Proxy:       g.Proxy.Copy(),
		Ingress:     g.Ingress.Copy(),
		Terminating: g.Terminating.Copy(),
	}
}

type ConsulGatewayBindAddress struct {
	Name    string `hcl:",label"`
	Address string `mapstructure:"address" hcl:"address,optional"`
	Port    int    `mapstructure:"port" hcl:"port,optional"`
}

var (
	// defaultGatewayConnectTimeout is the default amount of time connections to
	// upstreams are allowed before timing out.
	defaultGatewayConnectTimeout = 5 * time.Second
)

// ConsulGatewayProxy is used to tune parameters of the proxy instance acting as
// one of the forms of Connect gateways that Consul supports.
//
// https://www.consul.io/docs/connect/proxies/envoy#gateway-options
type ConsulGatewayProxy struct {
	ConnectTimeout                  *time.Duration                       `mapstructure:"connect_timeout" hcl:"connect_timeout,optional"`
	EnvoyGatewayBindTaggedAddresses bool                                 `mapstructure:"envoy_gateway_bind_tagged_addresses" hcl:"envoy_gateway_bind_tagged_addresses,optional"`
	EnvoyGatewayBindAddresses       map[string]*ConsulGatewayBindAddress `mapstructure:"envoy_gateway_bind_addresses" hcl:"envoy_gateway_bind_addresses,block"`
	EnvoyGatewayNoDefaultBind       bool                                 `mapstructure:"envoy_gateway_no_default_bind" hcl:"envoy_gateway_no_default_bind,optional"`
	EnvoyDNSDiscoveryType           string                               `mapstructure:"envoy_dns_discovery_type" hcl:"envoy_dns_discovery_type,optional"`
	Config                          map[string]interface{}               `hcl:"config,block"` // escape hatch envoy config
}

func (p *ConsulGatewayProxy) Canonicalize() {
	if p == nil {
		return
	}

	if p.ConnectTimeout == nil {
		// same as the default from consul
		p.ConnectTimeout = pointerOf(defaultGatewayConnectTimeout)
	}

	if len(p.EnvoyGatewayBindAddresses) == 0 {
		p.EnvoyGatewayBindAddresses = nil
	}

	if len(p.Config) == 0 {
		p.Config = nil
	}
}

func (p *ConsulGatewayProxy) Copy() *ConsulGatewayProxy {
	if p == nil {
		return nil
	}

	var binds map[string]*ConsulGatewayBindAddress = nil
	if p.EnvoyGatewayBindAddresses != nil {
		binds = make(map[string]*ConsulGatewayBindAddress, len(p.EnvoyGatewayBindAddresses))
		for k, v := range p.EnvoyGatewayBindAddresses {
			binds[k] = v
		}
	}

	var config map[string]interface{} = nil
	if p.Config != nil {
		config = make(map[string]interface{}, len(p.Config))
		for k, v := range p.Config {
			config[k] = v
		}
	}

	return &ConsulGatewayProxy{
		ConnectTimeout:                  pointerOf(*p.ConnectTimeout),
		EnvoyGatewayBindTaggedAddresses: p.EnvoyGatewayBindTaggedAddresses,
		EnvoyGatewayBindAddresses:       binds,
		EnvoyGatewayNoDefaultBind:       p.EnvoyGatewayNoDefaultBind,
		EnvoyDNSDiscoveryType:           p.EnvoyDNSDiscoveryType,
		Config:                          config,
	}
}

// ConsulGatewayTLSConfig is used to configure TLS for a gateway.
type ConsulGatewayTLSConfig struct {
	Enabled       bool     `hcl:"enabled,optional"`
	TLSMinVersion string   `hcl:"tls_min_version,optional" mapstructure:"tls_min_version"`
	TLSMaxVersion string   `hcl:"tls_max_version,optional" mapstructure:"tls_max_version"`
	CipherSuites  []string `hcl:"cipher_suites,optional" mapstructure:"cipher_suites"`
}

func (tc *ConsulGatewayTLSConfig) Canonicalize() {
}

func (tc *ConsulGatewayTLSConfig) Copy() *ConsulGatewayTLSConfig {
	if tc == nil {
		return nil
	}

	result := &ConsulGatewayTLSConfig{
		Enabled:       tc.Enabled,
		TLSMinVersion: tc.TLSMinVersion,
		TLSMaxVersion: tc.TLSMaxVersion,
	}
	if len(tc.CipherSuites) != 0 {
		cipherSuites := make([]string, len(tc.CipherSuites))
		copy(cipherSuites, tc.CipherSuites)
		result.CipherSuites = cipherSuites
	}

	return result
}

// ConsulIngressService is used to configure a service fronted by the ingress gateway.
type ConsulIngressService struct {
	// Namespace is not yet supported.
	// Namespace string
	Name string `hcl:"name,optional"`

	Hosts []string `hcl:"hosts,optional"`
}

func (s *ConsulIngressService) Canonicalize() {
	if s == nil {
		return
	}

	if len(s.Hosts) == 0 {
		s.Hosts = nil
	}
}

func (s *ConsulIngressService) Copy() *ConsulIngressService {
	if s == nil {
		return nil
	}

	var hosts []string = nil
	if n := len(s.Hosts); n > 0 {
		hosts = make([]string, n)
		copy(hosts, s.Hosts)
	}

	return &ConsulIngressService{
		Name:  s.Name,
		Hosts: hosts,
	}
}

const (
	defaultIngressListenerProtocol = "tcp"
)

// ConsulIngressListener is used to configure a listener on a Consul Ingress
// Gateway.
type ConsulIngressListener struct {
	Port     int                     `hcl:"port,optional"`
	Protocol string                  `hcl:"protocol,optional"`
	Services []*ConsulIngressService `hcl:"service,block"`
}

func (l *ConsulIngressListener) Canonicalize() {
	if l == nil {
		return
	}

	if l.Protocol == "" {
		// same as default from consul
		l.Protocol = defaultIngressListenerProtocol
	}

	if len(l.Services) == 0 {
		l.Services = nil
	}
}

func (l *ConsulIngressListener) Copy() *ConsulIngressListener {
	if l == nil {
		return nil
	}

	var services []*ConsulIngressService = nil
	if n := len(l.Services); n > 0 {
		services = make([]*ConsulIngressService, n)
		for i := 0; i < n; i++ {
			services[i] = l.Services[i].Copy()
		}
	}

	return &ConsulIngressListener{
		Port:     l.Port,
		Protocol: l.Protocol,
		Services: services,
	}
}

// ConsulIngressConfigEntry represents the Consul Configuration Entry type for
// an Ingress Gateway.
//
// https://www.consul.io/docs/agent/config-entries/ingress-gateway#available-fields
type ConsulIngressConfigEntry struct {
	// Namespace is not yet supported.
	// Namespace string

	TLS       *ConsulGatewayTLSConfig  `hcl:"tls,block"`
	Listeners []*ConsulIngressListener `hcl:"listener,block"`
}

func (e *ConsulIngressConfigEntry) Canonicalize() {
	if e == nil {
		return
	}

	e.TLS.Canonicalize()

	if len(e.Listeners) == 0 {
		e.Listeners = nil
	}

	for _, listener := range e.Listeners {
		listener.Canonicalize()
	}
}

func (e *ConsulIngressConfigEntry) Copy() *ConsulIngressConfigEntry {
	if e == nil {
		return nil
	}

	var listeners []*ConsulIngressListener = nil
	if n := len(e.Listeners); n > 0 {
		listeners = make([]*ConsulIngressListener, n)
		for i := 0; i < n; i++ {
			listeners[i] = e.Listeners[i].Copy()
		}
	}

	return &ConsulIngressConfigEntry{
		TLS:       e.TLS.Copy(),
		Listeners: listeners,
	}
}

type ConsulLinkedService struct {
	Name     string `hcl:"name,optional"`
	CAFile   string `hcl:"ca_file,optional" mapstructure:"ca_file"`
	CertFile string `hcl:"cert_file,optional" mapstructure:"cert_file"`
	KeyFile  string `hcl:"key_file,optional" mapstructure:"key_file"`
	SNI      string `hcl:"sni,optional"`
}

func (s *ConsulLinkedService) Canonicalize() {
	// nothing to do for now
}

func (s *ConsulLinkedService) Copy() *ConsulLinkedService {
	if s == nil {
		return nil
	}

	return &ConsulLinkedService{
		Name:     s.Name,
		CAFile:   s.CAFile,
		CertFile: s.CertFile,
		KeyFile:  s.KeyFile,
		SNI:      s.SNI,
	}
}

// ConsulTerminatingConfigEntry represents the Consul Configuration Entry type
// for a Terminating Gateway.
//
// https://www.consul.io/docs/agent/config-entries/terminating-gateway#available-fields
type ConsulTerminatingConfigEntry struct {
	// Namespace is not yet supported.
	// Namespace string

	Services []*ConsulLinkedService `hcl:"service,block"`
}

func (e *ConsulTerminatingConfigEntry) Canonicalize() {
	if e == nil {
		return
	}

	if len(e.Services) == 0 {
		e.Services = nil
	}

	for _, service := range e.Services {
		service.Canonicalize()
	}
}

func (e *ConsulTerminatingConfigEntry) Copy() *ConsulTerminatingConfigEntry {
	if e == nil {
		return nil
	}

	var services []*ConsulLinkedService = nil
	if n := len(e.Services); n > 0 {
		services = make([]*ConsulLinkedService, n)
		for i := 0; i < n; i++ {
			services[i] = e.Services[i].Copy()
		}
	}

	return &ConsulTerminatingConfigEntry{
		Services: services,
	}
}

// ConsulMeshConfigEntry is a stub used to represent that the gateway service type
// should be for a Mesh Gateway. Unlike Ingress and Terminating, there is no
// actual Consul Config Entry type for mesh-gateway, at least for now. We still
// create a type for future proofing, instead just using a bool for example.
type ConsulMeshConfigEntry struct {
	// nothing in here
}

func (e *ConsulMeshConfigEntry) Canonicalize() {}

func (e *ConsulMeshConfigEntry) Copy() *ConsulMeshConfigEntry {
	if e == nil {
		return nil
	}
	return new(ConsulMeshConfigEntry)
}
