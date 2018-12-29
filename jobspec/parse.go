package jobspec

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/hcl"
	"github.com/hashicorp/hcl/hcl/ast"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/mapstructure"
)

var reDynamicPorts = regexp.MustCompile("^[a-zA-Z0-9_]+$")
var errPortLabel = fmt.Errorf("Port label does not conform to naming requirements %s", reDynamicPorts.String())

// Parse parses the job spec from the given io.Reader.
//
// Due to current internal limitations, the entire contents of the
// io.Reader will be copied into memory first before parsing.
func Parse(r io.Reader) (*api.Job, error) {
	// Copy the reader into an in-memory buffer first since HCL requires it.
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		return nil, err
	}

	// Parse the buffer
	root, err := hcl.Parse(buf.String())
	if err != nil {
		return nil, fmt.Errorf("error parsing: %s", err)
	}
	buf.Reset()

	// Top-level item should be a list
	list, ok := root.Node.(*ast.ObjectList)
	if !ok {
		return nil, fmt.Errorf("error parsing: root should be an object")
	}

	// Check for invalid keys
	valid := []string{
		"job",
	}
	if err := helper.CheckHCLKeys(list, valid); err != nil {
		return nil, err
	}

	var job api.Job

	// Parse the job out
	matches := list.Filter("job")
	if len(matches.Items) == 0 {
		return nil, fmt.Errorf("'job' stanza not found")
	}
	if err := parseJob(&job, matches); err != nil {
		return nil, fmt.Errorf("error parsing 'job': %s", err)
	}

	return &job, nil
}

// ParseFile parses the given path as a job spec.
func ParseFile(path string) (*api.Job, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return Parse(f)
}

func parseJob(result *api.Job, list *ast.ObjectList) error {
	if len(list.Items) != 1 {
		return fmt.Errorf("only one 'job' block allowed")
	}
	list = list.Children()
	if len(list.Items) != 1 {
		return fmt.Errorf("'job' block missing name")
	}

	// Get our job object
	obj := list.Items[0]

	// Decode the full thing into a map[string]interface for ease
	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, obj.Val); err != nil {
		return err
	}
	delete(m, "constraint")
	delete(m, "meta")
	delete(m, "update")
	delete(m, "periodic")
	delete(m, "vault")
	delete(m, "parameterized")

	// Set the ID and name to the object key
	result.ID = helper.StringToPtr(obj.Keys[0].Token.Value().(string))
	result.Name = helper.StringToPtr(*result.ID)

	// Decode the rest
	if err := mapstructure.WeakDecode(m, result); err != nil {
		return err
	}

	// Value should be an object
	var listVal *ast.ObjectList
	if ot, ok := obj.Val.(*ast.ObjectType); ok {
		listVal = ot.List
	} else {
		return fmt.Errorf("job '%s' value: should be an object", *result.ID)
	}

	// Check for invalid keys
	valid := []string{
		"all_at_once",
		"constraint",
		"datacenters",
		"parameterized",
		"group",
		"id",
		"meta",
		"name",
		"namespace",
		"periodic",
		"priority",
		"region",
		"task",
		"type",
		"update",
		"vault",
		"vault_token",
	}
	if err := helper.CheckHCLKeys(listVal, valid); err != nil {
		return multierror.Prefix(err, "job:")
	}

	// Parse constraints
	if o := listVal.Filter("constraint"); len(o.Items) > 0 {
		if err := parseConstraints(&result.Constraints, o); err != nil {
			return multierror.Prefix(err, "constraint ->")
		}
	}

	// If we have an update strategy, then parse that
	if o := listVal.Filter("update"); len(o.Items) > 0 {
		if err := parseUpdate(&result.Update, o); err != nil {
			return multierror.Prefix(err, "update ->")
		}
	}

	// If we have a periodic definition, then parse that
	if o := listVal.Filter("periodic"); len(o.Items) > 0 {
		if err := parsePeriodic(&result.Periodic, o); err != nil {
			return multierror.Prefix(err, "periodic ->")
		}
	}

	// If we have a parameterized definition, then parse that
	if o := listVal.Filter("parameterized"); len(o.Items) > 0 {
		if err := parseParameterizedJob(&result.ParameterizedJob, o); err != nil {
			return multierror.Prefix(err, "parameterized ->")
		}
	}

	// Parse out meta fields. These are in HCL as a list so we need
	// to iterate over them and merge them.
	if metaO := listVal.Filter("meta"); len(metaO.Items) > 0 {
		for _, o := range metaO.Elem().Items {
			var m map[string]interface{}
			if err := hcl.DecodeObject(&m, o.Val); err != nil {
				return err
			}
			if err := mapstructure.WeakDecode(m, &result.Meta); err != nil {
				return err
			}
		}
	}

	// If we have tasks outside, create TaskGroups for them
	if o := listVal.Filter("task"); len(o.Items) > 0 {
		var tasks []*api.Task
		if err := parseTasks(*result.Name, "", &tasks, o); err != nil {
			return multierror.Prefix(err, "task:")
		}

		result.TaskGroups = make([]*api.TaskGroup, len(tasks), len(tasks)*2)
		for i, t := range tasks {
			result.TaskGroups[i] = &api.TaskGroup{
				Name:  helper.StringToPtr(t.Name),
				Tasks: []*api.Task{t},
			}
		}
	}

	// Parse the task groups
	if o := listVal.Filter("group"); len(o.Items) > 0 {
		if err := parseGroups(result, o); err != nil {
			return multierror.Prefix(err, "group:")
		}
	}

	// If we have a vault block, then parse that
	if o := listVal.Filter("vault"); len(o.Items) > 0 {
		jobVault := &api.Vault{
			Env:        helper.BoolToPtr(true),
			ChangeMode: helper.StringToPtr("restart"),
		}

		if err := parseVault(jobVault, o); err != nil {
			return multierror.Prefix(err, "vault ->")
		}

		// Go through the task groups/tasks and if they don't have a Vault block, set it
		for _, tg := range result.TaskGroups {
			for _, task := range tg.Tasks {
				if task.Vault == nil {
					task.Vault = jobVault
				}
			}
		}
	}

	return nil
}

