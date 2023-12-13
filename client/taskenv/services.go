// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskenv

import (
	"github.com/hashicorp/nomad/nomad/structs"
)

// InterpolateServices returns an interpolated copy of services and checks with
// values from the task's environment.
func InterpolateServices(taskEnv *TaskEnv, services []*structs.Service) []*structs.Service {
	// Guard against not having a valid taskEnv. This can be the case if the
	// PreKilling or Exited hook is run before Poststart.
	if taskEnv == nil || len(services) == 0 {
		return nil
	}

	interpolated := make([]*structs.Service, len(services))

	for i, service := range services {
		interpolated[i] = InterpolateService(taskEnv, service)
	}

	return interpolated
}

func InterpolateService(taskEnv *TaskEnv, origService *structs.Service) *structs.Service {
	// Guard against not having a valid taskEnv. This can be the case if the
	// PreKilling or Exited hook is run before Poststart.
	if taskEnv == nil || origService == nil {
		return nil
	}

	// Create a copy as we need to re-interpolate every time the
	// environment changes.
	service := origService.Copy()

	for _, check := range service.Checks {
		check.Name = taskEnv.ReplaceEnv(check.Name)
		check.Type = taskEnv.ReplaceEnv(check.Type)
		check.Command = taskEnv.ReplaceEnv(check.Command)
		check.Args = taskEnv.ParseAndReplace(check.Args)
		check.Path = taskEnv.ReplaceEnv(check.Path)
		check.Protocol = taskEnv.ReplaceEnv(check.Protocol)
		check.PortLabel = taskEnv.ReplaceEnv(check.PortLabel)
		check.InitialStatus = taskEnv.ReplaceEnv(check.InitialStatus)
		check.Method = taskEnv.ReplaceEnv(check.Method)
		check.GRPCService = taskEnv.ReplaceEnv(check.GRPCService)
		check.Header = interpolateMapStringSliceString(taskEnv, check.Header)
	}

	service.Name = taskEnv.ReplaceEnv(service.Name)
	service.PortLabel = taskEnv.ReplaceEnv(service.PortLabel)
	service.Address = taskEnv.ReplaceEnv(service.Address)
	service.Tags = taskEnv.ParseAndReplace(service.Tags)
	service.CanaryTags = taskEnv.ParseAndReplace(service.CanaryTags)
	service.Meta = interpolateMapStringString(taskEnv, service.Meta)
	service.CanaryMeta = interpolateMapStringString(taskEnv, service.CanaryMeta)
	service.TaggedAddresses = interpolateMapStringString(taskEnv, service.TaggedAddresses)
	interpolateConnect(taskEnv, service.Connect)

	return service
}

func interpolateMapStringSliceString(taskEnv *TaskEnv, orig map[string][]string) map[string][]string {
	if len(orig) == 0 {
		return nil
	}

	m := make(map[string][]string, len(orig))
	for k, vs := range orig {
		m[taskEnv.ReplaceEnv(k)] = taskEnv.ParseAndReplace(vs)
	}
	return m
}

func interpolateMapStringString(taskEnv *TaskEnv, orig map[string]string) map[string]string {
	if len(orig) == 0 {
		return nil
	}

	m := make(map[string]string, len(orig))
	for k, v := range orig {
		m[taskEnv.ReplaceEnv(k)] = taskEnv.ReplaceEnv(v)
	}
	return m
}

func interpolateMapStringInterface(taskEnv *TaskEnv, orig map[string]any) map[string]any {
	if len(orig) == 0 {
		return nil
	}

	m := make(map[string]any, len(orig))
	for k, v := range orig {
		envK := taskEnv.ReplaceEnv(k)
		if vStr, ok := v.(string); ok {
			m[envK] = taskEnv.ReplaceEnv(vStr)
		} else {
			m[envK] = v
		}
	}
	return m
}

func interpolateConnect(taskEnv *TaskEnv, connect *structs.ConsulConnect) {
	if connect == nil {
		return
	}

	interpolateConnectSidecarService(taskEnv, connect.SidecarService)
	interpolateConnectSidecarTask(taskEnv, connect.SidecarTask)
	if connect.Gateway != nil {
		interpolateConnectGatewayProxy(taskEnv, connect.Gateway.Proxy)
		interpolateConnectGatewayIngress(taskEnv, connect.Gateway.Ingress)
	}
}

func interpolateConnectGatewayProxy(taskEnv *TaskEnv, proxy *structs.ConsulGatewayProxy) {
	if proxy == nil {
		return
	}

	m := make(map[string]*structs.ConsulGatewayBindAddress, len(proxy.EnvoyGatewayBindAddresses))
	for k, v := range proxy.EnvoyGatewayBindAddresses {
		m[taskEnv.ReplaceEnv(k)] = &structs.ConsulGatewayBindAddress{
			Address: taskEnv.ReplaceEnv(v.Address),
			Port:    v.Port,
		}
	}

	proxy.EnvoyGatewayBindAddresses = m
	proxy.Config = interpolateMapStringInterface(taskEnv, proxy.Config)
}

