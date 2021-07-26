package jobspec

import (
	"fmt"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/hcl"
	"github.com/hashicorp/hcl/hcl/ast"
	"github.com/hashicorp/nomad/api"
	"github.com/mitchellh/mapstructure"
)

func parseGroupServices(g *api.TaskGroup, serviceObjs *ast.ObjectList) error {
	g.Services = make([]*api.Service, len(serviceObjs.Items))
	for idx, o := range serviceObjs.Items {
		service, err := parseService(o)
		if err != nil {
			return multierror.Prefix(err, fmt.Sprintf("service (%d):", idx))
		}
		g.Services[idx] = service
	}

	return nil
}

func parseServices(serviceObjs *ast.ObjectList) ([]*api.Service, error) {
	services := make([]*api.Service, len(serviceObjs.Items))
	for idx, o := range serviceObjs.Items {
		service, err := parseService(o)
		if err != nil {
			return nil, multierror.Prefix(err, fmt.Sprintf("service (%d):", idx))
		}
		services[idx] = service
	}
	return services, nil
}

func parseService(o *ast.ObjectItem) (*api.Service, error) {
	// Check for invalid keys
	valid := []string{
		"name",
		"tags",
		"canary_tags",
		"enable_tag_override",
		"port",
		"check",
		"address_mode",
		"check_restart",
		"connect",
		"task",
		"meta",
		"canary_meta",
		"on_update",
	}
	if err := checkHCLKeys(o.Val, valid); err != nil {
		return nil, err
	}

	var service api.Service
	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, o.Val); err != nil {
		return nil, err
	}

	delete(m, "check")
	delete(m, "check_restart")
	delete(m, "connect")
	delete(m, "meta")
	delete(m, "canary_meta")

	if err := mapstructure.WeakDecode(m, &service); err != nil {
		return nil, err
	}

	// Filter list
	var listVal *ast.ObjectList
	if ot, ok := o.Val.(*ast.ObjectType); ok {
		listVal = ot.List
	} else {
		return nil, fmt.Errorf("'%s': should be an object", service.Name)
	}

	if co := listVal.Filter("check"); len(co.Items) > 0 {
		if err := parseChecks(&service, co); err != nil {
			return nil, multierror.Prefix(err, fmt.Sprintf("'%s',", service.Name))
		}
	}

	// Filter check_restart
	if cro := listVal.Filter("check_restart"); len(cro.Items) > 0 {
		if len(cro.Items) > 1 {
			return nil, fmt.Errorf("check_restart '%s': cannot have more than 1 check_restart", service.Name)
		}
		cr, err := parseCheckRestart(cro.Items[0])
		if err != nil {
			return nil, multierror.Prefix(err, fmt.Sprintf("'%s',", service.Name))
		}
		service.CheckRestart = cr

	}

	// Filter connect
	if co := listVal.Filter("connect"); len(co.Items) > 0 {
		if len(co.Items) > 1 {
			return nil, fmt.Errorf("connect '%s': cannot have more than 1 connect stanza", service.Name)
		}

		c, err := parseConnect(co.Items[0])
		if err != nil {
			return nil, multierror.Prefix(err, fmt.Sprintf("'%s',", service.Name))
		}

		service.Connect = c
	}

	// Parse out meta fields. These are in HCL as a list so we need
	// to iterate over them and merge them.
	if metaO := listVal.Filter("meta"); len(metaO.Items) > 0 {
		for _, o := range metaO.Elem().Items {
			var m map[string]interface{}
			if err := hcl.DecodeObject(&m, o.Val); err != nil {
				return nil, err
			}
			if err := mapstructure.WeakDecode(m, &service.Meta); err != nil {
				return nil, err
			}
		}
	}

	// Parse out canary_meta fields. These are in HCL as a list so we need
	// to iterate over them and merge them.
	if metaO := listVal.Filter("canary_meta"); len(metaO.Items) > 0 {
		for _, o := range metaO.Elem().Items {
			var m map[string]interface{}
			if err := hcl.DecodeObject(&m, o.Val); err != nil {
				return nil, err
			}
			if err := mapstructure.WeakDecode(m, &service.CanaryMeta); err != nil {
				return nil, err
			}
		}
	}

	return &service, nil
}