func parseGroups(result *api.Job, list *ast.ObjectList) error {
	list = list.Children()
	if len(list.Items) == 0 {
		return nil
	}

	// Go through each object and turn it into an actual result.
	collection := make([]*api.TaskGroup, 0, len(list.Items))
	seen := make(map[string]struct{})
	for _, item := range list.Items {
		n := item.Keys[0].Token.Value().(string)

		// Make sure we haven't already found this
		if _, ok := seen[n]; ok {
			return fmt.Errorf("group '%s' defined more than once", n)
		}
		seen[n] = struct{}{}

		// We need this later
		var listVal *ast.ObjectList
		if ot, ok := item.Val.(*ast.ObjectType); ok {
			listVal = ot.List
		} else {
			return fmt.Errorf("group '%s': should be an object", n)
		}

		// Check for invalid keys
		valid := []string{
			"count",
			"constraint",
			"restart",
			"meta",
			"task",
			"ephemeral_disk",
			"update",
			"vault",
		}
		if err := helper.CheckHCLKeys(listVal, valid); err != nil {
			return multierror.Prefix(err, fmt.Sprintf("'%s' ->", n))
		}

		var m map[string]interface{}
		if err := hcl.DecodeObject(&m, item.Val); err != nil {
			return err
		}
		delete(m, "constraint")
		delete(m, "meta")
		delete(m, "task")
		delete(m, "restart")
		delete(m, "ephemeral_disk")
		delete(m, "update")
		delete(m, "vault")

		// Build the group with the basic decode
		var g api.TaskGroup
		g.Name = helper.StringToPtr(n)
		if err := mapstructure.WeakDecode(m, &g); err != nil {
			return err
		}

		// Parse constraints
		if o := listVal.Filter("constraint"); len(o.Items) > 0 {
			if err := parseConstraints(&g.Constraints, o); err != nil {
				return multierror.Prefix(err, fmt.Sprintf("'%s', constraint ->", n))
			}
		}

		// Parse restart policy
		if o := listVal.Filter("restart"); len(o.Items) > 0 {
			if err := parseRestartPolicy(&g.RestartPolicy, o); err != nil {
				return multierror.Prefix(err, fmt.Sprintf("'%s', restart ->", n))
			}
		}

		// Parse ephemeral disk
		if o := listVal.Filter("ephemeral_disk"); len(o.Items) > 0 {
			g.EphemeralDisk = &api.EphemeralDisk{}
			if err := parseEphemeralDisk(&g.EphemeralDisk, o); err != nil {
				return multierror.Prefix(err, fmt.Sprintf("'%s', ephemeral_disk ->", n))
			}
		}

		// If we have an update strategy, then parse that
		if o := listVal.Filter("update"); len(o.Items) > 0 {
			if err := parseUpdate(&g.Update, o); err != nil {
				return multierror.Prefix(err, "update ->")
			}
		}

		// Parse out meta fields. These are in HCL as a list so we need
		// to iterate over them and merge them.
		if metaO := listVal.Filter("meta"); len(metaO.Items) > 0 {
			for _, o := range metaO.Elem().Items {
				var m map[string]interface{}
				if err := hcl.DecodeObject(&m, o.Val); err != nil {
					return err
				}
				if err := mapstructure.WeakDecode(m, &g.Meta); err != nil {
					return err
				}
			}
		}

		// Parse tasks
		if o := listVal.Filter("task"); len(o.Items) > 0 {
			if err := parseTasks(*result.Name, *g.Name, &g.Tasks, o); err != nil {
				return multierror.Prefix(err, fmt.Sprintf("'%s', task:", n))
			}
		}

		// If we have a vault block, then parse that
		if o := listVal.Filter("vault"); len(o.Items) > 0 {
			tgVault := &api.Vault{
				Env:        helper.BoolToPtr(true),
				ChangeMode: helper.StringToPtr("restart"),
			}

			if err := parseVault(tgVault, o); err != nil {
				return multierror.Prefix(err, fmt.Sprintf("'%s', vault ->", n))
			}

			// Go through the tasks and if they don't have a Vault block, set it
			for _, task := range g.Tasks {
				if task.Vault == nil {
					task.Vault = tgVault
				}
			}
		}

		collection = append(collection, &g)
	}

	result.TaskGroups = append(result.TaskGroups, collection...)
	return nil
}