func interpolateConnectGatewayIngress(taskEnv *TaskEnv, ingress *structs.ConsulIngressConfigEntry) {
	if ingress == nil {
		return
	}

	for _, listener := range ingress.Listeners {
		listener.Protocol = taskEnv.ReplaceEnv(listener.Protocol)
		for _, service := range listener.Services {
			service.Name = taskEnv.ReplaceEnv(service.Name)
			service.Hosts = taskEnv.ParseAndReplace(service.Hosts)
		}
	}
}

func interpolateConnectSidecarService(taskEnv *TaskEnv, sidecar *structs.ConsulSidecarService) {
	if sidecar == nil {
		return
	}

	sidecar.Port = taskEnv.ReplaceEnv(sidecar.Port)
	sidecar.Tags = taskEnv.ParseAndReplace(sidecar.Tags)
	if sidecar.Proxy != nil {
		sidecar.Proxy.LocalServiceAddress = taskEnv.ReplaceEnv(sidecar.Proxy.LocalServiceAddress)
		if sidecar.Proxy.Expose != nil {
			for i := 0; i < len(sidecar.Proxy.Expose.Paths); i++ {
				sidecar.Proxy.Expose.Paths[i].Protocol = taskEnv.ReplaceEnv(sidecar.Proxy.Expose.Paths[i].Protocol)
				sidecar.Proxy.Expose.Paths[i].ListenerPort = taskEnv.ReplaceEnv(sidecar.Proxy.Expose.Paths[i].ListenerPort)
				sidecar.Proxy.Expose.Paths[i].Path = taskEnv.ReplaceEnv(sidecar.Proxy.Expose.Paths[i].Path)
			}
		}
		for i := 0; i < len(sidecar.Proxy.Upstreams); i++ {
			sidecar.Proxy.Upstreams[i].Datacenter = taskEnv.ReplaceEnv(sidecar.Proxy.Upstreams[i].Datacenter)
			sidecar.Proxy.Upstreams[i].DestinationName = taskEnv.ReplaceEnv(sidecar.Proxy.Upstreams[i].DestinationName)
			sidecar.Proxy.Upstreams[i].LocalBindAddress = taskEnv.ReplaceEnv(sidecar.Proxy.Upstreams[i].LocalBindAddress)
			sidecar.Proxy.Upstreams[i].Config = interpolateMapStringInterface(taskEnv, sidecar.Proxy.Upstreams[i].Config)
		}
		sidecar.Proxy.Config = interpolateMapStringInterface(taskEnv, sidecar.Proxy.Config)
	}
}

func interpolateConnectSidecarTask(taskEnv *TaskEnv, task *structs.SidecarTask) {
	if task == nil {
		return
	}

	task.Driver = taskEnv.ReplaceEnv(task.Driver)
	task.Config = interpolateMapStringInterface(taskEnv, task.Config)
	task.Env = interpolateMapStringString(taskEnv, task.Env)
	task.KillSignal = taskEnv.ReplaceEnv(task.KillSignal)
	task.Meta = interpolateMapStringString(taskEnv, task.Meta)
	interpolateTaskResources(taskEnv, task.Resources)
	task.User = taskEnv.ReplaceEnv(task.User)
}

func interpolateTaskResources(taskEnv *TaskEnv, resources *structs.Resources) {
	if resources == nil {
		return
	}

	for i := 0; i < len(resources.Devices); i++ {
		resources.Devices[i].Name = taskEnv.ReplaceEnv(resources.Devices[i].Name)
		// do not interpolate constraints & affinities
	}

	for i := 0; i < len(resources.Networks); i++ {
		resources.Networks[i].CIDR = taskEnv.ReplaceEnv(resources.Networks[i].CIDR)
		resources.Networks[i].Device = taskEnv.ReplaceEnv(resources.Networks[i].Device)
		resources.Networks[i].IP = taskEnv.ReplaceEnv(resources.Networks[i].IP)
		resources.Networks[i].Mode = taskEnv.ReplaceEnv(resources.Networks[i].Mode)

		if resources.Networks[i].DNS != nil {
			resources.Networks[i].DNS.Options = taskEnv.ParseAndReplace(resources.Networks[i].DNS.Options)
			resources.Networks[i].DNS.Searches = taskEnv.ParseAndReplace(resources.Networks[i].DNS.Searches)
			resources.Networks[i].DNS.Servers = taskEnv.ParseAndReplace(resources.Networks[i].DNS.Servers)
		}

		for p := 0; p < len(resources.Networks[i].DynamicPorts); p++ {
			resources.Networks[i].DynamicPorts[p].HostNetwork = taskEnv.ReplaceEnv(resources.Networks[i].DynamicPorts[p].HostNetwork)
			resources.Networks[i].DynamicPorts[p].Label = taskEnv.ReplaceEnv(resources.Networks[i].DynamicPorts[p].Label)
		}

		for p := 0; p < len(resources.Networks[i].ReservedPorts); p++ {
			resources.Networks[i].ReservedPorts[p].HostNetwork = taskEnv.ReplaceEnv(resources.Networks[i].ReservedPorts[p].HostNetwork)
			resources.Networks[i].ReservedPorts[p].Label = taskEnv.ReplaceEnv(resources.Networks[i].ReservedPorts[p].Label)
		}
	}
}