func parseConnect(co *ast.ObjectItem) (*api.ConsulConnect, error) {
	valid := []string{
		"native",
		"gateway",
		"sidecar_service",
		"sidecar_task",
	}

	if err := checkHCLKeys(co.Val, valid); err != nil {
		return nil, multierror.Prefix(err, "connect ->")
	}

	var connect api.ConsulConnect
	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, co.Val); err != nil {
		return nil, err
	}

	delete(m, "gateway")
	delete(m, "sidecar_service")
	delete(m, "sidecar_task")

	if err := mapstructure.WeakDecode(m, &connect); err != nil {
		return nil, err
	}

	var connectList *ast.ObjectList
	if ot, ok := co.Val.(*ast.ObjectType); ok {
		connectList = ot.List
	} else {
		return nil, fmt.Errorf("connect should be an object")
	}

	// Parse the gateway
	o := connectList.Filter("gateway")
	if len(o.Items) > 1 {
		return nil, fmt.Errorf("only one 'gateway' block allowed per task")
	} else if len(o.Items) == 1 {
		g, err := parseGateway(o.Items[0])
		if err != nil {
			return nil, fmt.Errorf("gateway, %v", err)
		}
		connect.Gateway = g
	}

	// Parse the sidecar_service
	o = connectList.Filter("sidecar_service")
	if len(o.Items) == 0 {
		return &connect, nil
	}
	if len(o.Items) > 1 {
		return nil, fmt.Errorf("only one 'sidecar_service' block allowed per task")
	}

	r, err := parseSidecarService(o.Items[0])
	if err != nil {
		return nil, fmt.Errorf("sidecar_service, %v", err)
	}
	connect.SidecarService = r

	// Parse the sidecar_task
	o = connectList.Filter("sidecar_task")
	if len(o.Items) == 0 {
		return &connect, nil
	}
	if len(o.Items) > 1 {
		return nil, fmt.Errorf("only one 'sidecar_task' block allowed per task")
	}

	t, err := parseSidecarTask(o.Items[0])
	if err != nil {
		return nil, fmt.Errorf("sidecar_task, %v", err)
	}
	connect.SidecarTask = t

	return &connect, nil
}

func parseGateway(o *ast.ObjectItem) (*api.ConsulGateway, error) {
	valid := []string{
		"proxy",
		"ingress",
		"terminating",
		"mesh",
	}

	if err := checkHCLKeys(o.Val, valid); err != nil {
		return nil, multierror.Prefix(err, "gateway ->")
	}

	var gateway api.ConsulGateway
	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, o.Val); err != nil {
		return nil, err
	}

	delete(m, "proxy")
	delete(m, "ingress")
	delete(m, "terminating")
	delete(m, "mesh")

	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook:       mapstructure.StringToTimeDurationHookFunc(),
		WeaklyTypedInput: true,
		Result:           &gateway,
	})
	if err != nil {
		return nil, err
	}
	if err := dec.Decode(m); err != nil {
		return nil, fmt.Errorf("gateway: %v", err)
	}

	// list of parameters
	var listVal *ast.ObjectList
	if ot, ok := o.Val.(*ast.ObjectType); ok {
		listVal = ot.List
	} else {
		return nil, fmt.Errorf("proxy: should be an object")
	}

	// extract and parse the proxy block
	po := listVal.Filter("proxy")
	if len(po.Items) != 1 {
		return nil, fmt.Errorf("must have one 'proxy' block")
	}
	proxy, err := parseGatewayProxy(po.Items[0])
	if err != nil {
		return nil, fmt.Errorf("proxy, %v", err)
	}
	gateway.Proxy = proxy

	// extract and parse the ingress block
	if io := listVal.Filter("ingress"); len(io.Items) > 0 {
		if len(io.Items) > 1 {
			return nil, fmt.Errorf("ingress, %s", "multiple ingress stanzas not allowed")
		}

		ingress, err := parseIngressConfigEntry(io.Items[0])
		if err != nil {
			return nil, fmt.Errorf("ingress, %v", err)
		}
		gateway.Ingress = ingress
	}

	if to := listVal.Filter("terminating"); len(to.Items) > 0 {
		if len(to.Items) > 1 {
			return nil, fmt.Errorf("terminating, %s", "multiple terminating stanzas not allowed")
		}

		terminating, err := parseTerminatingConfigEntry(to.Items[0])
		if err != nil {
			return nil, fmt.Errorf("terminating, %v", err)
		}
		gateway.Terminating = terminating
	}

	if mo := listVal.Filter("mesh"); len(mo.Items) > 0 {
		if len(mo.Items) > 1 {
			return nil, fmt.Errorf("mesh, %s", "multiple mesh stanzas not allowed")
		}

		// mesh should have no keys
		if err := checkHCLKeys(mo.Items[0].Val, []string{}); err != nil {
			return nil, fmt.Errorf("mesh, %s", err)
		}

		gateway.Mesh = &api.ConsulMeshConfigEntry{}
	}

	return &gateway, nil
}