func parseRestartPolicy(final **api.RestartPolicy, list *ast.ObjectList) error {
	list = list.Elem()
	if len(list.Items) > 1 {
		return fmt.Errorf("only one 'restart' block allowed")
	}

	// Get our job object
	obj := list.Items[0]

	// Check for invalid keys
	valid := []string{
		"attempts",
		"interval",
		"delay",
		"mode",
	}
	if err := helper.CheckHCLKeys(obj.Val, valid); err != nil {
		return err
	}

	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, obj.Val); err != nil {
		return err
	}

	var result api.RestartPolicy
	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook:       mapstructure.StringToTimeDurationHookFunc(),
		WeaklyTypedInput: true,
		Result:           &result,
	})
	if err != nil {
		return err
	}
	if err := dec.Decode(m); err != nil {
		return err
	}

	*final = &result
	return nil
}

func parseConstraints(result *[]*api.Constraint, list *ast.ObjectList) error {
	for _, o := range list.Elem().Items {
		// Check for invalid keys
		valid := []string{
			"attribute",
			"distinct_hosts",
			"distinct_property",
			"operator",
			"regexp",
			"set_contains",
			"value",
			"version",
		}
		if err := helper.CheckHCLKeys(o.Val, valid); err != nil {
			return err
		}

		var m map[string]interface{}
		if err := hcl.DecodeObject(&m, o.Val); err != nil {
			return err
		}

		m["LTarget"] = m["attribute"]
		m["RTarget"] = m["value"]
		m["Operand"] = m["operator"]

		// If "version" is provided, set the operand
		// to "version" and the value to the "RTarget"
		if constraint, ok := m[structs.ConstraintVersion]; ok {
			m["Operand"] = structs.ConstraintVersion
			m["RTarget"] = constraint
		}

		// If "regexp" is provided, set the operand
		// to "regexp" and the value to the "RTarget"
		if constraint, ok := m[structs.ConstraintRegex]; ok {
			m["Operand"] = structs.ConstraintRegex
			m["RTarget"] = constraint
		}

		// If "set_contains" is provided, set the operand
		// to "set_contains" and the value to the "RTarget"
		if constraint, ok := m[structs.ConstraintSetContains]; ok {
			m["Operand"] = structs.ConstraintSetContains
			m["RTarget"] = constraint
		}

		if value, ok := m[structs.ConstraintDistinctHosts]; ok {
			enabled, err := parseBool(value)
			if err != nil {
				return fmt.Errorf("distinct_hosts should be set to true or false; %v", err)
			}

			// If it is not enabled, skip the constraint.
			if !enabled {
				continue
			}

			m["Operand"] = structs.ConstraintDistinctHosts
		}

		if property, ok := m[structs.ConstraintDistinctProperty]; ok {
			m["Operand"] = structs.ConstraintDistinctProperty
			m["LTarget"] = property
		}

		// Build the constraint
		var c api.Constraint
		if err := mapstructure.WeakDecode(m, &c); err != nil {
			return err
		}
		if c.Operand == "" {
			c.Operand = "="
		}

		*result = append(*result, &c)
	}

	return nil
}

