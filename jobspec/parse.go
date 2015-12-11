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

	"github.com/hashicorp/hcl"
	"github.com/hashicorp/hcl/hcl/ast"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/mapstructure"
)

var reDynamicPorts *regexp.Regexp = regexp.MustCompile("^[a-zA-Z0-9_]+$")
var errPortLabel = fmt.Errorf("Port label does not conform to naming requirements %s", reDynamicPorts.String())

// Parse parses the job spec from the given io.Reader.
//
// Due to current internal limitations, the entire contents of the
// io.Reader will be copied into memory first before parsing.
func Parse(r io.Reader) (*structs.Job, error) {
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

	var job structs.Job

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
func ParseFile(path string) (*structs.Job, error) {
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

func parseJob(result *structs.Job, list *ast.ObjectList) error {
	list = list.Children()
	if len(list.Items) != 1 {
		return fmt.Errorf("only one 'job' block allowed")
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

	// Set the ID and name to the object key
	result.ID = obj.Keys[0].Token.Value().(string)
	result.Name = result.ID

	// Defaults
	result.Priority = 50
	result.Region = "global"
	result.Type = "service"

	// Decode the rest
	if err := mapstructure.WeakDecode(m, result); err != nil {
		return err
	}

	// Value should be an object
	var listVal *ast.ObjectList
	if ot, ok := obj.Val.(*ast.ObjectType); ok {
		listVal = ot.List
	} else {
		return fmt.Errorf("job '%s' value: should be an object", result.ID)
	}

	// Parse constraints
	if o := listVal.Filter("constraint"); len(o.Items) > 0 {
		if err := parseConstraints(&result.Constraints, o); err != nil {
			return err
		}
	}

	// If we have an update strategy, then parse that
	if o := listVal.Filter("update"); len(o.Items) > 0 {
		if err := parseUpdate(&result.Update, o); err != nil {
			return err
		}
	}

	// If we have a periodic definition, then parse that
	if o := listVal.Filter("periodic"); len(o.Items) > 0 {
		if err := parsePeriodic(&result.Periodic, o); err != nil {
			return err
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
		var tasks []*structs.Task
		if err := parseTasks(result.Name, "", &tasks, o); err != nil {
			return err
		}

		result.TaskGroups = make([]*structs.TaskGroup, len(tasks), len(tasks)*2)
		for i, t := range tasks {
			result.TaskGroups[i] = &structs.TaskGroup{
				Name:          t.Name,
				Count:         1,
				Tasks:         []*structs.Task{t},
				RestartPolicy: structs.NewRestartPolicy(result.Type),
			}
		}
	}

	// Parse the task groups
	if o := listVal.Filter("group"); len(o.Items) > 0 {
		if err := parseGroups(result, o); err != nil {
			return fmt.Errorf("error parsing 'group': %s", err)
		}
	}

	return nil
}

func parseGroups(result *structs.Job, list *ast.ObjectList) error {
	list = list.Children()
	if len(list.Items) == 0 {
		return nil
	}

	// Go through each object and turn it into an actual result.
	collection := make([]*structs.TaskGroup, 0, len(list.Items))
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

		var m map[string]interface{}
		if err := hcl.DecodeObject(&m, item.Val); err != nil {
			return err
		}
		delete(m, "constraint")
		delete(m, "meta")
		delete(m, "task")
		delete(m, "restart")

		// Default count to 1 if not specified
		if _, ok := m["count"]; !ok {
			m["count"] = 1
		}

		// Build the group with the basic decode
		var g structs.TaskGroup
		g.Name = n
		if err := mapstructure.WeakDecode(m, &g); err != nil {
			return err
		}

		// Parse constraints
		if o := listVal.Filter("constraint"); len(o.Items) > 0 {
			if err := parseConstraints(&g.Constraints, o); err != nil {
				return err
			}
		}
		g.RestartPolicy = structs.NewRestartPolicy(result.Type)

		// Parse restart policy
		if o := listVal.Filter("restart"); len(o.Items) > 0 {
			if err := parseRestartPolicy(g.RestartPolicy, o); err != nil {
				return err
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
			if err := parseTasks(result.Name, g.Name, &g.Tasks, o); err != nil {
				return err
			}
		}

		collection = append(collection, &g)
	}

	result.TaskGroups = append(result.TaskGroups, collection...)
	return nil
}

func parseRestartPolicy(final *structs.RestartPolicy, list *ast.ObjectList) error {
	list = list.Elem()
	if len(list.Items) == 0 {
		return nil
	}
	if len(list.Items) != 1 {
		return fmt.Errorf("only one 'restart' block allowed")
	}

	// Get our job object
	obj := list.Items[0]

	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, obj.Val); err != nil {
		return err
	}

	var result structs.RestartPolicy
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

	*final = result
	return nil
}

func parseConstraints(result *[]*structs.Constraint, list *ast.ObjectList) error {
	for _, o := range list.Elem().Items {
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

		// Build the constraint
		var c structs.Constraint
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

func parseTasks(jobName string, taskGroupName string, result *[]*structs.Task, list *ast.ObjectList) error {
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

		var m map[string]interface{}
		if err := hcl.DecodeObject(&m, item.Val); err != nil {
			return err
		}
		delete(m, "config")
		delete(m, "env")
		delete(m, "constraint")
		delete(m, "service")
		delete(m, "meta")
		delete(m, "resources")

		// Build the task
		var t structs.Task
		t.Name = n
		if taskGroupName == "" {
			taskGroupName = n
		}
		if err := mapstructure.WeakDecode(m, &t); err != nil {
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
				return err
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
				return err
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
			var r structs.Resources
			if err := parseResources(&r, o); err != nil {
				return fmt.Errorf("task '%s': %s", t.Name, err)
			}

			t.Resources = &r
		}

		*result = append(*result, &t)
	}

	return nil
}

func parseServices(jobName string, taskGroupName string, task *structs.Task, serviceObjs *ast.ObjectList) error {
	task.Services = make([]*structs.Service, len(serviceObjs.Items))
	var defaultServiceName bool
	for idx, o := range serviceObjs.Items {
		var service structs.Service
		var m map[string]interface{}
		if err := hcl.DecodeObject(&m, o.Val); err != nil {
			return err
		}

		delete(m, "check")

		if err := mapstructure.WeakDecode(m, &service); err != nil {
			return err
		}

		if defaultServiceName && service.Name == "" {
			return fmt.Errorf("Only one service block may omit the Name field")
		}

		if service.Name == "" {
			defaultServiceName = true
			service.Name = fmt.Sprintf("%s-%s-%s", jobName, taskGroupName, task.Name)
		}

		// Fileter checks
		var checkList *ast.ObjectList
		if ot, ok := o.Val.(*ast.ObjectType); ok {
			checkList = ot.List
		} else {
			return fmt.Errorf("service '%s': should be an object", service.Name)
		}

		if co := checkList.Filter("check"); len(co.Items) > 0 {
			if err := parseChecks(&service, co); err != nil {
				return err
			}
		}

		task.Services[idx] = &service
	}

	return nil
}

func parseChecks(service *structs.Service, checkObjs *ast.ObjectList) error {
	service.Checks = make([]*structs.ServiceCheck, len(checkObjs.Items))
	for idx, co := range checkObjs.Items {
		var check structs.ServiceCheck
		var cm map[string]interface{}
		if err := hcl.DecodeObject(&cm, co.Val); err != nil {
			return err
		}
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

		service.Checks[idx] = &check
	}

	return nil
}

func parseResources(result *structs.Resources, list *ast.ObjectList) error {
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

		var r structs.NetworkResource
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
			return err
		}

		result.Networks = []*structs.NetworkResource{&r}
	}

	return nil
}

func parsePorts(networkObj *ast.ObjectList, nw *structs.NetworkResource) error {
	portsObjList := networkObj.Filter("port")
	knownPortLabels := make(map[string]bool)
	for _, port := range portsObjList.Items {
		label := port.Keys[0].Token.Value().(string)
		if !reDynamicPorts.MatchString(label) {
			return errPortLabel
		}
		l := strings.ToLower(label)
		if knownPortLabels[l] {
			return fmt.Errorf("Found a port label collision: %s", label)
		}
		var p map[string]interface{}
		var res structs.Port
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

func parseUpdate(result *structs.UpdateStrategy, list *ast.ObjectList) error {
	list = list.Elem()
	if len(list.Items) > 1 {
		return fmt.Errorf("only one 'update' block allowed per job")
	}

	// Get our resource object
	o := list.Items[0]

	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, o.Val); err != nil {
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

func parsePeriodic(result **structs.PeriodicConfig, list *ast.ObjectList) error {
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

	// Enabled by default if the periodic block exists.
	if value, ok := m["enabled"]; !ok {
		m["Enabled"] = true
	} else {
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
	var p structs.PeriodicConfig
	if err := mapstructure.WeakDecode(m, &p); err != nil {
		return err
	}
	*result = &p
	return nil
}