// parseGatewayProxy parses envoy gateway proxy options supported by Consul.
//
// consul.io/docs/connect/proxies/envoy#gateway-options
func parseGatewayProxy(o *ast.ObjectItem) (*api.ConsulGatewayProxy, error) {
	valid := []string{
		"connect_timeout",
		"envoy_gateway_bind_tagged_addresses",
		"envoy_gateway_bind_addresses",
		"envoy_gateway_no_default_bind",
		"envoy_dns_discovery_type",
		"config",
	}

	if err := checkHCLKeys(o.Val, valid); err != nil {
		return nil, multierror.Prefix(err, "proxy ->")
	}

	var proxy api.ConsulGatewayProxy
	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, o.Val); err != nil {
		return nil, err
	}

	delete(m, "config")
	delete(m, "envoy_gateway_bind_addresses")

	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook:       mapstructure.StringToTimeDurationHookFunc(),
		WeaklyTypedInput: true,
		Result:           &proxy,
	})
	if err != nil {
		return nil, err
	}
	if err := dec.Decode(m); err != nil {
		return nil, fmt.Errorf("proxy: %v", err)
	}

	var listVal *ast.ObjectList
	if ot, ok := o.Val.(*ast.ObjectType); ok {
		listVal = ot.List
	} else {
		return nil, fmt.Errorf("proxy: should be an object")
	}

	// need to parse envoy_gateway_bind_addresses if present

	if ebo := listVal.Filter("envoy_gateway_bind_addresses"); len(ebo.Items) > 0 {
		proxy.EnvoyGatewayBindAddresses = make(map[string]*api.ConsulGatewayBindAddress)
		for _, listenerM := range ebo.Items { // object item, each listener object
			listenerName := listenerM.Keys[0].Token.Value().(string)

			var listenerListVal *ast.ObjectList
			if ot, ok := listenerM.Val.(*ast.ObjectType); ok {
				listenerListVal = ot.List
			} else {
				return nil, fmt.Errorf("listener: should be an object")
			}

			var bind api.ConsulGatewayBindAddress
			if err := hcl.DecodeObject(&bind, listenerListVal); err != nil {
				return nil, fmt.Errorf("port: should be an int")
			}
			bind.Name = listenerName
			proxy.EnvoyGatewayBindAddresses[listenerName] = &bind
		}
	}

	// need to parse the opaque config if present

	if co := listVal.Filter("config"); len(co.Items) > 1 {
		return nil, fmt.Errorf("only 1 meta object supported")
	} else if len(co.Items) == 1 {
		var mSlice []map[string]interface{}
		if err := hcl.DecodeObject(&mSlice, co.Items[0].Val); err != nil {
			return nil, err
		}

		if len(mSlice) > 1 {
			return nil, fmt.Errorf("only 1 meta object supported")
		}

		m := mSlice[0]

		if err := mapstructure.WeakDecode(m, &proxy.Config); err != nil {
			return nil, err
		}

		proxy.Config = flattenMapSlice(proxy.Config)
	}

	return &proxy, nil
}

func parseConsulIngressService(o *ast.ObjectItem) (*api.ConsulIngressService, error) {
	valid := []string{
		"name",
		"hosts",
	}

	if err := checkHCLKeys(o.Val, valid); err != nil {
		return nil, multierror.Prefix(err, "service ->")
	}

	var service api.ConsulIngressService
	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, o.Val); err != nil {
		return nil, err
	}

	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result: &service,
	})
	if err != nil {
		return nil, err
	}

	if err := dec.Decode(m); err != nil {
		return nil, err
	}

	return &service, nil
}