func parseEphemeralDisk(result **api.EphemeralDisk, list *ast.ObjectList) error {
	list = list.Elem()
	if len(list.Items) > 1 {
		return fmt.Errorf("only one 'ephemeral_disk' block allowed")
	}

	// Get our ephemeral_disk object
	obj := list.Items[0]

	// Check for invalid keys
	valid := []string{
		"sticky",
		"size",
		"migrate",
	}
	if err := helper.CheckHCLKeys(obj.Val, valid); err != nil {
		return err
	}

	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, obj.Val); err != nil {
		return err
	}

	var ephemeralDisk api.EphemeralDisk
	if err := mapstructure.WeakDecode(m, &ephemeralDisk); err != nil {
		return err
	}
	*result = &ephemeralDisk

	return nil
}

// parseBool takes an interface value and tries to convert it to a boolean and
// returns an error if the type can't be converted.
func parseBool(value interface{}) (bool, error) {
	var enabled bool
	var err error
	switch value.(type) {
	case string:
		enabled, err = strconv.ParseBool(value.(string))
	case bool:
		enabled = value.(bool)
	default:
		err = fmt.Errorf("%v couldn't be converted to boolean value", value)
	}

	return enabled, err
}

func parseTasks(jobName string, taskGroupName string, result *[]*api.Task, list *ast.ObjectList) error {
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

		// We need this later
		var listVal *ast.ObjectList
		if ot, ok := item.Val.(*ast.ObjectType); ok {
			listVal = ot.List
		} else {
			return fmt.Errorf("group '%s': should be an object", n)
		}

		// Check for invalid keys
		valid := []string{
			"artifact",
			"config",
			"constraint",
			"dispatch_payload",
			"driver",
			"env",
			"kill_timeout",
			"leader",
			"logs",
			"meta",
			"resources",
			"service",
			"shutdown_delay",
			"template",
			"user",
			"vault",
		}
		if err := helper.CheckHCLKeys(listVal, valid); err != nil {
			return multierror.Prefix(err, fmt.Sprintf("'%s' ->", n))
		}

		var m map[string]interface{}
		if err := hcl.DecodeObject(&m, item.Val); err != nil {
			return err
		}
		delete(m, "artifact")
		delete(m, "config")
		delete(m, "constraint")
		delete(m, "dispatch_payload")
		delete(m, "env")
		delete(m, "logs")
		delete(m, "meta")
		delete(m, "resources")
		delete(m, "service")
		delete(m, "template")
		delete(m, "vault")

		// Build the task
		var t api.Task
		t.Name = n
		if taskGroupName == "" {
			taskGroupName = n
		}
		dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
			DecodeHook:       mapstructure.StringToTimeDurationHookFunc(),
			WeaklyTypedInput: true,
			Result:           &t,
		})
		if err != nil {
			return err
		}
		if err := dec.Decode(m); err != nil {
			return err
		}

		// If we have env, then parse them
		if o := listVal.Filter("env"); len(o.Items) > 0 {
			for _, o := range o.Elem().Items {
				var m map[string]interface{}
				if err := hcl.DecodeObject(&m, o.Val); err != nil {
					return err
				}
				if err := mapstructure.WeakDecode(m, &t.Env); err != nil {
					return err
				}
			}
		}

		if o := listVal.Filter("service"); len(o.Items) > 0 {
			if err := parseServices(jobName, taskGroupName, &t, o); err != nil {
				return multierror.Prefix(err, fmt.Sprintf("'%s',", n))
			}
		}

		// If we have config, then parse that
		if o := listVal.Filter("config"); len(o.Items) > 0 {
			for _, o := range o.Elem().Items {
				var m map[string]interface{}
				if err := hcl.DecodeObject(&m, o.Val); err != nil {
					return err
				}

				if err := mapstructure.WeakDecode(m, &t.Config); err != nil {
					return err
				}
			}
		}

		// Parse constraints
		if o := listVal.Filter("constraint"); len(o.Items) > 0 {
			if err := parseConstraints(&t.Constraints, o); err != nil {
				return multierror.Prefix(err, fmt.Sprintf(
					"'%s', constraint ->", n))
			}
		}

		// Parse out meta fields. These are in HCL as a list so we need
		// to iterate over them and merge them.
		if metaO := listVal.Filter("meta"); len(metaO.Items) > 0 {
			for _, o := range metaO.Elem().Items {
				var m map[string]interface{}
				if err := hcl.DecodeObject(&m, o.Val); err != nil {
					return err
				}
				if err := mapstructure.WeakDecode(m, &t.Meta); err != nil {
					return err
				}
			}
		}

		// If we have resources, then parse that
		if o := listVal.Filter("resources"); len(o.Items) > 0 {
			var r api.Resources
			if err := parseResources(&r, o); err != nil {
				return multierror.Prefix(err, fmt.Sprintf("'%s',", n))
			}

			t.Resources = &r
		}

		// If we have logs then parse that
		if o := listVal.Filter("logs"); len(o.Items) > 0 {
			if len(o.Items) > 1 {
				return fmt.Errorf("only one logs block is allowed in a Task. Number of logs block found: %d", len(o.Items))
			}
			var m map[string]interface{}
			logsBlock := o.Items[0]

			// Check for invalid keys
			valid := []string{
				"max_files",
				"max_file_size",
			}
			if err := helper.CheckHCLKeys(logsBlock.Val, valid); err != nil {
				return multierror.Prefix(err, fmt.Sprintf("'%s', logs ->", n))
			}

			if err := hcl.DecodeObject(&m, logsBlock.Val); err != nil {
				return err
			}

			var log api.LogConfig
			if err := mapstructure.WeakDecode(m, &log); err != nil {
				return err
			}

			t.LogConfig = &log
		}

		// Parse artifacts
		if o := listVal.Filter("artifact"); len(o.Items) > 0 {
			if err := parseArtifacts(&t.Artifacts, o); err != nil {
				return multierror.Prefix(err, fmt.Sprintf("'%s', artifact ->", n))
			}
		}

		// Parse templates
		if o := listVal.Filter("template"); len(o.Items) > 0 {
			if err := parseTemplates(&t.Templates, o); err != nil {
				return multierror.Prefix(err, fmt.Sprintf("'%s', template ->", n))
			}
		}

		// If we have a vault block, then parse that
		if o := listVal.Filter("vault"); len(o.Items) > 0 {
			v := &api.Vault{
				Env:        helper.BoolToPtr(true),
				ChangeMode: helper.StringToPtr("restart"),
			}

			if err := parseVault(v, o); err != nil {
				return multierror.Prefix(err, fmt.Sprintf("'%s', vault ->", n))
			}

			t.Vault = v
		}

		// If we have a dispatch_payload block parse that
		if o := listVal.Filter("dispatch_payload"); len(o.Items) > 0 {
			if len(o.Items) > 1 {
				return fmt.Errorf("only one dispatch_payload block is allowed in a task. Number of dispatch_payload blocks found: %d", len(o.Items))
			}
			var m map[string]interface{}
			dispatchBlock := o.Items[0]

			// Check for invalid keys
			valid := []string{
				"file",
			}
			if err := helper.CheckHCLKeys(dispatchBlock.Val, valid); err != nil {
				return multierror.Prefix(err, fmt.Sprintf("'%s', dispatch_payload ->", n))
			}

			if err := hcl.DecodeObject(&m, dispatchBlock.Val); err != nil {
				return err
			}

			t.DispatchPayload = &api.DispatchPayloadConfig{}
			if err := mapstructure.WeakDecode(m, t.DispatchPayload); err != nil {
				return err
			}
		}

		*result = append(*result, &t)
	}

	return nil
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
			"vault_grace",
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

