package consul

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/nomad/structs"
	"golang.org/x/exp/slices"
)

// newConnect creates a new Consul AgentServiceConnect struct based on a Nomad
// Connect struct. If the nomad Connect struct is nil, nil will be returned to
// disable Connect for this service.
func newConnect(serviceID, allocID string, serviceName string, nc *structs.ConsulConnect, networks structs.Networks, ports structs.AllocatedPorts) (*api.AgentServiceConnect, error) {
	switch {
	case nc == nil:
		// no connect stanza means there is no connect service to register
		return nil, nil

	case nc.IsGateway():
		// gateway settings are configured on the service block on the consul side
		return nil, nil

	case nc.IsNative():
		// the service is connect native
		return &api.AgentServiceConnect{Native: true}, nil

	case nc.HasSidecar():
		// must register the sidecar for this service
		if nc.SidecarService.Port == "" {
			nc.SidecarService.Port = fmt.Sprintf("%s-%s", structs.ConnectProxyPrefix, serviceName)
		}
		sidecarReg, err := connectSidecarRegistration(serviceID, allocID, nc.SidecarService, networks, ports)
		if err != nil {
			return nil, err
		}
		return &api.AgentServiceConnect{SidecarService: sidecarReg}, nil

	default:
		// a non-nil but empty connect block makes no sense
		return nil, fmt.Errorf("Connect configuration empty for service %s", serviceName)
	}
}

// newConnectGateway creates a new Consul AgentServiceConnectProxyConfig struct based on
// a Nomad Connect struct. If the Nomad Connect struct does not contain a gateway, nil
// will be returned as this service is not a gateway.
func newConnectGateway(connect *structs.ConsulConnect) *api.AgentServiceConnectProxyConfig {
	if !connect.IsGateway() {
		return nil
	}

	var envoyConfig map[string]interface{}

	// Populate the envoy configuration from the gateway.proxy stanza, if
	// such configuration is provided.
	if proxy := connect.Gateway.Proxy; proxy != nil {
		envoyConfig = make(map[string]interface{})

		if len(proxy.EnvoyGatewayBindAddresses) > 0 {
			envoyConfig["envoy_gateway_bind_addresses"] = proxy.EnvoyGatewayBindAddresses
		}

		if proxy.EnvoyGatewayNoDefaultBind {
			envoyConfig["envoy_gateway_no_default_bind"] = true
		}

		if proxy.EnvoyGatewayBindTaggedAddresses {
			envoyConfig["envoy_gateway_bind_tagged_addresses"] = true
		}

		if proxy.EnvoyDNSDiscoveryType != "" {
			envoyConfig["envoy_dns_discovery_type"] = proxy.EnvoyDNSDiscoveryType
		}

		if proxy.ConnectTimeout != nil {
			envoyConfig["connect_timeout_ms"] = proxy.ConnectTimeout.Milliseconds()
		}

		if len(proxy.Config) > 0 {
			for k, v := range proxy.Config {
				envoyConfig[k] = v
			}
		}
	}

	return &api.AgentServiceConnectProxyConfig{Config: envoyConfig}
}

func connectSidecarRegistration(serviceID, allocID string, css *structs.ConsulSidecarService, networks structs.Networks, ports structs.AllocatedPorts) (*api.AgentServiceRegistration, error) {
	if css == nil {
		// no sidecar stanza means there is no sidecar service to register
		return nil, nil
	}

	cMapping, err := connectPort(css.Port, networks, ports)
	if err != nil {
		return nil, err
	}

	proxy, err := connectSidecarProxy(allocID, css.Proxy, cMapping.To, networks)
	if err != nil {
		return nil, err
	}

	// if the service has a TCP check that's failing, we need an alias to
	// ensure service discovery excludes this sidecar from queries
	// (ex. in the case of Connect upstreams)
	checks := api.AgentServiceChecks{{
		Name:         "Connect Sidecar Aliasing " + serviceID,
		AliasService: serviceID,
	}}
	if !css.DisableDefaultTCPCheck {
		checks = append(checks, &api.AgentServiceCheck{
			Name:     "Connect Sidecar Listening",
			TCP:      net.JoinHostPort(cMapping.HostIP, strconv.Itoa(cMapping.Value)),
			Interval: "10s",
		})
	}

	return &api.AgentServiceRegistration{
		Tags:    slices.Clone(css.Tags),
		Port:    cMapping.Value,
		Address: cMapping.HostIP,
		Proxy:   proxy,
		Checks:  checks,
	}, nil
}

func connectSidecarProxy(allocID string, proxy *structs.ConsulProxy, cPort int, networks structs.Networks) (*api.AgentServiceConnectProxyConfig, error) {
	if proxy == nil {
		proxy = new(structs.ConsulProxy)
	}

	expose, err := connectProxyExpose(proxy.Expose, networks)
	if err != nil {
		return nil, err
	}

	return &api.AgentServiceConnectProxyConfig{
		LocalServiceAddress: proxy.LocalServiceAddress,
		LocalServicePort:    proxy.LocalServicePort,
		Config:              connectProxyConfig(proxy.Config, cPort, allocID),
		Upstreams:           connectUpstreams(proxy.Upstreams),
		Expose:              expose,
	}, nil
}