func parseConsulLinkedService(o *ast.ObjectItem) (*api.ConsulLinkedService, error) {
	valid := []string{
		"name",
		"ca_file",
		"cert_file",
		"key_file",
		"sni",
	}

	if err := checkHCLKeys(o.Val, valid); err != nil {
		return nil, multierror.Prefix(err, "service ->")
	}

	var service api.ConsulLinkedService
	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, o.Val); err != nil {
		return nil, err
	}

	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result: &service,
	})
	if err != nil {
		return nil, err
	}

	if err := dec.Decode(m); err != nil {
		return nil, err
	}

	return &service, nil
}

func parseConsulIngressListener(o *ast.ObjectItem) (*api.ConsulIngressListener, error) {
	valid := []string{
		"port",
		"protocol",
		"service",
	}

	if err := checkHCLKeys(o.Val, valid); err != nil {
		return nil, multierror.Prefix(err, "listener ->")
	}

	var listener api.ConsulIngressListener
	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, o.Val); err != nil {
		return nil, err
	}

	delete(m, "service")

	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result: &listener,
	})
	if err != nil {
		return nil, err
	}

	if err := dec.Decode(m); err != nil {
		return nil, err
	}

	// Parse services

	var listVal *ast.ObjectList
	if ot, ok := o.Val.(*ast.ObjectType); ok {
		listVal = ot.List
	} else {
		return nil, fmt.Errorf("listener: should be an object")
	}

	so := listVal.Filter("service")
	if len(so.Items) > 0 {
		listener.Services = make([]*api.ConsulIngressService, len(so.Items))
		for i := range so.Items {
			is, err := parseConsulIngressService(so.Items[i])
			if err != nil {
				return nil, err
			}
			listener.Services[i] = is
		}
	}
	return &listener, nil
}

func parseConsulGatewayTLS(o *ast.ObjectItem) (*api.ConsulGatewayTLSConfig, error) {
	valid := []string{
		"enabled",
	}

	if err := checkHCLKeys(o.Val, valid); err != nil {
		return nil, multierror.Prefix(err, "tls ->")
	}

	var tls api.ConsulGatewayTLSConfig
	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, o.Val); err != nil {
		return nil, err
	}

	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result: &tls,
	})
	if err != nil {
		return nil, err
	}

	if err := dec.Decode(m); err != nil {
		return nil, err
	}

	return &tls, nil
}

func parseIngressConfigEntry(o *ast.ObjectItem) (*api.ConsulIngressConfigEntry, error) {
	valid := []string{
		"tls",
		"listener",
	}

	if err := checkHCLKeys(o.Val, valid); err != nil {
		return nil, multierror.Prefix(err, "ingress ->")
	}

	var ingress api.ConsulIngressConfigEntry
	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, o.Val); err != nil {
		return nil, err
	}

	delete(m, "tls")
	delete(m, "listener")

	// Parse tls and listener(s)

	var listVal *ast.ObjectList
	if ot, ok := o.Val.(*ast.ObjectType); ok {
		listVal = ot.List
	} else {
		return nil, fmt.Errorf("ingress: should be an object")
	}

	if to := listVal.Filter("tls"); len(to.Items) > 1 {
		return nil, fmt.Errorf("only 1 tls object supported")
	} else if len(to.Items) == 1 {
		if tls, err := parseConsulGatewayTLS(to.Items[0]); err != nil {
			return nil, err
		} else {
			ingress.TLS = tls
		}
	}

	lo := listVal.Filter("listener")
	if len(lo.Items) > 0 {
		ingress.Listeners = make([]*api.ConsulIngressListener, len(lo.Items))
		for i := range lo.Items {
			listener, err := parseConsulIngressListener(lo.Items[i])
			if err != nil {
				return nil, err
			}
			ingress.Listeners[i] = listener
		}
	}

	return &ingress, nil
}