func parseServices(jobName string, taskGroupName string, task *api.Task, serviceObjs *ast.ObjectList) error {
	task.Services = make([]*api.Service, len(serviceObjs.Items))
	for idx, o := range serviceObjs.Items {
		// Check for invalid keys
		valid := []string{
			"name",
			"tags",
			"port",
			"check",
			"address_mode",
		}
		if err := helper.CheckHCLKeys(o.Val, valid); err != nil {
			return multierror.Prefix(err, fmt.Sprintf("service (%d) ->", idx))
		}

		var service api.Service
		var m map[string]interface{}
		if err := hcl.DecodeObject(&m, o.Val); err != nil {
			return err
		}

		delete(m, "check")
		delete(m, "check_restart")

		if err := mapstructure.WeakDecode(m, &service); err != nil {
			return err
		}

		// Filter checks
		var checkList *ast.ObjectList
		if ot, ok := o.Val.(*ast.ObjectType); ok {
			checkList = ot.List
		} else {
			return fmt.Errorf("service '%s': should be an object", service.Name)
		}

		if co := checkList.Filter("check"); len(co.Items) > 0 {
			if err := parseChecks(&service, co); err != nil {
				return multierror.Prefix(err, fmt.Sprintf("service: '%s',", service.Name))
			}
		}

		// Filter check_restart
		if cro := checkList.Filter("check_restart"); len(cro.Items) > 0 {
			if len(cro.Items) > 1 {
				return fmt.Errorf("check_restart '%s': cannot have more than 1 check_restart", service.Name)
			}
			if cr, err := parseCheckRestart(cro.Items[0]); err != nil {
				return multierror.Prefix(err, fmt.Sprintf("service: '%s',", service.Name))
			} else {
				service.CheckRestart = cr
			}
		}

		task.Services[idx] = &service
	}

	return nil
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
			"command",
			"args",
			"initial_status",
			"tls_skip_verify",
			"header",
			"method",
			"check_restart",
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
			if cr, err := parseCheckRestart(cro.Items[0]); err != nil {
				return multierror.Prefix(err, fmt.Sprintf("check: '%s',", check.Name))
			} else {
				check.CheckRestart = cr
			}
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
		"iops",
		"disk",
		"memory",
		"network",
	}
	if err := helper.CheckHCLKeys(listVal, valid); err != nil {
		return multierror.Prefix(err, "resources ->")
	}

	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, o.Val); err != nil {
		return err
	}
	delete(m, "network")

	if err := mapstructure.WeakDecode(m, result); err != nil {
		return err
	}

	// Parse the network resources
	if o := listVal.Filter("network"); len(o.Items) > 0 {
		if len(o.Items) > 1 {
			return fmt.Errorf("only one 'network' resource allowed")
		}

		// Check for invalid keys
		valid := []string{
			"mbits",
			"port",
		}
		if err := helper.CheckHCLKeys(o.Items[0].Val, valid); err != nil {
			return multierror.Prefix(err, "resources, network ->")
		}

		var r api.NetworkResource
		var m map[string]interface{}
		if err := hcl.DecodeObject(&m, o.Items[0].Val); err != nil {
			return err
		}
		if err := mapstructure.WeakDecode(m, &r); err != nil {
			return err
		}

		var networkObj *ast.ObjectList
		if ot, ok := o.Items[0].Val.(*ast.ObjectType); ok {
			networkObj = ot.List
		} else {
			return fmt.Errorf("resource: should be an object")
		}
		if err := parsePorts(networkObj, &r); err != nil {
			return multierror.Prefix(err, "resources, network, ports ->")
		}

		result.Networks = []*api.NetworkResource{&r}
	}

	return nil
}

