package jobspec

import (
	"fmt"
	"time"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/hcl"
	"github.com/hashicorp/hcl/hcl/ast"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/helper"
	"github.com/mitchellh/mapstructure"
)

var (
	commonTaskKeys = []string{
		"driver",
		"user",
		"config",
		"env",
		"resources",
		"meta",
		"logs",
		"kill_timeout",
		"shutdown_delay",
		"kill_signal",
	}

	normalTaskKeys = append(commonTaskKeys,
		"artifact",
		"constraint",
		"affinity",
		"dispatch_payload",
		"lifecycle",
		"leader",
		"restart",
		"service",
		"template",
		"vault",
		"kind",
		"volume_mount",
		"csi_plugin",
	)

	sidecarTaskKeys = append(commonTaskKeys,
		"name",
	)
)

func parseTasks(result *[]*api.Task, list *ast.ObjectList) error {
	list = list.Children()
	if len(list.Items) == 0 {
		return nil
	}

	// Go through each object and turn it into an actual result.
	seen := make(map[string]struct{})
	for _, item := range list.Items {
		n := item.Keys[0].Token.Value().(string)

		// Make sure we haven't already found this
		if _, ok := seen[n]; ok {
			return fmt.Errorf("task '%s' defined more than once", n)
		}
		seen[n] = struct{}{}

		t, err := parseTask(item, normalTaskKeys)
		if err != nil {
			return multierror.Prefix(err, fmt.Sprintf("'%s',", n))
		}

		t.Name = n

		*result = append(*result, t)
	}

	return nil
}

