package jobspec

import (
	"fmt"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/hcl"
	"github.com/hashicorp/hcl/hcl/ast"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/helper"
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
		"meta",
		"canary_meta",
	}
	if err := helper.CheckHCLKeys(o.Val, valid); err != nil {
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
		"sidecar_service",
		"sidecar_task",
	}

	if err := helper.CheckHCLKeys(co.Val, valid); err != nil {
		return nil, multierror.Prefix(err, "connect ->")
	}

	var connect api.ConsulConnect
	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, co.Val); err != nil {
		return nil, err
	}

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

	// Parse the sidecar_service
	o := connectList.Filter("sidecar_service")
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

func parseSidecarService(o *ast.ObjectItem) (*api.ConsulSidecarService, error) {
	valid := []string{
		"port",
		"proxy",
		"tags",
	}

	if err := helper.CheckHCLKeys(o.Val, valid); err != nil {
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
	// We need this later
	var listVal *ast.ObjectList
	if ot, ok := item.Val.(*ast.ObjectType); ok {
		listVal = ot.List
	} else {
		return nil, fmt.Errorf("should be an object")
	}

	// Check for invalid keys
	valid := []string{
		"config",
		"driver",
		"env",
		"kill_timeout",
		"logs",
		"meta",
		"resources",
		"shutdown_delay",
		"user",
		"kill_signal",
	}
	if err := helper.CheckHCLKeys(listVal, valid); err != nil {
		return nil, err
	}

	task, err := parseTask(item)
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

	// Parse ShutdownDelay separately to get pointer
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

	if err := helper.CheckHCLKeys(o.Val, valid); err != nil {
		return nil, multierror.Prefix(err, "proxy ->")
	}

	var proxy api.ConsulProxy

	var listVal *ast.ObjectList
	if ot, ok := o.Val.(*ast.ObjectType); ok {
		listVal = ot.List
	} else {
		return nil, fmt.Errorf("proxy: should be an object")
	}

	// Parse the proxy

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

	if err := helper.CheckHCLKeys(eo.Val, valid); err != nil {
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

	if err := helper.CheckHCLKeys(epo.Val, valid); err != nil {
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
	}

	if err := helper.CheckHCLKeys(uo.Val, valid); err != nil {
		return nil, multierror.Prefix(err, "upstream ->")
	}

	var upstream api.ConsulUpstream
	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, uo.Val); err != nil {
		return nil, err
	}

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

	return &upstream, nil
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
		}
		if err := helper.CheckHCLKeys(co.Val, valid); err != nil {
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

	if err := helper.CheckHCLKeys(cro.Val, valid); err != nil {
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