func parsePorts(networkObj *ast.ObjectList, nw *api.NetworkResource) error {
	// Check for invalid keys
	valid := []string{
		"mbits",
		"port",
	}
	if err := helper.CheckHCLKeys(networkObj, valid); err != nil {
		return err
	}

	portsObjList := networkObj.Filter("port")
	knownPortLabels := make(map[string]bool)
	for _, port := range portsObjList.Items {
		if len(port.Keys) == 0 {
			return fmt.Errorf("ports must be named")
		}
		label := port.Keys[0].Token.Value().(string)
		if !reDynamicPorts.MatchString(label) {
			return errPortLabel
		}
		l := strings.ToLower(label)
		if knownPortLabels[l] {
			return fmt.Errorf("found a port label collision: %s", label)
		}
		var p map[string]interface{}
		var res api.Port
		if err := hcl.DecodeObject(&p, port.Val); err != nil {
			return err
		}
		if err := mapstructure.WeakDecode(p, &res); err != nil {
			return err
		}
		res.Label = label
		if res.Value > 0 {
			nw.ReservedPorts = append(nw.ReservedPorts, res)
		} else {
			nw.DynamicPorts = append(nw.DynamicPorts, res)
		}
		knownPortLabels[l] = true
	}
	return nil
}

func parseUpdate(result **api.UpdateStrategy, list *ast.ObjectList) error {
	list = list.Elem()
	if len(list.Items) > 1 {
		return fmt.Errorf("only one 'update' block allowed")
	}

	// Get our resource object
	o := list.Items[0]

	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, o.Val); err != nil {
		return err
	}

	// Check for invalid keys
	valid := []string{
		// COMPAT: Remove in 0.7.0. Stagger is deprecated in 0.6.0.
		"stagger",
		"max_parallel",
		"health_check",
		"min_healthy_time",
		"healthy_deadline",
		"auto_revert",
		"canary",
	}
	if err := helper.CheckHCLKeys(o.Val, valid); err != nil {
		return err
	}

	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook:       mapstructure.StringToTimeDurationHookFunc(),
		WeaklyTypedInput: true,
		Result:           result,
	})
	if err != nil {
		return err
	}
	return dec.Decode(m)
}

