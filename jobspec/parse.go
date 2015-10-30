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

	"github.com/hashicorp/hcl"
	hclobj "github.com/hashicorp/hcl/hcl"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/mapstructure"
)

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
	obj, err := hcl.Parse(buf.String())

	if err != nil {
		return nil, fmt.Errorf("error parsing: %s", err)
	}
	buf.Reset()

	var job structs.Job

	// Parse the job out
	jobO := obj.Get("job", false)
	if jobO == nil {
		return nil, fmt.Errorf("'job' stanza not found")
	}
	if err := parseJob(&job, jobO); err != nil {
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

func parseJob(result *structs.Job, obj *hclobj.Object) error {
	if obj.Len() > 1 {
		return fmt.Errorf("only one 'job' block allowed")
	}

	// Get our job object
	obj = obj.Elem(true)[0]

	// Decode the full thing into a map[string]interface for ease
	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, obj); err != nil {
		return err
	}
	delete(m, "constraint")
	delete(m, "meta")
	delete(m, "update")

	// Set the ID and name to the object key
	result.ID = obj.Key
	result.Name = obj.Key

	// Defaults
	result.Priority = 50
	result.Region = "global"
	result.Type = "service"

	// Decode the rest
	if err := mapstructure.WeakDecode(m, result); err != nil {
		return err
	}

	// Parse constraints
	if o := obj.Get("constraint", false); o != nil {
		if err := parseConstraints(&result.Constraints, o); err != nil {
			return err
		}
	}

	// If we have an update strategy, then parse that
	if o := obj.Get("update", false); o != nil {
		if err := parseUpdate(&result.Update, o); err != nil {
			return err
		}
	}

	// Parse out meta fields. These are in HCL as a list so we need
	// to iterate over them and merge them.
	if metaO := obj.Get("meta", false); metaO != nil {
		for _, o := range metaO.Elem(false) {
			var m map[string]interface{}
			if err := hcl.DecodeObject(&m, o); err != nil {
				return err
			}
			if err := mapstructure.WeakDecode(m, &result.Meta); err != nil {
				return err
			}
		}
	}

	// If we have tasks outside, create TaskGroups for them
	if o := obj.Get("task", false); o != nil {
		var tasks []*structs.Task
		if err := parseTasks(&tasks, o); err != nil {
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
	if o := obj.Get("group", false); o != nil {
		if err := parseGroups(result, o); err != nil {
			return fmt.Errorf("error parsing 'group': %s", err)
		}
	}

	return nil
}

func parseGroups(result *structs.Job, obj *hclobj.Object) error {
	// Get all the maps of keys to the actual object
	objects := make(map[string]*hclobj.Object)
	for _, o1 := range obj.Elem(false) {
		for _, o2 := range o1.Elem(true) {
			if _, ok := objects[o2.Key]; ok {
				return fmt.Errorf(
					"group '%s' defined more than once",
					o2.Key)
			}

			objects[o2.Key] = o2
		}
	}

	if len(objects) == 0 {
		return nil
	}

	// Go through each object and turn it into an actual result.
	collection := make([]*structs.TaskGroup, 0, len(objects))
	for n, o := range objects {
		var m map[string]interface{}
		if err := hcl.DecodeObject(&m, o); err != nil {
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
		if o := o.Get("constraint", false); o != nil {
			if err := parseConstraints(&g.Constraints, o); err != nil {
				return err
			}
		}
		g.RestartPolicy = structs.NewRestartPolicy(result.Type)

		if err := parseRestartPolicy(g.RestartPolicy, o); err != nil {
			return err
		}

		// Parse out meta fields. These are in HCL as a list so we need
		// to iterate over them and merge them.
		if metaO := o.Get("meta", false); metaO != nil {
			for _, o := range metaO.Elem(false) {
				var m map[string]interface{}
				if err := hcl.DecodeObject(&m, o); err != nil {
					return err
				}
				if err := mapstructure.WeakDecode(m, &g.Meta); err != nil {
					return err
				}
			}
		}

		// Parse tasks
		if o := o.Get("task", false); o != nil {
			if err := parseTasks(&g.Tasks, o); err != nil {
				return err
			}
		}

		collection = append(collection, &g)
	}

	result.TaskGroups = append(result.TaskGroups, collection...)
	return nil
}

func parseRestartPolicy(result *structs.RestartPolicy, obj *hclobj.Object) error {
	var restartHclObj *hclobj.Object
	var m map[string]interface{}
	if restartHclObj = obj.Get("restart", false); restartHclObj == nil {
		return nil
	}
	if err := hcl.DecodeObject(&m, restartHclObj); err != nil {
		return err
	}

	if delay, ok := m["delay"]; ok {
		d, err := toDuration(delay)
		if err != nil {
			return fmt.Errorf("Invalid Delay time in restart policy: %v", err)
		}
		result.Delay = d
	}

	if interval, ok := m["interval"]; ok {
		i, err := toDuration(interval)
		if err != nil {
			return fmt.Errorf("Invalid Interval time in restart policy: %v", err)
		}
		result.Interval = i
	}

	if attempts, ok := m["attempts"]; ok {
		a, err := toInteger(attempts)
		if err != nil {
			return fmt.Errorf("Invalid value in attempts: %v", err)
		}
		result.Attempts = a
	}
	return nil
}

func parseConstraints(result *[]*structs.Constraint, obj *hclobj.Object) error {
	for _, o := range obj.Elem(false) {
		var m map[string]interface{}
		if err := hcl.DecodeObject(&m, o); err != nil {
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
			enabled, err := strconv.ParseBool(value.(string))
			if err != nil {
				return err
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

func parseTasks(result *[]*structs.Task, obj *hclobj.Object) error {
	// Get all the maps of keys to the actual object
	objects := make([]*hclobj.Object, 0, 5)
	set := make(map[string]struct{})
	for _, o1 := range obj.Elem(false) {
		for _, o2 := range o1.Elem(true) {
			if _, ok := set[o2.Key]; ok {
				return fmt.Errorf(
					"group '%s' defined more than once",
					o2.Key)
			}

			objects = append(objects, o2)
			set[o2.Key] = struct{}{}
		}
	}

	if len(objects) == 0 {
		return nil
	}

	for _, o := range objects {
		var m map[string]interface{}
		if err := hcl.DecodeObject(&m, o); err != nil {
			return err
		}
		delete(m, "config")
		delete(m, "env")
		delete(m, "constraint")
		delete(m, "meta")
		delete(m, "resources")

		// Build the task
		var t structs.Task
		t.Name = o.Key
		if err := mapstructure.WeakDecode(m, &t); err != nil {
			return err
		}

		// If we have env, then parse them
		if o := o.Get("env", false); o != nil {
			for _, o := range o.Elem(false) {
				var m map[string]interface{}
				if err := hcl.DecodeObject(&m, o); err != nil {
					return err
				}
				if err := mapstructure.WeakDecode(m, &t.Env); err != nil {
					return err
				}
			}
		}

		// If we have config, then parse that
		if o := o.Get("config", false); o != nil {
			for _, o := range o.Elem(false) {
				var m map[string]interface{}
				if err := hcl.DecodeObject(&m, o); err != nil {
					return err
				}
				if err := mapstructure.WeakDecode(m, &t.Config); err != nil {
					return err
				}
			}
		}

		// Parse constraints
		if o := o.Get("constraint", false); o != nil {
			if err := parseConstraints(&t.Constraints, o); err != nil {
				return err
			}
		}

		// Parse out meta fields. These are in HCL as a list so we need
		// to iterate over them and merge them.
		if metaO := o.Get("meta", false); metaO != nil {
			for _, o := range metaO.Elem(false) {
				var m map[string]interface{}
				if err := hcl.DecodeObject(&m, o); err != nil {
					return err
				}
				if err := mapstructure.WeakDecode(m, &t.Meta); err != nil {
					return err
				}
			}
		}

		// If we have resources, then parse that
		if o := o.Get("resources", false); o != nil {
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

var reDynamicPorts *regexp.Regexp = regexp.MustCompile("^[a-zA-Z0-9_]+$")
var errDynamicPorts = fmt.Errorf("DynamicPort label does not conform to naming requirements %s", reDynamicPorts.String())

func parseResources(result *structs.Resources, obj *hclobj.Object) error {
	if obj.Len() > 1 {
		return fmt.Errorf("only one 'resource' block allowed per task")
	}

	for _, o := range obj.Elem(false) {
		var m map[string]interface{}
		if err := hcl.DecodeObject(&m, o); err != nil {
			return err
		}
		delete(m, "network")

		if err := mapstructure.WeakDecode(m, result); err != nil {
			return err
		}

		// Parse the network resources
		if o := o.Get("network", false); o != nil {
			if o.Len() > 1 {
				return fmt.Errorf("only one 'network' resource allowed")
			}

			var r structs.NetworkResource
			var m map[string]interface{}
			if err := hcl.DecodeObject(&m, o); err != nil {
				return err
			}
			if err := mapstructure.WeakDecode(m, &r); err != nil {
				return err
			}

			// Keep track of labels we've already seen so we can ensure there
			// are no collisions when we turn them into environment variables.
			// lowercase:NomalCase so we can get the first for the error message
			seenLabel := map[string]string{}

			for _, label := range r.DynamicPorts {
				if !reDynamicPorts.MatchString(label) {
					return errDynamicPorts
				}
				first, seen := seenLabel[strings.ToLower(label)]
				if seen {
					return fmt.Errorf("Found a port label collision: `%s` overlaps with previous `%s`", label, first)
				} else {
					seenLabel[strings.ToLower(label)] = label
				}

			}

			result.Networks = []*structs.NetworkResource{&r}
		}

	}

	return nil
}

func parseUpdate(result *structs.UpdateStrategy, obj *hclobj.Object) error {
	if obj.Len() > 1 {
		return fmt.Errorf("only one 'update' block allowed per job")
	}

	for _, o := range obj.Elem(false) {
		var m map[string]interface{}
		if err := hcl.DecodeObject(&m, o); err != nil {
			return err
		}
		for _, key := range []string{"stagger", "Stagger"} {
			if raw, ok := m[key]; ok {
				switch v := raw.(type) {
				case string:
					dur, err := time.ParseDuration(v)
					if err != nil {
						return fmt.Errorf("invalid stagger time '%s'", raw)
					}
					m[key] = dur
				case int:
					m[key] = time.Duration(v) * time.Second
				default:
					return fmt.Errorf("invalid type for stagger time '%s'",
						raw)
				}
			}
		}

		if err := mapstructure.WeakDecode(m, result); err != nil {
			return err
		}
	}
	return nil
}

func toDuration(value interface{}) (time.Duration, error) {
	var dur time.Duration
	var err error
	switch v := value.(type) {
	case string:
		dur, err = time.ParseDuration(v)
	case int:
		dur = time.Duration(v) * time.Second
	default:
		err = fmt.Errorf("Invalid time %s", value)
	}

	return dur, err
}

func toInteger(value interface{}) (int, error) {
	var integer int
	var err error
	switch v := value.(type) {
	case string:
		var i int64
		i, err = strconv.ParseInt(v, 10, 32)
		integer = int(i)
	case int:
		integer = v
	default:
		err = fmt.Errorf("Value: %v can't be parsed into int", value)
	}

	return integer, err
}