func parseTerminatingConfigEntry(o *ast.ObjectItem) (*api.ConsulTerminatingConfigEntry, error) {
	valid := []string{
		"service",
	}

	if err := checkHCLKeys(o.Val, valid); err != nil {
		return nil, multierror.Prefix(err, "terminating ->")
	}

	var terminating api.ConsulTerminatingConfigEntry

	// Parse service(s)

	var listVal *ast.ObjectList
	if ot, ok := o.Val.(*ast.ObjectType); ok {
		listVal = ot.List
	} else {
		return nil, fmt.Errorf("terminating: should be an object")
	}

	lo := listVal.Filter("service")
	if len(lo.Items) > 0 {
		terminating.Services = make([]*api.ConsulLinkedService, len(lo.Items))
		for i := range lo.Items {
			service, err := parseConsulLinkedService(lo.Items[i])
			if err != nil {
				return nil, err
			}
			terminating.Services[i] = service
		}
	}

	return &terminating, nil
}

func parseSidecarService(o *ast.ObjectItem) (*api.ConsulSidecarService, error) {
	valid := []string{
		"port",
		"proxy",
		"tags",
		"disable_default_tcp_check",
	}

	if err := checkHCLKeys(o.Val, valid); err != nil {
		return nil, multierror.Prefix(err, "sidecar_service ->")
	}

	var sidecar api.ConsulSidecarService
	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, o.Val); err != nil {
		return nil, err
	}

	delete(m, "proxy")

	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook:       mapstructure.StringToTimeDurationHookFunc(),
		WeaklyTypedInput: true,
		Result:           &sidecar,
	})
	if err != nil {
		return nil, err
	}
	if err := dec.Decode(m); err != nil {
		return nil, fmt.Errorf("sidecar_service: %v", err)
	}

	var proxyList *ast.ObjectList
	if ot, ok := o.Val.(*ast.ObjectType); ok {
		proxyList = ot.List
	} else {
		return nil, fmt.Errorf("sidecar_service: should be an object")
	}

	// Parse the proxy
	po := proxyList.Filter("proxy")
	if len(po.Items) == 0 {
		return &sidecar, nil
	}
	if len(po.Items) > 1 {
		return nil, fmt.Errorf("only one 'proxy' block allowed per task")
	}

	r, err := parseProxy(po.Items[0])
	if err != nil {
		return nil, fmt.Errorf("proxy, %v", err)
	}
	sidecar.Proxy = r

	return &sidecar, nil
}

func parseSidecarTask(item *ast.ObjectItem) (*api.SidecarTask, error) {
	task, err := parseTask(item, sidecarTaskKeys)
	if err != nil {
		return nil, err
	}

	sidecarTask := &api.SidecarTask{
		Name:        task.Name,
		Driver:      task.Driver,
		User:        task.User,
		Config:      task.Config,
		Env:         task.Env,
		Resources:   task.Resources,
		Meta:        task.Meta,
		KillTimeout: task.KillTimeout,
		LogConfig:   task.LogConfig,
		KillSignal:  task.KillSignal,
	}

	// Parse ShutdownDelay separatly to get pointer
	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, item.Val); err != nil {
		return nil, err
	}

	m = map[string]interface{}{
		"shutdown_delay": m["shutdown_delay"],
	}

	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook:       mapstructure.StringToTimeDurationHookFunc(),
		WeaklyTypedInput: true,
		Result:           sidecarTask,
	})

	if err != nil {
		return nil, err
	}
	if err := dec.Decode(m); err != nil {
		return nil, err
	}
	return sidecarTask, nil
}

func parseProxy(o *ast.ObjectItem) (*api.ConsulProxy, error) {
	valid := []string{
		"local_service_address",
		"local_service_port",
		"upstreams",
		"expose",
		"config",
	}

	if err := checkHCLKeys(o.Val, valid); err != nil {
		return nil, multierror.Prefix(err, "proxy ->")
	}

	var proxy api.ConsulProxy
	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, o.Val); err != nil {
		return nil, err
	}

	delete(m, "upstreams")
	delete(m, "expose")
	delete(m, "config")

	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result: &proxy,
	})
	if err != nil {
		return nil, err
	}
	if err := dec.Decode(m); err != nil {
		return nil, fmt.Errorf("proxy: %v", err)
	}

	// Parse upstreams, expose, and config

	var listVal *ast.ObjectList
	if ot, ok := o.Val.(*ast.ObjectType); ok {
		listVal = ot.List
	} else {
		return nil, fmt.Errorf("proxy: should be an object")
	}

	uo := listVal.Filter("upstreams")
	if len(uo.Items) > 0 {
		proxy.Upstreams = make([]*api.ConsulUpstream, len(uo.Items))
		for i := range uo.Items {
			u, err := parseUpstream(uo.Items[i])
			if err != nil {
				return nil, err
			}
			proxy.Upstreams[i] = u
		}
	}

	if eo := listVal.Filter("expose"); len(eo.Items) > 1 {
		return nil, fmt.Errorf("only 1 expose object supported")
	} else if len(eo.Items) == 1 {
		if e, err := parseExpose(eo.Items[0]); err != nil {
			return nil, err
		} else {
			proxy.ExposeConfig = e
		}
	}

	// If we have config, then parse that
	if o := listVal.Filter("config"); len(o.Items) > 1 {
		return nil, fmt.Errorf("only 1 meta object supported")
	} else if len(o.Items) == 1 {
		var mSlice []map[string]interface{}
		if err := hcl.DecodeObject(&mSlice, o.Items[0].Val); err != nil {
			return nil, err
		}

		if len(mSlice) > 1 {
			return nil, fmt.Errorf("only 1 meta object supported")
		}

		m := mSlice[0]

		if err := mapstructure.WeakDecode(m, &proxy.Config); err != nil {
			return nil, err
		}

		proxy.Config = flattenMapSlice(proxy.Config)
	}

	return &proxy, nil
}