func parsePeriodic(result **api.PeriodicConfig, list *ast.ObjectList) error {
	list = list.Elem()
	if len(list.Items) > 1 {
		return fmt.Errorf("only one 'periodic' block allowed per job")
	}

	// Get our resource object
	o := list.Items[0]

	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, o.Val); err != nil {
		return err
	}

	// Check for invalid keys
	valid := []string{
		"enabled",
		"cron",
		"prohibit_overlap",
		"time_zone",
	}
	if err := helper.CheckHCLKeys(o.Val, valid); err != nil {
		return err
	}

	if value, ok := m["enabled"]; ok {
		enabled, err := parseBool(value)
		if err != nil {
			return fmt.Errorf("periodic.enabled should be set to true or false; %v", err)
		}
		m["Enabled"] = enabled
	}

	// If "cron" is provided, set the type to "cron" and store the spec.
	if cron, ok := m["cron"]; ok {
		m["SpecType"] = structs.PeriodicSpecCron
		m["Spec"] = cron
	}

	// Build the constraint
	var p api.PeriodicConfig
	if err := mapstructure.WeakDecode(m, &p); err != nil {
		return err
	}
	*result = &p
	return nil
}

func parseVault(result *api.Vault, list *ast.ObjectList) error {
	list = list.Elem()
	if len(list.Items) == 0 {
		return nil
	}
	if len(list.Items) > 1 {
		return fmt.Errorf("only one 'vault' block allowed per task")
	}

	// Get our resource object
	o := list.Items[0]

	// We need this later
	var listVal *ast.ObjectList
	if ot, ok := o.Val.(*ast.ObjectType); ok {
		listVal = ot.List
	} else {
		return fmt.Errorf("vault: should be an object")
	}

	// Check for invalid keys
	valid := []string{
		"policies",
		"env",
		"change_mode",
		"change_signal",
	}
	if err := helper.CheckHCLKeys(listVal, valid); err != nil {
		return multierror.Prefix(err, "vault ->")
	}

	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, o.Val); err != nil {
		return err
	}

	if err := mapstructure.WeakDecode(m, result); err != nil {
		return err
	}

	return nil
}

func parseParameterizedJob(result **api.ParameterizedJobConfig, list *ast.ObjectList) error {
	list = list.Elem()
	if len(list.Items) > 1 {
		return fmt.Errorf("only one 'parameterized' block allowed per job")
	}

	// Get our resource object
	o := list.Items[0]

	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, o.Val); err != nil {
		return err
	}

	// Check for invalid keys
	valid := []string{
		"payload",
		"meta_required",
		"meta_optional",
	}
	if err := helper.CheckHCLKeys(o.Val, valid); err != nil {
		return err
	}

	// Build the parameterized job block
	var d api.ParameterizedJobConfig
	if err := mapstructure.WeakDecode(m, &d); err != nil {
		return err
	}

	*result = &d
	return nil
}