func parseTask(item *ast.ObjectItem, keys []string) (*api.Task, error) {
	// We need this later
	var listVal *ast.ObjectList
	if ot, ok := item.Val.(*ast.ObjectType); ok {
		listVal = ot.List
	} else {
		return nil, fmt.Errorf("should be an object")
	}

	// Check for invalid keys
	if err := helper.CheckHCLKeys(listVal, keys); err != nil {
		return nil, err
	}

	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, item.Val); err != nil {
		return nil, err
	}
	delete(m, "artifact")
	delete(m, "config")
	delete(m, "constraint")
	delete(m, "affinity")
	delete(m, "dispatch_payload")
	delete(m, "lifecycle")
	delete(m, "env")
	delete(m, "logs")
	delete(m, "meta")
	delete(m, "resources")
	delete(m, "restart")
	delete(m, "service")
	delete(m, "template")
	delete(m, "vault")
	delete(m, "volume_mount")
	delete(m, "csi_plugin")

	// Build the task
	var t api.Task
	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook:       mapstructure.StringToTimeDurationHookFunc(),
		WeaklyTypedInput: true,
		Result:           &t,
	})

	if err != nil {
		return nil, err
	}
	if err := dec.Decode(m); err != nil {
		return nil, err
	}

	// If we have env, then parse them
	if o := listVal.Filter("env"); len(o.Items) > 0 {
		for _, o := range o.Elem().Items {
			var m map[string]interface{}
			if err := hcl.DecodeObject(&m, o.Val); err != nil {
				return nil, err
			}
			if err := mapstructure.WeakDecode(m, &t.Env); err != nil {
				return nil, err
			}
		}
	}

	if o := listVal.Filter("service"); len(o.Items) > 0 {
		services, err := parseServices(o)
		if err != nil {
			return nil, err
		}

		t.Services = services
	}

	if o := listVal.Filter("csi_plugin"); len(o.Items) > 0 {
		if len(o.Items) != 1 {
			return nil, fmt.Errorf("csi_plugin -> Expected single stanza, got %d", len(o.Items))
		}
		i := o.Elem().Items[0]

		var m map[string]interface{}
		if err := hcl.DecodeObject(&m, i.Val); err != nil {
			return nil, err
		}

		var cfg api.TaskCSIPluginConfig
		if err := mapstructure.WeakDecode(m, &cfg); err != nil {
			return nil, err
		}

		t.CSIPluginConfig = &cfg
	}

	// If we have config, then parse that
	if o := listVal.Filter("config"); len(o.Items) > 0 {
		for _, o := range o.Elem().Items {
			var m map[string]interface{}
			if err := hcl.DecodeObject(&m, o.Val); err != nil {
				return nil, err
			}

			if err := mapstructure.WeakDecode(m, &t.Config); err != nil {
				return nil, err
			}
		}
	}

	// Parse constraints
	if o := listVal.Filter("constraint"); len(o.Items) > 0 {
		if err := parseConstraints(&t.Constraints, o); err != nil {
			return nil, multierror.Prefix(err, "constraint ->")
		}
	}

	// Parse affinities
	if o := listVal.Filter("affinity"); len(o.Items) > 0 {
		if err := parseAffinities(&t.Affinities, o); err != nil {
			return nil, multierror.Prefix(err, "affinity ->")
		}
	}

	// Parse out meta fields. These are in HCL as a list so we need
	// to iterate over them and merge them.
	if metaO := listVal.Filter("meta"); len(metaO.Items) > 0 {
		for _, o := range metaO.Elem().Items {
			var m map[string]interface{}
			if err := hcl.DecodeObject(&m, o.Val); err != nil {
				return nil, err
			}
			if err := mapstructure.WeakDecode(m, &t.Meta); err != nil {
				return nil, err
			}
		}
	}

	// Parse volume mounts
	if o := listVal.Filter("volume_mount"); len(o.Items) > 0 {
		if err := parseVolumeMounts(&t.VolumeMounts, o); err != nil {
			return nil, multierror.Prefix(err, "volume_mount ->")
		}
	}

	// If we have resources, then parse that
	if o := listVal.Filter("resources"); len(o.Items) > 0 {
		var r api.Resources
		if err := parseResources(&r, o); err != nil {
			return nil, multierror.Prefix(err, "resources ->")
		}

		t.Resources = &r
	}

	// Parse restart policy
	if o := listVal.Filter("restart"); len(o.Items) > 0 {
		if err := parseRestartPolicy(&t.RestartPolicy, o); err != nil {
			return nil, multierror.Prefix(err, "restart ->")
		}
	}

	// If we have logs then parse that
	if o := listVal.Filter("logs"); len(o.Items) > 0 {
		if len(o.Items) > 1 {
			return nil, fmt.Errorf("only one logs block is allowed in a Task. Number of logs block found: %d", len(o.Items))
		}
		var m map[string]interface{}
		logsBlock := o.Items[0]

		// Check for invalid keys
		valid := []string{
			"max_files",
			"max_file_size",
		}
		if err := helper.CheckHCLKeys(logsBlock.Val, valid); err != nil {
			return nil, multierror.Prefix(err, "logs ->")
		}

		if err := hcl.DecodeObject(&m, logsBlock.Val); err != nil {
			return nil, err
		}

		var log api.LogConfig
		if err := mapstructure.WeakDecode(m, &log); err != nil {
			return nil, err
		}

		t.LogConfig = &log
	}

	// Parse artifacts
	if o := listVal.Filter("artifact"); len(o.Items) > 0 {
		if err := parseArtifacts(&t.Artifacts, o); err != nil {
			return nil, multierror.Prefix(err, "artifact ->")
		}
	}

	// Parse templates
	if o := listVal.Filter("template"); len(o.Items) > 0 {
		if err := parseTemplates(&t.Templates, o); err != nil {
			return nil, multierror.Prefix(err, "template ->")
		}
	}

	// If we have a vault block, then parse that
	if o := listVal.Filter("vault"); len(o.Items) > 0 {
		v := &api.Vault{
			Env:        helper.BoolToPtr(true),
			ChangeMode: helper.StringToPtr("restart"),
		}

		if err := parseVault(v, o); err != nil {
			return nil, multierror.Prefix(err, "vault ->")
		}

		t.Vault = v
	}

	// If we have a dispatch_payload block parse that
	if o := listVal.Filter("dispatch_payload"); len(o.Items) > 0 {
		if len(o.Items) > 1 {
			return nil, fmt.Errorf("only one dispatch_payload block is allowed in a task. Number of dispatch_payload blocks found: %d", len(o.Items))
		}
		var m map[string]interface{}
		dispatchBlock := o.Items[0]

		// Check for invalid keys
		valid := []string{
			"file",
		}
		if err := helper.CheckHCLKeys(dispatchBlock.Val, valid); err != nil {
			return nil, multierror.Prefix(err, "dispatch_payload ->")
		}

		if err := hcl.DecodeObject(&m, dispatchBlock.Val); err != nil {
			return nil, err
		}

		t.DispatchPayload = &api.DispatchPayloadConfig{}
		if err := mapstructure.WeakDecode(m, t.DispatchPayload); err != nil {
			return nil, err
		}
	}

	// If we have a lifecycle block parse that
	if o := listVal.Filter("lifecycle"); len(o.Items) > 0 {
		if len(o.Items) > 1 {
			return nil, fmt.Errorf("only one lifecycle block is allowed in a task. Number of lifecycle blocks found: %d", len(o.Items))
		}

		var m map[string]interface{}
		lifecycleBlock := o.Items[0]

		// Check for invalid keys
		valid := []string{
			"hook",
			"sidecar",
		}
		if err := helper.CheckHCLKeys(lifecycleBlock.Val, valid); err != nil {
			return nil, multierror.Prefix(err, "lifecycle ->")
		}

		if err := hcl.DecodeObject(&m, lifecycleBlock.Val); err != nil {
			return nil, err
		}

		t.Lifecycle = &api.TaskLifecycle{}
		if err := mapstructure.WeakDecode(m, t.Lifecycle); err != nil {
			return nil, err
		}
	}
	return &t, nil
}