func parseExpose(eo *ast.ObjectItem) (*api.ConsulExposeConfig, error) {
	valid := []string{
		"path", // an array of path blocks
	}

	if err := checkHCLKeys(eo.Val, valid); err != nil {
		return nil, multierror.Prefix(err, "expose ->")
	}

	var expose api.ConsulExposeConfig

	var listVal *ast.ObjectList
	if eoType, ok := eo.Val.(*ast.ObjectType); ok {
		listVal = eoType.List
	} else {
		return nil, fmt.Errorf("expose: should be an object")
	}

	// Parse the expose block

	po := listVal.Filter("path") // array
	if len(po.Items) > 0 {
		expose.Path = make([]*api.ConsulExposePath, len(po.Items))
		for i := range po.Items {
			p, err := parseExposePath(po.Items[i])
			if err != nil {
				return nil, err
			}
			expose.Path[i] = p
		}
	}

	return &expose, nil
}

func parseExposePath(epo *ast.ObjectItem) (*api.ConsulExposePath, error) {
	valid := []string{
		"path",
		"protocol",
		"local_path_port",
		"listener_port",
	}

	if err := checkHCLKeys(epo.Val, valid); err != nil {
		return nil, multierror.Prefix(err, "path ->")
	}

	var path api.ConsulExposePath
	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, epo.Val); err != nil {
		return nil, err
	}

	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result: &path,
	})
	if err != nil {
		return nil, err
	}

	if err := dec.Decode(m); err != nil {
		return nil, err
	}

	return &path, nil
}

func parseUpstream(uo *ast.ObjectItem) (*api.ConsulUpstream, error) {
	valid := []string{
		"destination_name",
		"local_bind_port",
		"local_bind_address",
		"datacenter",
		"mesh_gateway",
	}

	if err := checkHCLKeys(uo.Val, valid); err != nil {
		return nil, multierror.Prefix(err, "upstream ->")
	}

	var upstream api.ConsulUpstream
	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, uo.Val); err != nil {
		return nil, err
	}

	delete(m, "mesh_gateway")

	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook:       mapstructure.StringToTimeDurationHookFunc(),
		WeaklyTypedInput: true,
		Result:           &upstream,
	})
	if err != nil {
		return nil, err
	}

	if err := dec.Decode(m); err != nil {
		return nil, err
	}

	var listVal *ast.ObjectList
	if ot, ok := uo.Val.(*ast.ObjectType); ok {
		listVal = ot.List
	} else {
		return nil, fmt.Errorf("'%s': should be an object", upstream.DestinationName)
	}

	if mgO := listVal.Filter("mesh_gateway"); len(mgO.Items) > 0 {
		if len(mgO.Items) > 1 {
			return nil, fmt.Errorf("upstream '%s': cannot have more than 1 mesh_gateway", upstream.DestinationName)
		}

		mgw, err := parseMeshGateway(mgO.Items[0])
		if err != nil {
			return nil, multierror.Prefix(err, fmt.Sprintf("'%s',", upstream.DestinationName))
		}

		upstream.MeshGateway = mgw

	}
	return &upstream, nil
}

