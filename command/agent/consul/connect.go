package consul

import (
	"fmt"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
)

// newConnect creates a new Consul AgentServiceConnect struct based on a Nomad
// Connect struct. If the nomad Connect struct is nil, nil will be returned to
// disable Connect for this service.
func newConnect(serviceName string, nc *structs.ConsulConnect, networks structs.Networks) (*api.AgentServiceConnect, error) {
	if nc == nil {
		// no connect stanza means there is no connect service to register
		return nil, nil
	}

	if nc.IsNative() {
		return &api.AgentServiceConnect{Native: true}, nil
	}

	sidecarReg, err := connectSidecarRegistration(serviceName, nc.SidecarService, networks)
	if err != nil {
		return nil, err
	}

	return &api.AgentServiceConnect{
		Native:         false,
		SidecarService: sidecarReg,
	}, nil
}

func connectSidecarRegistration(serviceName string, css *structs.ConsulSidecarService, networks structs.Networks) (*api.AgentServiceRegistration, error) {
	if css == nil {
		// no sidecar stanza means there is no sidecar service to register
		return nil, nil
	}

	cNet, cPort, err := connectPort(serviceName, networks)
	if err != nil {
		return nil, err
	}

	proxy, err := connectProxy(css.Proxy, cPort.To, networks)
	if err != nil {
		return nil, err
	}

	return &api.AgentServiceRegistration{
		Tags:    helper.CopySliceString(css.Tags),
		Port:    cPort.Value,
		Address: cNet.IP,
		Proxy:   proxy,
	}, nil
}

func connectProxy(proxy *structs.ConsulProxy, cPort int, networks structs.Networks) (*api.AgentServiceConnectProxyConfig, error) {
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
		Config:              connectProxyConfig(proxy.Config, cPort),
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
			DestinationName: upstream.DestinationName,
			LocalBindPort:   upstream.LocalBindPort,
		}
	}
	return upstreams
}

func connectProxyConfig(cfg map[string]interface{}, port int) map[string]interface{} {
	if cfg == nil {
		cfg = make(map[string]interface{})
	}
	cfg["bind_address"] = "0.0.0.0"
	cfg["bind_port"] = port
	return cfg
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
func connectPort(serviceName string, networks structs.Networks) (*structs.NetworkResource, structs.Port, error) {
	if err := connectNetworkInvariants(networks); err != nil {
		return nil, structs.Port{}, err
	}

	port, ok := networks[0].PortForService(serviceName)
	if !ok {
		return nil, structs.Port{}, fmt.Errorf("No Connect port defined for service %q", serviceName)
	}

	return networks[0], port, nil
}

// connectExposePathPort returns the port for the exposed path for the exposed
// proxy path.
func connectExposePathPort(portLabel string, networks structs.Networks) (string, int, error) {
	if err := connectNetworkInvariants(networks); err != nil {
		return "", 0, err
	}

	ip, port := networks.Port(portLabel)
	if port == 0 {
		return "", 0, fmt.Errorf("No port of label %q defined", portLabel)
	}

	return ip, port, nil
}