func parseArtifacts(result *[]*api.TaskArtifact, list *ast.ObjectList) error {
	for _, o := range list.Elem().Items {
		// Check for invalid keys
		valid := []string{
			"source",
			"options",
			"mode",
			"destination",
		}
		if err := helper.CheckHCLKeys(o.Val, valid); err != nil {
			return err
		}

		var m map[string]interface{}
		if err := hcl.DecodeObject(&m, o.Val); err != nil {
			return err
		}

		delete(m, "options")

		var ta api.TaskArtifact
		if err := mapstructure.WeakDecode(m, &ta); err != nil {
			return err
		}

		var optionList *ast.ObjectList
		if ot, ok := o.Val.(*ast.ObjectType); ok {
			optionList = ot.List
		} else {
			return fmt.Errorf("artifact should be an object")
		}

		if oo := optionList.Filter("options"); len(oo.Items) > 0 {
			options := make(map[string]string)
			if err := parseArtifactOption(options, oo); err != nil {
				return multierror.Prefix(err, "options: ")
			}
			ta.GetterOptions = options
		}

		*result = append(*result, &ta)
	}

	return nil
}

func parseArtifactOption(result map[string]string, list *ast.ObjectList) error {
	list = list.Elem()
	if len(list.Items) > 1 {
		return fmt.Errorf("only one 'options' block allowed per artifact")
	}

	// Get our resource object
	o := list.Items[0]

	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, o.Val); err != nil {
		return err
	}

	if err := mapstructure.WeakDecode(m, &result); err != nil {
		return err
	}

	return nil
}

func parseTemplates(result *[]*api.Template, list *ast.ObjectList) error {
	for _, o := range list.Elem().Items {
		// Check for invalid keys
		valid := []string{
			"change_mode",
			"change_signal",
			"data",
			"destination",
			"left_delimiter",
			"perms",
			"right_delimiter",
			"source",
			"splay",
			"env",
			"vault_grace", //COMPAT(0.12) not used; emits warning in 0.11.
		}
		if err := helper.CheckHCLKeys(o.Val, valid); err != nil {
			return err
		}

		var m map[string]interface{}
		if err := hcl.DecodeObject(&m, o.Val); err != nil {
			return err
		}

		templ := &api.Template{
			ChangeMode: helper.StringToPtr("restart"),
			Splay:      helper.TimeToPtr(5 * time.Second),
			Perms:      helper.StringToPtr("0644"),
		}

		dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
			DecodeHook:       mapstructure.StringToTimeDurationHookFunc(),
			WeaklyTypedInput: true,
			Result:           templ,
		})
		if err != nil {
			return err
		}
		if err := dec.Decode(m); err != nil {
			return err
		}

		*result = append(*result, templ)
	}

	return nil
}