func parseMeshGateway(gwo *ast.ObjectItem) (*api.ConsulMeshGateway, error) {
	valid := []string{
		"mode",
	}

	if err := checkHCLKeys(gwo.Val, valid); err != nil {
		return nil, multierror.Prefix(err, "mesh_gateway ->")
	}

	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, gwo.Val); err != nil {
		return nil, err
	}

	var mgw api.ConsulMeshGateway
	if err := mapstructure.WeakDecode(m, &mgw); err != nil {
		return nil, err
	}

	return &mgw, nil
}

func parseChecks(service *api.Service, checkObjs *ast.ObjectList) error {
	service.Checks = make([]api.ServiceCheck, len(checkObjs.Items))
	for idx, co := range checkObjs.Items {
		// Check for invalid keys
		valid := []string{
			"name",
			"type",
			"interval",
			"timeout",
			"path",
			"protocol",
			"port",
			"expose",
			"command",
			"args",
			"initial_status",
			"tls_skip_verify",
			"header",
			"method",
			"check_restart",
			"address_mode",
			"grpc_service",
			"grpc_use_tls",
			"task",
			"success_before_passing",
			"failures_before_critical",
			"on_update",
			"body",
		}
		if err := checkHCLKeys(co.Val, valid); err != nil {
			return multierror.Prefix(err, "check ->")
		}

		var check api.ServiceCheck
		var cm map[string]interface{}
		if err := hcl.DecodeObject(&cm, co.Val); err != nil {
			return err
		}

		// HCL allows repeating stanzas so merge 'header' into a single
		// map[string][]string.
		if headerI, ok := cm["header"]; ok {
			headerRaw, ok := headerI.([]map[string]interface{})
			if !ok {
				return fmt.Errorf("check -> header -> expected a []map[string][]string but found %T", headerI)
			}
			m := map[string][]string{}
			for _, rawm := range headerRaw {
				for k, vI := range rawm {
					vs, ok := vI.([]interface{})
					if !ok {
						return fmt.Errorf("check -> header -> %q expected a []string but found %T", k, vI)
					}
					for _, vI := range vs {
						v, ok := vI.(string)
						if !ok {
							return fmt.Errorf("check -> header -> %q expected a string but found %T", k, vI)
						}
						m[k] = append(m[k], v)
					}
				}
			}

			check.Header = m

			// Remove "header" as it has been parsed
			delete(cm, "header")
		}

		delete(cm, "check_restart")

		dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
			DecodeHook:       mapstructure.StringToTimeDurationHookFunc(),
			WeaklyTypedInput: true,
			Result:           &check,
		})
		if err != nil {
			return err
		}
		if err := dec.Decode(cm); err != nil {
			return err
		}

		// Filter check_restart
		var checkRestartList *ast.ObjectList
		if ot, ok := co.Val.(*ast.ObjectType); ok {
			checkRestartList = ot.List
		} else {
			return fmt.Errorf("check_restart '%s': should be an object", check.Name)
		}

		if cro := checkRestartList.Filter("check_restart"); len(cro.Items) > 0 {
			if len(cro.Items) > 1 {
				return fmt.Errorf("check_restart '%s': cannot have more than 1 check_restart", check.Name)
			}
			cr, err := parseCheckRestart(cro.Items[0])
			if err != nil {
				return multierror.Prefix(err, fmt.Sprintf("check: '%s',", check.Name))
			}
			check.CheckRestart = cr
		}

		service.Checks[idx] = check
	}

	return nil
}

func parseCheckRestart(cro *ast.ObjectItem) (*api.CheckRestart, error) {
	valid := []string{
		"limit",
		"grace",
		"ignore_warnings",
	}

	if err := checkHCLKeys(cro.Val, valid); err != nil {
		return nil, multierror.Prefix(err, "check_restart ->")
	}

	var checkRestart api.CheckRestart
	var crm map[string]interface{}
	if err := hcl.DecodeObject(&crm, cro.Val); err != nil {
		return nil, err
	}

	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook:       mapstructure.StringToTimeDurationHookFunc(),
		WeaklyTypedInput: true,
		Result:           &checkRestart,
	})
	if err != nil {
		return nil, err
	}
	if err := dec.Decode(crm); err != nil {
		return nil, err
	}

	return &checkRestart, nil
}