func connectProxyExpose(expose *structs.ConsulExposeConfig, networks structs.Networks) (api.ExposeConfig, error) {
	if expose == nil {
		return api.ExposeConfig{}, nil
	}

	paths, err := connectProxyExposePaths(expose.Paths, networks)
	if err != nil {
		return api.ExposeConfig{}, err
	}

	return api.ExposeConfig{
		Checks: false,
		Paths:  paths,
	}, nil
}

func connectProxyExposePaths(in []structs.ConsulExposePath, networks structs.Networks) ([]api.ExposePath, error) {
	if len(in) == 0 {
		return nil, nil
	}

	paths := make([]api.ExposePath, len(in))
	for i, path := range in {
		if _, exposedPort, err := connectExposePathPort(path.ListenerPort, networks); err != nil {
			return nil, err
		} else {
			paths[i] = api.ExposePath{
				ListenerPort:    exposedPort,
				Path:            path.Path,
				LocalPathPort:   path.LocalPathPort,
				Protocol:        path.Protocol,
				ParsedFromCheck: false,
			}
		}
	}
	return paths, nil
}

func connectUpstreams(in []structs.ConsulUpstream) []api.Upstream {
	if len(in) == 0 {
		return nil
	}

	upstreams := make([]api.Upstream, len(in))
	for i, upstream := range in {
		upstreams[i] = api.Upstream{
			DestinationName:      upstream.DestinationName,
			DestinationNamespace: upstream.DestinationNamespace,
			LocalBindPort:        upstream.LocalBindPort,
			Datacenter:           upstream.Datacenter,
			LocalBindAddress:     upstream.LocalBindAddress,
			MeshGateway:          connectMeshGateway(upstream.MeshGateway),
		}
	}
	return upstreams
}

// connectMeshGateway creates an api.MeshGatewayConfig from the nomad upstream
// block. A non-existent config or unsupported gateway mode will default to the
// Consul default mode.
func connectMeshGateway(in *structs.ConsulMeshGateway) api.MeshGatewayConfig {
	gw := api.MeshGatewayConfig{
		Mode: api.MeshGatewayModeDefault,
	}

	if in == nil {
		return gw
	}

	switch in.Mode {
	case "local":
		gw.Mode = api.MeshGatewayModeLocal
	case "remote":
		gw.Mode = api.MeshGatewayModeRemote
	case "none":
		gw.Mode = api.MeshGatewayModeNone
	}

	return gw
}

func connectProxyConfig(cfg map[string]interface{}, port int, allocID string) map[string]interface{} {
	if cfg == nil {
		cfg = make(map[string]interface{})
	}
	cfg["bind_address"] = "0.0.0.0"
	cfg["bind_port"] = port
	injectAllocID(cfg, allocID)
	return cfg
}

// injectAllocID merges allocID into cfg=>envoy_stats_tags
//
// cfg must not be nil
func injectAllocID(cfg map[string]interface{}, allocID string) {
	const key = "envoy_stats_tags"
	const prefix = "nomad.alloc_id="
	pair := prefix + allocID
	tags, exists := cfg[key]
	if !exists {
		cfg[key] = []string{pair}
		return
	}

	switch v := tags.(type) {
	case []string:
		// scan the existing tags to see if alloc_id= is already set
		for _, s := range v {
			if strings.HasPrefix(s, prefix) {
				return
			}
		}
		v = append(v, pair)
		cfg[key] = v
	}
}

func connectNetworkInvariants(networks structs.Networks) error {
	if n := len(networks); n != 1 {
		return fmt.Errorf("Connect only supported with exactly 1 network (found %d)", n)
	}
	return nil
}

// connectPort returns the network and port for the Connect proxy sidecar
// defined for this service. An error is returned if the network and port
// cannot be determined.
func connectPort(portLabel string, networks structs.Networks, ports structs.AllocatedPorts) (structs.AllocatedPortMapping, error) {
	if err := connectNetworkInvariants(networks); err != nil {
		return structs.AllocatedPortMapping{}, err
	}
	mapping, ok := ports.Get(portLabel)
	if !ok {
		mapping = networks.Port(portLabel)
		if mapping.Value > 0 {
			return mapping, nil
		}
		return structs.AllocatedPortMapping{}, fmt.Errorf("No port of label %q defined", portLabel)
	}
	return mapping, nil
}

// connectExposePathPort returns the port for the exposed path for the exposed
// proxy path.
func connectExposePathPort(portLabel string, networks structs.Networks) (string, int, error) {
	if err := connectNetworkInvariants(networks); err != nil {
		return "", 0, err
	}

	mapping := networks.Port(portLabel)
	if mapping.Value == 0 {
		return "", 0, fmt.Errorf("No port of label %q defined", portLabel)
	}

	return mapping.HostIP, mapping.Value, nil
}