func parseResources(result *api.Resources, list *ast.ObjectList) error {
	list = list.Elem()
	if len(list.Items) == 0 {
		return nil
	}
	if len(list.Items) > 1 {
		return fmt.Errorf("only one 'resource' block allowed per task")
	}

	// Get our resource object
	o := list.Items[0]

	// We need this later
	var listVal *ast.ObjectList
	if ot, ok := o.Val.(*ast.ObjectType); ok {
		listVal = ot.List
	} else {
		return fmt.Errorf("resource: should be an object")
	}

	// Check for invalid keys
	valid := []string{
		"cpu",
		"iops", // COMPAT(0.10): Remove after one release to allow it to be removed from jobspecs
		"disk",
		"memory",
		"network",
		"device",
	}
	if err := helper.CheckHCLKeys(listVal, valid); err != nil {
		return multierror.Prefix(err, "resources ->")
	}

	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, o.Val); err != nil {
		return err
	}
	delete(m, "network")
	delete(m, "device")

	if err := mapstructure.WeakDecode(m, result); err != nil {
		return err
	}

	// Parse the network resources
	if o := listVal.Filter("network"); len(o.Items) > 0 {
		r, err := ParseNetwork(o)
		if err != nil {
			return fmt.Errorf("resource, %v", err)
		}
		result.Networks = []*api.NetworkResource{r}
	}

	// Parse the device resources
	if o := listVal.Filter("device"); len(o.Items) > 0 {
		result.Devices = make([]*api.RequestedDevice, len(o.Items))
		for idx, do := range o.Items {
			if l := len(do.Keys); l == 0 {
				return multierror.Prefix(fmt.Errorf("missing device name"), fmt.Sprintf("resources, device[%d]->", idx))
			} else if l > 1 {
				return multierror.Prefix(fmt.Errorf("only one name may be specified"), fmt.Sprintf("resources, device[%d]->", idx))
			}
			name := do.Keys[0].Token.Value().(string)

			// Value should be an object
			var listVal *ast.ObjectList
			if ot, ok := do.Val.(*ast.ObjectType); ok {
				listVal = ot.List
			} else {
				return fmt.Errorf("device should be an object")
			}

			// Check for invalid keys
			valid := []string{
				"name",
				"count",
				"affinity",
				"constraint",
			}
			if err := helper.CheckHCLKeys(do.Val, valid); err != nil {
				return multierror.Prefix(err, fmt.Sprintf("resources, device[%d]->", idx))
			}

			// Set the name
			var r api.RequestedDevice
			r.Name = name

			var m map[string]interface{}
			if err := hcl.DecodeObject(&m, do.Val); err != nil {
				return err
			}

			delete(m, "constraint")
			delete(m, "affinity")

			if err := mapstructure.WeakDecode(m, &r); err != nil {
				return err
			}

			// Parse constraints
			if o := listVal.Filter("constraint"); len(o.Items) > 0 {
				if err := parseConstraints(&r.Constraints, o); err != nil {
					return multierror.Prefix(err, "constraint ->")
				}
			}

			// Parse affinities
			if o := listVal.Filter("affinity"); len(o.Items) > 0 {
				if err := parseAffinities(&r.Affinities, o); err != nil {
					return multierror.Prefix(err, "affinity ->")
				}
			}

			result.Devices[idx] = &r
		}
	}

	return nil
}

func parseVolumeMounts(out *[]*api.VolumeMount, list *ast.ObjectList) error {
	mounts := make([]*api.VolumeMount, len(list.Items))

	for i, item := range list.Items {
		valid := []string{
			"volume",
			"read_only",
			"destination",
			"propagation_mode",
		}
		if err := helper.CheckHCLKeys(item.Val, valid); err != nil {
			return err
		}

		var m map[string]interface{}
		if err := hcl.DecodeObject(&m, item.Val); err != nil {
			return err
		}

		var result api.VolumeMount
		dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
			WeaklyTypedInput: true,
			Result:           &result,
		})
		if err != nil {
			return err
		}
		if err := dec.Decode(m); err != nil {
			return err
		}

		mounts[i] = &result
	}

	*out = mounts
	return nil
}
